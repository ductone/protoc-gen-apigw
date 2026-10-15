package tfoverlay

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/speakeasy-api/openapi/overlay"
	"gopkg.in/yaml.v3"
)

// Selector operations are the document-wide counterpart of Operation. Where an
// Operation writes one key at one JSON Pointer, a SelectorOperation writes every
// node a Selector matches, for rules that belong to the document rather than to
// one declaration ("every property named deletedAt is a soft-delete property").
//
// Stages. ApplyOverlay resolves selectors against the document first, then
// applies the exact operations to the result. An exact operation therefore sees
// the document the selectors produced and may replace what a selector wrote: an
// exact operation that writes the same node and key as a selector wins, and that
// is not a conflict. It is how a selector with a few exceptions is spelled
// ("additionalProperties: true everywhere becomes false, except on these two
// nodes"). The ProjectionReport counts these overrides per selector.

// Selector picks nodes of a document with an RFC 9535 JSONPath query, for
// example `$..deletedAt` or `$..[?(@.additionalProperties == true)]`. The
// query is the action target of the rendered OpenAPI Overlay, so the matching
// semantics are those of the standard tooling that applies it.
type Selector struct {
	Path string
}

func (s Selector) validate() error {
	if s.Path == "" {
		return fmt.Errorf("a selector needs a JSONPath")
	}
	if _, err := newJSONPath(s.Path); err != nil {
		return fmt.Errorf("selector %q is not an RFC 9535 JSONPath: %w", s.Path, err)
	}
	return nil
}

// String is the JSONPath, for diagnostics.
func (s Selector) String() string { return s.Path }

// SelectorOperation writes or removes every node a Selector matches.
type SelectorOperation struct {
	Selector Selector
	// Mode is ModeSet, which writes Channel/Key=Value on each matched node, or
	// ModeRemove, which deletes each matched entry from its parent and is only
	// valid for a selector by Key.
	Mode    Mode
	Channel Channel
	Key     string
	Value   *yaml.Node
	// Provenance identifies the annotation, as for Operation.
	Provenance string
}

// Validate reports whether the selector operation is well formed.
func (s SelectorOperation) Validate() error {
	if err := s.Selector.validate(); err != nil {
		return err
	}
	switch s.Mode {
	case ModeSet:
		switch s.Channel {
		case ChannelExtension:
			if !strings.HasPrefix(s.Key, "x-") {
				return fmt.Errorf("selector %s: channel %s requires an extension key starting with \"x-\", got %q", s.Selector, s.Channel, s.Key)
			}
		case ChannelSchema:
			if s.Key == "" || strings.HasPrefix(s.Key, "x-") {
				return fmt.Errorf("selector %s: channel %s requires a schema keyword, got %q", s.Selector, s.Channel, s.Key)
			}
		default:
			return fmt.Errorf("selector %s: channel %q is not valid for a selector", s.Selector, s.Channel)
		}
		if s.Value == nil {
			return fmt.Errorf("selector %s: set requires a value", s.Selector)
		}
		// A standard Overlay update merges an object or array value into what is
		// already there; only a scalar is a plain replacement, so only a scalar
		// keeps one meaning for every matched node.
		if s.Value.Kind != yaml.ScalarNode {
			return fmt.Errorf("selector %s: set requires a scalar value, got %s", s.Selector, nodeKind(s.Value))
		}
	case ModeRemove:
		if s.Key != "" || s.Channel != "" || s.Value != nil {
			return fmt.Errorf("selector %s: remove carries no channel, key or value", s.Selector)
		}
	default:
		return fmt.Errorf("selector %s: unknown mode %q", s.Selector, s.Mode)
	}
	return nil
}

func (s SelectorOperation) identity() string {
	return s.Selector.String() + "|" + string(s.Channel) + "|" + s.Key + "|" + string(s.Mode)
}

func selectorsEqual(a, b SelectorOperation) bool {
	return a.identity() == b.identity() && nodesEqual(a.Value, b.Value)
}

// AddSelector records sel, deduplicating an identical selector operation and
// rejecting one that assigns a different value to the same selector and key, or
// that both sets and removes the same selector.
func (o *Overlay) AddSelector(sel SelectorOperation) error {
	if err := sel.Validate(); err != nil {
		return err
	}
	for _, existing := range o.Selectors {
		if existing.Selector.String() != sel.Selector.String() {
			continue
		}
		if selectorsEqual(existing, sel) {
			return nil
		}
		if existing.Mode == ModeRemove || sel.Mode == ModeRemove || existing.Key == sel.Key && existing.Channel == sel.Channel {
			return &Error{
				Channel: sel.Channel, Key: sel.Key,
				Owners: append(provenanceOf(existing.Provenance), provenanceOf(sel.Provenance)...),
				Message: fmt.Sprintf("conflicting selector assignments for %s: %s %s vs %s %s",
					sel.Selector, existing.Mode, displayValue(existing.Value), sel.Mode, displayValue(sel.Value)),
			}
		}
	}
	o.Selectors = append(o.Selectors, sel)
	sort.SliceStable(o.Selectors, func(i, j int) bool {
		a, b := o.Selectors[i], o.Selectors[j]
		if ai, bi := a.identity(), b.identity(); ai != bi {
			return ai < bi
		}
		return displayValue(a.Value) < displayValue(b.Value)
	})
	return nil
}

func provenanceOf(p string) []string {
	if p == "" {
		return nil
	}
	return []string{p}
}

// ---------------------------------------------------------------------------
// application
// ---------------------------------------------------------------------------

// SelectorReport records what one selector operation did.
type SelectorReport struct {
	Selector string
	Mode     Mode
	Channel  Channel
	Key      string
	// Matches is the number of nodes the selector matched. A selector that
	// matches none is an error.
	Matches int
	// Overridden counts exact operations that wrote a node and key the selector
	// wrote, and so replaced its result.
	Overridden int
}

type selectorWrite struct {
	pointer string
	key     string
}

// ApplyOverlay applies an overlay in its two stages: selectors against the
// document, then ApplyTerraform for the exact operations. See the package
// comment for the precedence between them.
func ApplyOverlay(doc []byte, ov *Overlay) ([]byte, *ProjectionReport, error) {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return nil, nil, fmt.Errorf("parsing document: %w", err)
	}
	report, err := ApplyOverlayToNode(root, ov)
	if err != nil {
		return nil, nil, err
	}
	out, err := encode(root)
	if err != nil {
		return nil, nil, err
	}
	return out, report, nil
}

// ApplyOverlayToNode is ApplyOverlay on a parsed document, modified in place.
func ApplyOverlayToNode(root *yaml.Node, ov *Overlay) (*ProjectionReport, error) {
	doc := documentRoot(root)
	if doc == nil {
		return nil, fmt.Errorf("document has no root node")
	}
	selectors, writes, err := applySelectors(doc, ov.Selectors, false)
	if err != nil {
		return nil, err
	}
	report, err := ApplyTerraformToNode(root, ov.Operations)
	if err != nil {
		return nil, err
	}
	for _, op := range ov.Operations {
		for i, w := range writes {
			if _, ok := w[selectorWrite{pointer: op.Pointer, key: op.Key}]; ok {
				selectors[i].Overridden++
			}
		}
	}
	report.Selectors = selectors
	return report, nil
}

type selectorMatch struct {
	tokens []string
	parent *yaml.Node // mapping or sequence holding the matched node
	node   *yaml.Node // the matched node
}

func applySelectorsLenient(doc *yaml.Node, sels []SelectorOperation) ([]SelectorReport, []map[selectorWrite]bool, error) {
	return applySelectors(doc, sels, true)
}

// applySelectors resolves every selector against the unmodified document, then
// writes. Matching first makes the result independent of selector order; writes
// that would disagree are reported rather than ordered. A selector that matches
// nothing is an error unless lenient, which Render uses on a per-service
// fragment of the document the selector is meant for.
func applySelectors(doc *yaml.Node, sels []SelectorOperation, lenient bool) ([]SelectorReport, []map[selectorWrite]bool, error) {
	reports := make([]SelectorReport, len(sels))
	writes := make([]map[selectorWrite]bool, len(sels))
	matches := make([][]selectorMatch, len(sels))
	for i, sel := range sels {
		if err := sel.Validate(); err != nil {
			return nil, nil, err
		}
		found, err := findMatches(doc, sel.Selector)
		if err != nil {
			return nil, nil, err
		}
		matches[i] = found
		reports[i] = SelectorReport{
			Selector: sel.Selector.String(), Mode: sel.Mode, Channel: sel.Channel, Key: sel.Key,
			Matches: len(matches[i]),
		}
		if len(matches[i]) == 0 && !lenient {
			return nil, nil, fmt.Errorf("selector %s matches no node in the document", sel.Selector)
		}
	}

	// Detect disagreement between selectors before changing anything.
	assigned := map[selectorWrite]string{}
	removed := map[string]string{}
	for i, sel := range sels {
		writes[i] = map[selectorWrite]bool{}
		for _, m := range matches[i] {
			if sel.Mode == ModeRemove {
				ptr := JoinPointer(m.tokens...)
				removed[ptr] = sel.Selector.String()
				writes[i][selectorWrite{pointer: JoinPointer(m.tokens[:len(m.tokens)-1]...), key: m.tokens[len(m.tokens)-1]}] = true
				continue
			}
			if m.node.Kind != yaml.MappingNode {
				return nil, nil, fmt.Errorf("selector %s matches %s, which is not an object and cannot carry %s",
					sel.Selector, pointerForDisplay(JoinPointer(m.tokens...)), sel.Key)
			}
			w := selectorWrite{pointer: JoinPointer(m.tokens...), key: sel.Key}
			writes[i][w] = true
			desc := string(sel.Channel) + "=" + displayValue(sel.Value)
			if prev, ok := assigned[w]; ok && prev != desc {
				return nil, nil, fmt.Errorf("selectors disagree at %s for %s: %s vs %s",
					pointerForDisplay(w.pointer), w.key, prev, desc)
			}
			assigned[w] = desc
		}
	}
	for w := range assigned {
		tokens, _ := ParsePointer(w.pointer)
		for n := len(tokens); n > 0; n-- {
			if by, ok := removed[JoinPointer(tokens[:n]...)]; ok {
				return nil, nil, fmt.Errorf("selector %s removes %s, which another selector writes %s on",
					by, JoinPointer(tokens[:n]...), w.key)
			}
		}
	}

	for i, sel := range sels {
		ms := matches[i]
		if sel.Mode == ModeRemove {
			for _, m := range ms {
				removeChild(m.parent, m.node)
			}
			continue
		}
		for _, m := range ms {
			if m.node.Kind != yaml.MappingNode {
				continue
			}
			if idx := findKey(m.node, sel.Key); idx >= 0 {
				m.node.Content[idx+1] = sel.Value
			} else {
				m.node.Content = append(m.node.Content, scalarNode("!!str", sel.Key), sel.Value)
			}
		}
	}
	return reports, writes, nil
}

// findMatches evaluates the selector's JSONPath on the document and returns
// every matched node with its pointer tokens and parent, in document order.
func findMatches(doc *yaml.Node, sel Selector) ([]selectorMatch, error) {
	path, err := newJSONPath(sel.Path)
	if err != nil {
		return nil, fmt.Errorf("selector %q: %w", sel.Path, err)
	}
	index := indexNodes(doc)
	var out []selectorMatch
	for _, node := range path.Query(doc) {
		at, ok := index[node]
		if !ok || at.parent == nil {
			// The document root has no parent and no pointer token; a selector
			// that selects it would rewrite the whole document.
			return nil, fmt.Errorf("selector %q matches the document root", sel.Path)
		}
		out = append(out, selectorMatch{tokens: at.tokens, parent: at.parent, node: node})
	}
	return out, nil
}

// newJSONPath parses an RFC 9535 query the way the Overlay 1.1.0 applier does.
func newJSONPath(query string) (overlay.Queryable, error) {
	return (&overlay.Overlay{Version: StandardOverlayVersion}).NewPath(query, nil)
}

type nodeLocation struct {
	tokens []string
	parent *yaml.Node
}

// indexNodes maps every node below doc to its pointer tokens and parent.
func indexNodes(doc *yaml.Node) map[*yaml.Node]nodeLocation {
	index := map[*yaml.Node]nodeLocation{doc: {}}
	var walk func(node *yaml.Node, tokens []string)
	walk = func(node *yaml.Node, tokens []string) {
		switch node.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(node.Content); i += 2 {
				child := append(append([]string(nil), tokens...), node.Content[i].Value)
				index[node.Content[i+1]] = nodeLocation{tokens: child, parent: node}
				walk(node.Content[i+1], child)
			}
		case yaml.SequenceNode:
			for i, item := range node.Content {
				child := append(append([]string(nil), tokens...), strconv.Itoa(i))
				index[item] = nodeLocation{tokens: child, parent: node}
				walk(item, child)
			}
		default:
			// Scalars and aliases have no children.
		}
	}
	walk(doc, nil)
	return index
}

// removeChild deletes node from parent: the entry whose value it is, or the
// sequence item it is.
func removeChild(parent, node *yaml.Node) {
	switch parent.Kind {
	case yaml.MappingNode:
		for i := 1; i < len(parent.Content); i += 2 {
			if parent.Content[i] == node {
				parent.Content = append(parent.Content[:i-1], parent.Content[i+1:]...)
				return
			}
		}
	case yaml.SequenceNode:
		for i, item := range parent.Content {
			if item == node {
				parent.Content = append(parent.Content[:i], parent.Content[i+1:]...)
				return
			}
		}
	default:
	}
}

package tfoverlay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/speakeasy-api/openapi/overlay"
	"gopkg.in/yaml.v3"
)

// The artifact is an OpenAPI Overlay 1.1.0 document
// (https://spec.openapis.org/overlay/v1.1.0.html). Standard tooling such as
// `speakeasy overlay apply` applies the global part directly; this package
// reads it back with Parse to merge and check it.
//
// An Overlay update merges objects and concatenates arrays, while a set must
// replace the complete value. Render therefore writes a set whose destination
// already holds a value that a plain update would not replace as a remove of
// that key followed by an update. The remove carries extensionReplace so Parse
// folds the pair back into one set.

const (
	// StandardOverlayVersion is the Overlay Specification version Render emits.
	StandardOverlayVersion = "1.1.0"

	// ArtifactSuffix names the rendered artifact beside an OpenAPI document:
	// <base>.<owner>.<suffix>.
	ArtifactSuffix = "terraform_overlay.yaml"

	extensionMetadata   = "x-apigw-terraform"
	extensionProvenance = "x-apigw-provenance"
	extensionReplace    = "x-apigw-replace"
)

// Render serializes the overlay against doc, the document it applies to. The
// document decides how a set is spelled: an update alone when the key is
// absent or holds a scalar of the same type, otherwise a remove followed by an
// update. Rendering does not modify the receiver or doc. An overlay with no
// operations renders as nil: there is no artifact to emit.
//
// Use VerifyStandard on the document the overlay is finally applied to.
func (o *Overlay) Render(doc []byte) ([]byte, error) {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return nil, fmt.Errorf("parsing document: %w", err)
	}
	target := documentRoot(root)
	if target == nil {
		return nil, fmt.Errorf("document has no root node")
	}

	ops := make([]Operation, len(o.Operations))
	copy(ops, o.Operations)
	sortOperations(ops)

	var actions []overlay.Action
	// Simulate the global part so each set is spelled against the value its
	// destination holds when the action runs.
	for _, op := range ops {
		spelled, err := operationActions(target, op)
		if err != nil {
			return nil, err
		}
		actions = append(actions, spelled...)
		if err := applyOne(target, op); err != nil {
			return nil, err
		}
	}
	if len(actions) == 0 {
		return nil, nil
	}
	return o.encodeStandard(actions)
}

func (o *Overlay) encodeStandard(actions []overlay.Action) ([]byte, error) {
	title := "Terraform customizations"
	if o.Source != "" {
		title = o.Source + " Terraform customizations"
	}
	meta := map[string]any{}
	if o.Source != "" {
		meta["source"] = o.Source
	}
	if o.Kind != "" {
		meta["kind"] = string(o.Kind)
	}
	doc := &overlay.Overlay{
		Extensions: overlay.Extensions{extensionMetadata: meta},
		Version:    StandardOverlayVersion,
		Info:       overlay.Info{Title: title, Version: "0.0.0"},
		Actions:    actions,
	}
	text, err := doc.ToString()
	if err != nil {
		return nil, fmt.Errorf("rendering overlay: %w", err)
	}
	return []byte(text), nil
}

// operationActions spells one exact operation against the current state of
// target.
func operationActions(target *yaml.Node, op Operation) ([]overlay.Action, error) {
	node, err := resolvePointer(target, op.Pointer)
	if err != nil {
		return nil, newError(op, ownerLabel(op), "%s", err)
	}
	path, err := normalizedPath(target, op.Pointer)
	if err != nil {
		return nil, newError(op, ownerLabel(op), "%s", err)
	}
	ext := overlay.Extensions{}
	if op.Provenance != "" {
		ext[extensionProvenance] = op.Provenance
	}
	keyPath := path + "[" + quoteMember(op.Key) + "]"
	if op.Mode == ModeRemove {
		return []overlay.Action{{Target: keyPath, Remove: true, Extensions: ext}}, nil
	}
	var actions []overlay.Action
	if idx := findKey(node, op.Key); idx >= 0 && !replacesInPlace(node.Content[idx+1], op.Value) {
		actions = append(actions, overlay.Action{Target: keyPath, Remove: true, Extensions: overlay.Extensions{extensionReplace: true}})
	}
	return append(actions, overlay.Action{Target: path, Update: mapping(op.Key, op.Value), Extensions: ext}), nil
}

// replacesInPlace reports whether an Overlay update of value over existing is
// a plain replacement. Only a scalar over a scalar of the same tag is: an
// object merges, an array concatenates, and a scalar update keeps the existing
// tag.
func replacesInPlace(existing, value *yaml.Node) bool {
	return existing.Kind == yaml.ScalarNode && value.Kind == yaml.ScalarNode && existing.Tag == value.Tag
}

func mapping(key string, value *yaml.Node) yaml.Node {
	return yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalarNode("!!str", key), value}}
}

// normalizedPath converts an RFC 6901 pointer into an RFC 9535 normalized path,
// using the document to tell an array index from a member name.
func normalizedPath(root *yaml.Node, pointer string) (string, error) {
	tokens, err := ParsePointer(pointer)
	if err != nil {
		return "", err
	}
	path := "$"
	node := root
	for _, token := range tokens {
		switch node.Kind {
		case yaml.SequenceNode:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node.Content) {
				return "", fmt.Errorf("pointer %s does not resolve: %q is not a valid index", pointerForDisplay(pointer), token)
			}
			path += "[" + token + "]"
			node = node.Content[index]
		case yaml.MappingNode:
			idx := findKey(node, token)
			if idx < 0 {
				return "", fmt.Errorf("pointer %s does not resolve: no key %q", pointerForDisplay(pointer), token)
			}
			path += "[" + quoteMember(token) + "]"
			node = node.Content[idx+1]
		default:
			return "", fmt.Errorf("pointer %s does not resolve: %q is not a mapping or sequence", pointerForDisplay(pointer), token)
		}
	}
	return path, nil
}

// quoteMember renders a member name as an RFC 9535 single-quoted string.
func quoteMember(name string) string {
	quoted := []byte{'\''}
	for _, r := range name {
		switch {
		case r == '\'' || r == '\\':
			quoted = append(quoted, '\\', byte(r))
		case r < 0x20:
			quoted = fmt.Appendf(quoted, `\u%04x`, r)
		default:
			quoted = utf8.AppendRune(quoted, r)
		}
	}
	return string(append(quoted, '\''))
}

// pathTokens parses a normalized path written by normalizedPath back into
// pointer tokens. Any other JSONPath is rejected: an exact operation addresses
// exactly one node.
func pathTokens(path string) ([]string, error) {
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("target %q is not a normalized path", path)
	}
	rest := path[1:]
	var tokens []string
	for rest != "" {
		if rest[0] != '[' {
			return nil, fmt.Errorf("target %q is not a normalized path", path)
		}
		rest = rest[1:]
		if rest != "" && rest[0] == '\'' {
			var token []byte
			i := 1
			closed := false
			for i < len(rest) {
				c := rest[i]
				if c == '\\' && i+1 < len(rest) {
					switch rest[i+1] {
					case '\'', '\\':
						token = append(token, rest[i+1])
						i += 2
						continue
					case 'u':
						if i+6 <= len(rest) {
							code, err := strconv.ParseUint(rest[i+2:i+6], 16, 32)
							if err == nil {
								token = utf8.AppendRune(token, rune(code))
								i += 6
								continue
							}
						}
					}
					return nil, fmt.Errorf("target %q has an invalid escape", path)
				}
				if c == '\'' {
					closed = true
					break
				}
				token = append(token, c)
				i++
			}
			if !closed || i+1 >= len(rest) || rest[i+1] != ']' {
				return nil, fmt.Errorf("target %q is not a normalized path", path)
			}
			tokens = append(tokens, string(token))
			rest = rest[i+2:]
			continue
		}
		end := strings.IndexByte(rest, ']')
		if end <= 0 {
			return nil, fmt.Errorf("target %q is not a normalized path", path)
		}
		if _, err := strconv.ParseUint(rest[:end], 10, 64); err != nil {
			return nil, fmt.Errorf("target %q is not a normalized path", path)
		}
		tokens = append(tokens, rest[:end])
		rest = rest[end+1:]
	}
	return tokens, nil
}

// Parse reads an artifact written by Render. It is strict: an
// action whose shape Render does not produce, an unknown extension, a
// duplicate key or a trailing YAML document is an error, because the producer
// and the consumer then disagree. Parsed operations go through Add, so a
// duplicate, contradictory or overlapping set is rejected here.
func Parse(data []byte) (*Overlay, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	root := &yaml.Node{}
	if err := dec.Decode(root); err != nil {
		return nil, fmt.Errorf("parsing overlay: %w", err)
	}
	var trailing yaml.Node
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing overlay: unexpected trailing YAML document")
	}
	if err := rejectDuplicateKeys(root); err != nil {
		return nil, fmt.Errorf("parsing overlay: %w", err)
	}
	var doc overlay.Overlay
	if err := root.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsing overlay: %w", err)
	}
	if doc.Version != StandardOverlayVersion {
		return nil, fmt.Errorf("parsing overlay: unsupported overlay version %q, expected %s", doc.Version, StandardOverlayVersion)
	}
	if doc.JSONPathVersion != "" {
		return nil, fmt.Errorf("parsing overlay: unexpected x-speakeasy-jsonpath; Overlay %s uses RFC 9535", StandardOverlayVersion)
	}
	ov := &Overlay{}
	if err := parseMetadata(doc.Extensions, ov); err != nil {
		return nil, err
	}
	for i, a := range doc.Actions {
		if err := parseAction(ov, a); err != nil {
			return nil, fmt.Errorf("parsing overlay: action %d (%s): %w", i, a.Target, err)
		}
	}
	ov.Normalize()
	return ov, nil
}

func parseMetadata(ext overlay.Extensions, ov *Overlay) error {
	for key := range ext {
		if key != extensionMetadata {
			return fmt.Errorf("parsing overlay: unknown extension %q", key)
		}
	}
	meta, ok := ext[extensionMetadata].(map[string]any)
	if !ok {
		return fmt.Errorf("parsing overlay: %s is required and must be a mapping", extensionMetadata)
	}
	for key, value := range meta {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("parsing overlay: %s.%s must be a string", extensionMetadata, key)
		}
		switch key {
		case "source":
			ov.Source = text
		case "kind":
			switch Kind(text) {
			case KindService, KindPackage:
				ov.Kind = Kind(text)
			default:
				return fmt.Errorf("parsing overlay: unknown kind %q", text)
			}
		default:
			return fmt.Errorf("parsing overlay: unknown field %s.%s", extensionMetadata, key)
		}
	}
	return nil
}

func parseAction(ov *Overlay, a overlay.Action) error {
	if a.Copy != "" || a.Description != "" {
		return fmt.Errorf("copy and description are not produced by Render")
	}
	if a.Remove == !a.Update.IsZero() {
		return fmt.Errorf("an action is either an update or a remove")
	}
	var provenance string
	replace := false
	for key, value := range a.Extensions {
		switch key {
		case extensionProvenance:
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("%s must be a string", key)
			}
			provenance = text
		case extensionReplace:
			flag, ok := value.(bool)
			if !ok || !flag {
				return fmt.Errorf("%s must be true when present", key)
			}
			replace = true
		default:
			return fmt.Errorf("unknown extension %q", key)
		}
	}
	if replace {
		// The remove half of a set; the update that follows carries the set.
		if !a.Remove || provenance != "" {
			return fmt.Errorf("%s marks only the remove half of a set", extensionReplace)
		}
		_, err := pathTokens(a.Target)
		return err
	}
	tokens, err := pathTokens(a.Target)
	if err != nil {
		return err
	}
	op := Operation{Provenance: provenance}
	if a.Remove {
		if len(tokens) == 0 {
			return fmt.Errorf("a remove cannot target the document root")
		}
		op.Pointer, op.Key, op.Mode = JoinPointer(tokens[:len(tokens)-1]...), tokens[len(tokens)-1], ModeRemove
	} else {
		key, value, err := singleEntry(&a.Update)
		if err != nil {
			return err
		}
		op.Pointer, op.Key, op.Value, op.Mode = JoinPointer(tokens...), key, value, ModeSet
	}
	op.Channel = channelOf(op.Key)
	return ov.Add(op)
}

func channelOf(key string) Channel {
	if strings.HasPrefix(key, "x-") {
		return ChannelExtension
	}
	return ChannelSchema
}

func singleEntry(update *yaml.Node) (string, *yaml.Node, error) {
	if update.Kind != yaml.MappingNode || len(update.Content) != 2 {
		return "", nil, fmt.Errorf("an update must set exactly one key")
	}
	return update.Content[0].Value, update.Content[1], nil
}

// rejectDuplicateKeys fails on a mapping that repeats a key. Decoding into a
// yaml.Node accepts one, and a duplicate must not silently take the last value.
func rejectDuplicateKeys(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if seen[key] {
				return fmt.Errorf("duplicate key %q at line %d", key, node.Content[i].Line)
			}
			seen[key] = true
		}
	}
	for _, child := range node.Content {
		if err := rejectDuplicateKeys(child); err != nil {
			return err
		}
	}
	return nil
}

// VerifyStandard applies the rendered artifact to doc with the Speakeasy
// Overlay implementation in strict mode and checks that the result equals
// Apply's. It proves that a consumer applying the artifact with standard
// tooling gets the document this package computes.
func VerifyStandard(doc []byte, ov *Overlay) error {
	want, err := Apply(doc, ov.Operations)
	if err != nil {
		return err
	}
	rendered, err := ov.Render(doc)
	if err != nil {
		return err
	}
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return fmt.Errorf("parsing document: %w", err)
	}
	if rendered != nil {
		standard, err := overlay.ParseReader(bytes.NewReader(rendered))
		if err != nil {
			return fmt.Errorf("verify: %w", err)
		}
		if _, err := standard.ApplyToStrict(root); err != nil {
			return fmt.Errorf("verify: standard application failed: %w", err)
		}
	}
	got, err := encode(root)
	if err != nil {
		return err
	}
	var a, b any
	if err := yaml.Unmarshal(want, &a); err != nil {
		return err
	}
	if err := yaml.Unmarshal(got, &b); err != nil {
		return err
	}
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("verify: the standard Overlay produces a different document than tfoverlay")
	}
	return nil
}

// applyOne applies one exact operation to target in place.
func applyOne(target *yaml.Node, op Operation) error {
	node, err := resolvePointer(target, op.Pointer)
	if err != nil {
		return newError(op, ownerLabel(op), "%s", err)
	}
	if op.Mode == ModeRemove {
		return removeKey(node, op)
	}
	return setKey(node, op)
}

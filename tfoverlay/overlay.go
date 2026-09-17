// Package tfoverlay defines the Terraform-scoped customization overlay that
// protoc-gen-apigw emits beside each OpenAPI document it generates.
//
// The overlay is a list of operations addressed by RFC 6901 JSON Pointer into
// the emitted OpenAPI document. Each operation sets or removes one key — an
// OpenAPI vendor extension (the extension channel) or a standard JSON Schema /
// OpenAPI keyword (the schema channel) — with a JSON value whose exact type is
// preserved.
//
// A consumer that merges several OpenAPI documents (for example the C1 spec
// aggregation) merges their overlays with Merge and applies the result with
// Apply immediately before invoking the Terraform generator, leaving the
// published OpenAPI document untouched.
package tfoverlay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is the overlay schema version written to and expected by this
// package.
const Version = 1

// Channel distinguishes the two annotation channels. The split exists so that
// a standard OpenAPI/JSON Schema keyword is never disguised as a vendor
// extension, and vice versa.
type Channel string

const (
	// ChannelExtension carries an OpenAPI vendor extension key ("x-...").
	ChannelExtension Channel = "extension"
	// ChannelSchema carries a standard OpenAPI/JSON Schema keyword.
	ChannelSchema Channel = "schema"
)

// Mode is how an operation is applied at its destination.
type Mode string

const (
	// ModeSet sets (replacing any existing value) the key at the destination.
	ModeSet Mode = "set"
	// ModeRemove removes the key from the destination.
	ModeRemove Mode = "remove"
)

// Kind identifies what document an overlay was generated for.
type Kind string

const (
	KindService Kind = "service"
	KindPackage Kind = "package"
)

// Operation is one customization applied at one destination.
type Operation struct {
	// Pointer is an RFC 6901 JSON Pointer, absolute in the emitted document.
	// The empty pointer addresses the document root.
	Pointer string
	// Channel is the annotation channel the operation came from.
	Channel Channel `yaml:"channel"`
	// Key is the extension key or schema keyword to set or remove.
	Key string `yaml:"key"`
	// Mode is ModeSet or ModeRemove.
	Mode Mode `yaml:"mode"`
	// Value is the value to set. It is nil for ModeRemove.
	Value *yaml.Node `yaml:",omitempty"`
}

// Overlay is a set of Operations for one emitted document.
type Overlay struct {
	// Source identifies the document owner: the service or proto package FQN.
	Source string
	// Kind is KindService or KindPackage.
	Kind Kind
	// Operations is kept sorted and deduplicated by Add.
	Operations []Operation
}

// Add records op, deduplicating an identical operation and rejecting a
// contradictory one.
//
// An operation is identical when the pointer, channel, key, mode and value all
// match. It is contradictory when the same pointer, channel and key are
// assigned a different mode or a different value: the generator has no
// documented precedence between two explicit assignments at the same
// destination, so it fails instead of picking one.
//
// Uniqueness of (pointer, channel, key) is not by itself enough for the result
// to be order-independent: writes at nested destinations interact. Setting the
// key "properties" on /components/schemas/X and setting "default" on
// /components/schemas/X/properties/foo both have unique keys, yet applying the
// first then the second keeps the second and applying them in the other order
// discards it. Add therefore also rejects an operation whose destination path
// (pointer tokens followed by the key) is a strict ancestor of another's, or a
// strict descendant, because no ordering can be shown to preserve both.
func (o *Overlay) Add(op Operation) error {
	if err := validateOperation(op); err != nil {
		return err
	}
	for i := range o.Operations {
		existing := &o.Operations[i]
		if existing.Pointer == op.Pointer && existing.Channel == op.Channel && existing.Key == op.Key {
			if existing.Mode == op.Mode && nodesEqual(existing.Value, op.Value) {
				return nil
			}
			return fmt.Errorf(
				"conflicting assignments at %s for %s: mode %s value %s vs mode %s value %s",
				pointerForDisplay(op.Pointer), op.Key,
				existing.Mode, displayValue(existing.Value),
				op.Mode, displayValue(op.Value),
			)
		}
		if existing.Channel == op.Channel && pathsOverlap(operationPath(*existing), operationPath(op)) {
			return fmt.Errorf(
				"overlapping destinations: %s at %s contains or is contained by %s at %s",
				existing.Key, pointerForDisplay(existing.Pointer),
				op.Key, pointerForDisplay(op.Pointer),
			)
		}
	}
	o.Operations = append(o.Operations, op)
	o.Normalize()
	return nil
}

// operationPath is the token path a write touches: the destination pointer's
// tokens, followed by the key it sets or removes.
func operationPath(op Operation) []string {
	tokens, err := ParsePointer(op.Pointer)
	if err != nil {
		return []string{op.Pointer, op.Key}
	}
	return append(tokens, op.Key)
}

// Overlaps reports whether two operations write to overlapping destinations on
// the same channel: the same destination and key, or one write path strictly
// contained in the other. Two overlapping operations cannot both be applied
// with an order-independent result, so callers that keep operations in separate
// lists (for example per scope) use this to reject them together.
func Overlaps(a, b Operation) bool {
	if a.Channel != b.Channel {
		return false
	}
	if a.Pointer == b.Pointer && a.Key == b.Key {
		return true
	}
	return pathsOverlap(operationPath(a), operationPath(b))
}

// pathsOverlap reports whether one token path is a strict ancestor of the
// other. Equal paths are not an overlap here; that case is the same-key
// conflict handled above.
func pathsOverlap(a, b []string) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return len(a) != len(b)
}

// Normalize sorts operations into the canonical order and drops duplicates.
// The order is stable and independent of proto declaration order or Go map
// iteration, so two runs over identical input produce identical bytes.
func (o *Overlay) Normalize() {
	sort.SliceStable(o.Operations, func(i, j int) bool {
		a, b := o.Operations[i], o.Operations[j]
		if a.Pointer != b.Pointer {
			return a.Pointer < b.Pointer
		}
		if a.Channel != b.Channel {
			return a.Channel < b.Channel
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		if a.Mode != b.Mode {
			return a.Mode < b.Mode
		}
		return displayValue(a.Value) < displayValue(b.Value)
	})
	out := o.Operations[:0]
	for i := range o.Operations {
		if len(out) > 0 {
			last := out[len(out)-1]
			cur := o.Operations[i]
			if last.Pointer == cur.Pointer && last.Channel == cur.Channel &&
				last.Key == cur.Key && last.Mode == cur.Mode && nodesEqual(last.Value, cur.Value) {
				continue
			}
		}
		out = append(out, o.Operations[i])
	}
	o.Operations = out
}

// Empty reports whether there is nothing to emit. An overlay with no
// operations produces no artifact, so a repository that has not adopted the
// annotations sees no change in generated output.
func (o *Overlay) Empty() bool { return o == nil || len(o.Operations) == 0 }

// Merge combines overlays (for example the per-service overlays of a merged
// document) into one. Identical operations deduplicate; contradictory ones
// fail. This is the cross-document check a per-file generator cannot perform:
// root-level annotations contributed by several services either agree or are
// rejected, never silently ordered.
func Merge(ovs ...*Overlay) (*Overlay, error) {
	merged := &Overlay{Kind: KindPackage}
	for _, ov := range ovs {
		if ov == nil {
			continue
		}
		if merged.Source == "" {
			merged.Source = ov.Source
			merged.Kind = ov.Kind
		}
		for _, op := range ov.Operations {
			if err := merged.Add(op); err != nil {
				return nil, fmt.Errorf("merging overlay from %s: %w", ov.Source, err)
			}
		}
	}
	return merged, nil
}

// Render serializes the overlay to the deterministic YAML artifact format.
func (o *Overlay) Render() ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setKV(root, "version", scalarNode("!!int", strconv.Itoa(Version)))
	if o.Source != "" {
		setKV(root, "source", scalarNode("!!str", o.Source))
	}
	if o.Kind != "" {
		setKV(root, "kind", scalarNode("!!str", string(o.Kind)))
	}
	ops := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, op := range o.Operations {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setKV(entry, "pointer", scalarNode("!!str", op.Pointer))
		setKV(entry, "channel", scalarNode("!!str", string(op.Channel)))
		setKV(entry, "key", scalarNode("!!str", op.Key))
		setKV(entry, "mode", scalarNode("!!str", string(op.Mode)))
		if op.Mode == ModeSet {
			value := op.Value
			if value == nil {
				value = scalarNode("!!null", "null")
			}
			setKV(entry, "value", value)
		}
		ops.Content = append(ops.Content, entry)
	}
	setKV(root, "operations", ops)
	return encode(root)
}

// Parse reads an overlay artifact written by Render.
func Parse(data []byte) (*Overlay, error) {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(data, root); err != nil {
		return nil, fmt.Errorf("parsing overlay: %w", err)
	}
	doc := documentRoot(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parsing overlay: expected a mapping at the document root")
	}
	ov := &Overlay{}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key, value := doc.Content[i], doc.Content[i+1]
		switch key.Value {
		case "version":
			v, err := strconv.Atoi(value.Value)
			if err != nil {
				return nil, fmt.Errorf("parsing overlay: invalid version %q", value.Value)
			}
			if v != Version {
				return nil, fmt.Errorf("parsing overlay: unsupported version %d, expected %d", v, Version)
			}
		case "source":
			ov.Source = value.Value
		case "kind":
			ov.Kind = Kind(value.Value)
		case "operations":
			if value.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("parsing overlay: operations must be a sequence")
			}
			for _, entry := range value.Content {
				op, err := parseOperation(entry)
				if err != nil {
					return nil, err
				}
				ov.Operations = append(ov.Operations, op)
			}
		}
	}
	ov.Normalize()
	return ov, nil
}

func parseOperation(entry *yaml.Node) (Operation, error) {
	if entry.Kind != yaml.MappingNode {
		return Operation{}, fmt.Errorf("parsing overlay: each operation must be a mapping")
	}
	op := Operation{}
	for i := 0; i+1 < len(entry.Content); i += 2 {
		key, value := entry.Content[i], entry.Content[i+1]
		switch key.Value {
		case "pointer":
			op.Pointer = value.Value
		case "channel":
			op.Channel = Channel(value.Value)
		case "key":
			op.Key = value.Value
		case "mode":
			op.Mode = Mode(value.Value)
		case "value":
			op.Value = value
		}
	}
	if op.Mode == ModeRemove {
		op.Value = nil
	}
	return op, validateOperation(op)
}

func validateOperation(op Operation) error {
	if _, err := ParsePointer(op.Pointer); err != nil {
		return err
	}
	switch op.Channel {
	case ChannelExtension:
		if !strings.HasPrefix(op.Key, "x-") {
			return fmt.Errorf("channel %s requires an extension key starting with \"x-\", got %q", op.Channel, op.Key)
		}
	case ChannelSchema:
		if op.Key == "" {
			return fmt.Errorf("channel %s requires a schema keyword", op.Channel)
		}
		if strings.HasPrefix(op.Key, "x-") {
			return fmt.Errorf("channel %s must not carry the vendor extension key %q; use channel %s", op.Channel, op.Key, ChannelExtension)
		}
	default:
		return fmt.Errorf("unknown channel %q", op.Channel)
	}
	switch op.Mode {
	case ModeSet:
		if op.Value == nil {
			return fmt.Errorf("mode %s requires a value at %s for %s", op.Mode, pointerForDisplay(op.Pointer), op.Key)
		}
	case ModeRemove:
		if op.Value != nil {
			return fmt.Errorf("mode %s must not carry a value at %s for %s", op.Mode, pointerForDisplay(op.Pointer), op.Key)
		}
	default:
		return fmt.Errorf("unknown mode %q", op.Mode)
	}
	return nil
}

func setKV(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content, scalarNode("!!str", key), value)
}

func scalarNode(tag, value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
}

func encode(node *yaml.Node) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, fmt.Errorf("encoding overlay: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding overlay: %w", err)
	}
	return buf.Bytes(), nil
}

// ParseJSONValue parses s as strict JSON and returns it as a yaml.Node with the
// value's exact type preserved.
//
// It rejects malformed JSON, trailing content after the first value, duplicate
// object keys at any level, and an empty input. Integers stay integers and
// decimals stay decimals — the literal text is preserved rather than being
// round-tripped through float64 — and false, 0, "" and empty arrays/objects
// are all values, not absences. Object keys are sorted so the rendered overlay
// is byte-stable.
func ParseJSONValue(s string) (*yaml.Node, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	value, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		// Both a well-formed second value and trailing junk are trailing
		// content: value_json must hold exactly one JSON value.
		return nil, fmt.Errorf("unexpected trailing content after the JSON value")
	}
	return value, nil
}

func decodeValue(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err == io.EOF {
		return nil, fmt.Errorf("missing JSON value")
	}
	if err != nil {
		return nil, fmt.Errorf("parsing JSON value: %w", err)
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeObject(dec)
		case '[':
			return decodeArray(dec)
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", t)
		}
	case string:
		return scalarNode("!!str", t), nil
	case json.Number:
		return numberNode(string(t)), nil
	case bool:
		return scalarNode("!!bool", strconv.FormatBool(t)), nil
	case nil:
		return scalarNode("!!null", "null"), nil
	default:
		return nil, fmt.Errorf("unsupported JSON token %T", tok)
	}
}

func decodeObject(dec *json.Decoder) (*yaml.Node, error) {
	values := map[string]*yaml.Node{}
	keys := []string{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parsing JSON object: %w", err)
		}
		if delim, ok := tok.(json.Delim); ok && delim == '}' {
			break
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("parsing JSON object: expected a key, got %v", tok)
		}
		if _, dup := values[key]; dup {
			return nil, fmt.Errorf("duplicate object key %q", key)
		}
		value, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		values[key] = value
		keys = append(keys, key)
	}
	sort.Strings(keys)
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range keys {
		setKV(node, k, values[k])
	}
	return node, nil
}

func decodeArray(dec *json.Decoder) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parsing JSON array: %w", err)
		}
		if delim, ok := tok.(json.Delim); ok && delim == ']' {
			break
		}
		value, err := decodeFromToken(dec, tok)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, value)
	}
	return node, nil
}

func decodeFromToken(dec *json.Decoder, tok json.Token) (*yaml.Node, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeObject(dec)
		case '[':
			return decodeArray(dec)
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", t)
		}
	case string:
		return scalarNode("!!str", t), nil
	case json.Number:
		return numberNode(string(t)), nil
	case bool:
		return scalarNode("!!bool", strconv.FormatBool(t)), nil
	case nil:
		return scalarNode("!!null", "null"), nil
	default:
		return nil, fmt.Errorf("unsupported JSON token %T", tok)
	}
}

func numberNode(literal string) *yaml.Node {
	if strings.ContainsAny(literal, ".eE") {
		return scalarNode("!!float", literal)
	}
	return scalarNode("!!int", literal)
}

// ParsePointer splits an RFC 6901 JSON Pointer into its reference tokens. The
// empty pointer addresses the document root and yields no tokens.
func ParsePointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON Pointer %q: must be empty or start with \"/\"", pointer)
	}
	raw := strings.Split(pointer, "/")[1:]
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		unescaped, err := UnescapeToken(token)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON Pointer %q: %w", pointer, err)
		}
		tokens = append(tokens, unescaped)
	}
	return tokens, nil
}

// EscapeToken escapes one reference token for use in a JSON Pointer.
func EscapeToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}

// UnescapeToken reverses EscapeToken.
func UnescapeToken(token string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(token); i++ {
		if token[i] != '~' {
			out.WriteByte(token[i])
			continue
		}
		if i+1 >= len(token) {
			return "", fmt.Errorf("dangling \"~\" in reference token %q", token)
		}
		switch token[i+1] {
		case '0':
			out.WriteByte('~')
		case '1':
			out.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid escape \"~%c\" in reference token %q", token[i+1], token)
		}
		i++
	}
	return out.String(), nil
}

// JoinPointer builds a JSON Pointer from reference tokens, escaping each.
func JoinPointer(tokens ...string) string {
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, EscapeToken(token))
	}
	return "/" + strings.Join(parts, "/")
}

// Apply applies ops to a rendered OpenAPI document (YAML or JSON) and returns
// the modified document. Operations are applied in the order given, so a
// caller that merges per-service overlays gets a deterministic result.
//
// Applying an operation whose pointer does not resolve, or removing a key that
// is not present, is an error: an overlay that silently does nothing hides the
// drift it was meant to catch.
func Apply(doc []byte, ops []Operation) ([]byte, error) {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return nil, fmt.Errorf("parsing document: %w", err)
	}
	if err := ApplyToNode(root, ops); err != nil {
		return nil, err
	}
	return encode(root)
}

// ApplyToNode applies ops to a parsed document node in place.
func ApplyToNode(root *yaml.Node, ops []Operation) error {
	target := documentRoot(root)
	if target == nil {
		return fmt.Errorf("document has no root node")
	}
	for _, op := range ops {
		if err := validateOperation(op); err != nil {
			return err
		}
		node, err := resolvePointer(target, op.Pointer)
		if err != nil {
			return err
		}
		switch op.Mode {
		case ModeSet:
			if err := setKey(node, op); err != nil {
				return err
			}
		case ModeRemove:
			if err := removeKey(node, op); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate reports whether every op can be applied to doc, without modifying
// it. Use it to fail generation on an annotation whose destination does not
// exist in the emitted document.
func Validate(doc []byte, ops []Operation) error {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return fmt.Errorf("parsing document: %w", err)
	}
	target := documentRoot(root)
	if target == nil {
		return fmt.Errorf("document has no root node")
	}
	for i := range ops {
		op := &ops[i]
		if err := validateOperation(*op); err != nil {
			return err
		}
		node, err := resolvePointer(target, op.Pointer)
		if err != nil {
			return err
		}
		if op.Mode == ModeRemove && findKey(node, op.Key) < 0 {
			return fmt.Errorf("cannot remove %s at %s: key is not present", op.Key, pointerForDisplay(op.Pointer))
		}
	}
	return nil
}

func documentRoot(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil
		}
		return root.Content[0]
	}
	return root
}

func resolvePointer(root *yaml.Node, pointer string) (*yaml.Node, error) {
	tokens, err := ParsePointer(pointer)
	if err != nil {
		return nil, err
	}
	node := root
	for _, token := range tokens {
		switch node.Kind {
		case yaml.MappingNode:
			idx := findKey(node, token)
			if idx < 0 {
				return nil, fmt.Errorf("pointer %s does not resolve: no key %q", pointerForDisplay(pointer), token)
			}
			node = node.Content[idx+1]
		case yaml.SequenceNode:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node.Content) {
				return nil, fmt.Errorf("pointer %s does not resolve: %q is not a valid index", pointerForDisplay(pointer), token)
			}
			node = node.Content[index]
		default:
			return nil, fmt.Errorf("pointer %s does not resolve: %q is not a mapping or sequence", pointerForDisplay(pointer), token)
		}
	}
	return node, nil
}

func findKey(node *yaml.Node, key string) int {
	if node.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func setKey(node *yaml.Node, op Operation) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot set %s at %s: destination is not an object", op.Key, pointerForDisplay(op.Pointer))
	}
	if idx := findKey(node, op.Key); idx >= 0 {
		node.Content[idx+1] = op.Value
		return nil
	}
	node.Content = append(node.Content, scalarNode("!!str", op.Key), op.Value)
	return nil
}

func removeKey(node *yaml.Node, op Operation) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot remove %s at %s: destination is not an object", op.Key, pointerForDisplay(op.Pointer))
	}
	idx := findKey(node, op.Key)
	if idx < 0 {
		return fmt.Errorf("cannot remove %s at %s: key is not present", op.Key, pointerForDisplay(op.Pointer))
	}
	node.Content = append(node.Content[:idx], node.Content[idx+2:]...)
	return nil
}

func pointerForDisplay(pointer string) string {
	if pointer == "" {
		return "(document root)"
	}
	return pointer
}

func displayValue(node *yaml.Node) string {
	if node == nil {
		return "<nil>"
	}
	data, err := encode(node)
	if err != nil {
		return fmt.Sprintf("<%v>", err)
	}
	return strings.TrimSpace(string(data))
}

func nodesEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return displayValue(a) == displayValue(b)
}

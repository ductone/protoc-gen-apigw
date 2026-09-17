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
	"errors"
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
	// Provenance identifies the descriptor the operation came from, for
	// example "field c1.api.app.v1.AppEntitlement.provisioner_policy". It is
	// advisory metadata for diagnostics and linters; it is never interpreted.
	// It is empty for an operation that was not generated from a proto
	// annotation.
	Provenance string `yaml:"provenance,omitempty"`
}

// Error is a structured failure: which destination was involved, what key, and
// which annotations contributed. Linters and pipeline steps should match it
// with errors.As rather than parsing the message.
type Error struct {
	// Pointer is the destination the operation would have written to.
	Pointer string
	// Channel and Key identify the write.
	Channel Channel
	Key     string
	// Owners names the annotations that contributed to the failure, in the
	// order they were seen. It is empty when the failure has no known owner.
	Owners []string
	// Message is the human-readable detail.
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Owners) > 0 {
		return fmt.Sprintf("%s (owner(s): %s)", e.Message, strings.Join(e.Owners, ", "))
	}
	return e.Message
}

func newError(op Operation, owners []string, format string, args ...interface{}) *Error {
	return &Error{
		Pointer: op.Pointer,
		Channel: op.Channel,
		Key:     op.Key,
		Owners:  owners,
		Message: fmt.Sprintf(format, args...),
	}
}

func ownerLabel(op Operation) []string {
	if op.Provenance == "" {
		return nil
	}
	return []string{op.Provenance}
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
// contradictory or overlapping one.
//
// An operation is identical when the pointer, channel, key, mode and value all
// match, and is recorded once.
//
// Uniqueness of (pointer, channel, key) is not by itself enough for the result
// to be order-independent: writes at nested destinations interact, and they
// interact *across channels*. Setting the schema keyword `properties` on
// /components/schemas/X and setting `x-test` on
// /components/schemas/X/properties/foo both have unique keys, yet the first
// replaces the whole `properties` object and discards the second; applied in
// the other order the first discards the second's value instead. Add therefore
// rejects any operation whose destination path — the pointer's reference tokens
// followed by the key — is a strict ancestor or descendant of another's,
// whatever channel either is on. A `remove` is a write at the same location and
// overlaps the same way.
//
// Two operations that write the same key at the same destination are a
// hard error unless they are identical: the generator has no documented
// precedence between two explicit assignments at one destination.
func (o *Overlay) Add(op Operation) error {
	if err := validateOperation(op); err != nil {
		return err
	}
	for i := range o.Operations {
		existing := &o.Operations[i]
		if existing.Pointer == op.Pointer && existing.Key == op.Key {
			if existing.Channel == op.Channel && existing.Mode == op.Mode && nodesEqual(existing.Value, op.Value) {
				return nil
			}
			return newError(op, append(ownerLabel(*existing), ownerLabel(op)...),
				"conflicting assignments at %s for %s: mode %s value %s vs mode %s value %s",
				pointerForDisplay(op.Pointer), op.Key,
				existing.Mode, displayValue(existing.Value),
				op.Mode, displayValue(op.Value),
			)
		}
		if pathsOverlap(operationPath(*existing), operationPath(op)) {
			return newError(op, append(ownerLabel(*existing), ownerLabel(op)...),
				"overlapping destinations: %s at %s contains or is contained by %s at %s",
				existing.Key, pointerForDisplay(existing.Pointer),
				op.Key, pointerForDisplay(op.Pointer),
			)
		}
	}
	o.Operations = append(o.Operations, op)
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

// Overlaps reports whether two operations write to overlapping destinations:
// the same destination and key, or one write path strictly contained in the
// other. Channels are deliberately ignored for the containment case — a write
// to an ancestor destroys its descendants whatever channel either is on — so
// callers that keep operations in separate lists (for example per scope) use
// this to reject them together.
func Overlaps(a, b Operation) bool {
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
// Add appends in call order, so call this (or Render, which sorts a copy) to
// get the canonical order. The order is stable and independent of proto
// declaration order or Go map iteration, so two runs over identical input
// produce identical bytes.
func (o *Overlay) Normalize() {
	sortOperations(o.Operations)
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

func sortOperations(ops []Operation) {
	sort.SliceStable(ops, func(i, j int) bool {
		a, b := ops[i], ops[j]
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
}

// Empty reports whether there is nothing to emit. An overlay with no
// operations produces no artifact, so a repository that has not adopted the
// annotations sees no change in generated output.
func (o *Overlay) Empty() bool { return o == nil || len(o.Operations) == 0 }

// Merge combines overlays (for example the per-service overlays of a merged
// document) into one. Identical operations deduplicate; contradictory and
// overlapping ones fail with both contributing sources named. This is the
// cross-document check a per-file generator cannot perform: root-level
// annotations contributed by several services either agree or are rejected,
// never silently ordered.
func Merge(ovs ...*Overlay) (*Overlay, error) {
	merged := &Overlay{Kind: KindPackage}
	// origin tracks which overlay contributed each recorded operation, so a
	// failure can name both sides rather than only the incoming one.
	origin := map[string]string{}
	for _, ov := range ovs {
		if ov == nil {
			continue
		}
		if merged.Source == "" {
			merged.Source = ov.Source
			merged.Kind = ov.Kind
		}
		for _, op := range ov.Operations {
			before := len(merged.Operations)
			if err := merged.Add(op); err != nil {
				var structured *Error
				if errors.As(err, &structured) && len(structured.Owners) < 2 {
					structured.Owners = append(structured.Owners, ov.Source)
					structured.Message = fmt.Sprintf(
						"merging %s into %s: %s", ov.Source, merged.Source, structured.Message)
				}
				if prev, ok := origin[opOriginKey(op)]; ok && prev != ov.Source {
					return nil, fmt.Errorf("merging %s into %s: %w", ov.Source, prev, err)
				}
				return nil, err
			}
			if len(merged.Operations) > before {
				origin[opOriginKey(op)] = ov.Source
			}
		}
	}
	return merged, nil
}

func opOriginKey(op Operation) string {
	return op.Pointer + "|" + string(op.Channel) + "|" + op.Key
}

// Render serializes the overlay to the deterministic YAML artifact format.
// Operations are sorted into canonical order for rendering; the receiver is
// not modified, so rendering is repeatable and side-effect free.
func (o *Overlay) Render() ([]byte, error) {
	sorted := make([]Operation, len(o.Operations))
	copy(sorted, o.Operations)
	sortOperations(sorted)

	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setKV(root, "version", scalarNode("!!int", strconv.Itoa(Version)))
	if o.Source != "" {
		setKV(root, "source", scalarNode("!!str", o.Source))
	}
	if o.Kind != "" {
		setKV(root, "kind", scalarNode("!!str", string(o.Kind)))
	}
	ops := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, op := range sorted {
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
		if op.Provenance != "" {
			setKV(entry, "provenance", scalarNode("!!str", op.Provenance))
		}
		ops.Content = append(ops.Content, entry)
	}
	setKV(root, "operations", ops)
	return encode(root)
}

// Parse reads an overlay artifact written by Render.
//
// It is strict on purpose: an artifact is machine-written, so a missing field,
// an unknown field, a value of the wrong YAML type or a duplicate operation
// means the producer and the consumer disagree, not that a default applies.
// Parsed operations go through the same Add validation as generated ones, so a
// duplicate, contradictory or overlapping set is rejected here rather than at
// Apply time.
func Parse(data []byte) (*Overlay, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	root := &yaml.Node{}
	if err := dec.Decode(root); err != nil {
		return nil, fmt.Errorf("parsing overlay: %w", err)
	}
	// Decoding into a yaml.Node does not reject a second document, and a
	// silently-ignored one is a producer/consumer disagreement.
	var trailing yaml.Node
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing overlay: unexpected trailing YAML document")
	}
	doc := documentRoot(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parsing overlay: expected a mapping at the document root")
	}
	ov := &Overlay{}
	haveVersion, haveOperations := false, false
	// yaml.Unmarshal into a Node does not detect a duplicate mapping key, so
	// the metadata fields need an explicit check: a duplicate must not silently
	// take the last value.
	seenMetadata := map[string]bool{}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key, value := doc.Content[i], doc.Content[i+1]
		if seenMetadata[key.Value] {
			return nil, fmt.Errorf("parsing overlay: duplicate field %q", key.Value)
		}
		seenMetadata[key.Value] = true
		switch key.Value {
		case "version":
			version, err := intScalar(value, "version")
			if err != nil {
				return nil, err
			}
			if version != Version {
				return nil, fmt.Errorf("parsing overlay: unsupported version %d, expected %d", version, Version)
			}
			haveVersion = true
		case "source":
			source, err := stringScalar(value, "source")
			if err != nil {
				return nil, err
			}
			ov.Source = source
		case "kind":
			kind, err := stringScalar(value, "kind")
			if err != nil {
				return nil, err
			}
			switch Kind(kind) {
			case KindService, KindPackage:
				ov.Kind = Kind(kind)
			default:
				return nil, fmt.Errorf("parsing overlay: unknown kind %q", kind)
			}
		case "operations":
			if value.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("parsing overlay: operations must be a sequence")
			}
			for _, entry := range value.Content {
				op, err := parseOperation(entry)
				if err != nil {
					return nil, err
				}
				if err := ov.Add(op); err != nil {
					return nil, fmt.Errorf("parsing overlay: %w", err)
				}
			}
			haveOperations = true
		default:
			return nil, fmt.Errorf("parsing overlay: unknown field %q", key.Value)
		}
	}
	if !haveVersion {
		return nil, fmt.Errorf("parsing overlay: version is required")
	}
	if !haveOperations {
		return nil, fmt.Errorf("parsing overlay: operations is required")
	}
	ov.Normalize()
	return ov, nil
}

func parseOperation(entry *yaml.Node) (Operation, error) {
	if entry.Kind != yaml.MappingNode {
		return Operation{}, fmt.Errorf("parsing overlay: each operation must be a mapping")
	}
	op := Operation{}
	havePointer, haveChannel, haveKey, haveMode, haveValue := false, false, false, false, false
	seen := map[string]bool{}
	for i := 0; i+1 < len(entry.Content); i += 2 {
		key, value := entry.Content[i], entry.Content[i+1]
		if seen[key.Value] {
			return Operation{}, fmt.Errorf("parsing overlay: duplicate operation field %q", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "pointer":
			pointer, err := stringScalar(value, "pointer")
			if err != nil {
				return Operation{}, err
			}
			op.Pointer = pointer
			havePointer = true
		case "channel":
			channel, err := stringScalar(value, "channel")
			if err != nil {
				return Operation{}, err
			}
			op.Channel = Channel(channel)
			haveChannel = true
		case "key":
			opKey, err := stringScalar(value, "key")
			if err != nil {
				return Operation{}, err
			}
			op.Key = opKey
			haveKey = true
		case "mode":
			mode, err := stringScalar(value, "mode")
			if err != nil {
				return Operation{}, err
			}
			op.Mode = Mode(mode)
			haveMode = true
		case "provenance":
			provenance, err := stringScalar(value, "provenance")
			if err != nil {
				return Operation{}, err
			}
			op.Provenance = provenance
		case "value":
			op.Value = value
			haveValue = true
		default:
			return Operation{}, fmt.Errorf("parsing overlay: unknown operation field %q", key.Value)
		}
	}
	// A missing pointer is a different artifact from an explicit empty pointer,
	// which legitimately addresses the document root.
	if !havePointer {
		return Operation{}, fmt.Errorf("parsing overlay: operation is missing pointer")
	}
	for _, required := range []struct {
		present bool
		name    string
	}{
		{haveChannel, "channel"},
		{haveKey, "key"},
		{haveMode, "mode"},
	} {
		if !required.present {
			return Operation{}, fmt.Errorf(
				"parsing overlay: operation %s is missing %s", pointerForDisplay(op.Pointer), required.name)
		}
	}
	switch op.Mode {
	case ModeSet:
		if !haveValue {
			return Operation{}, fmt.Errorf(
				"parsing overlay: operation %s is missing value", pointerForDisplay(op.Pointer))
		}
	case ModeRemove:
		if haveValue {
			return Operation{}, fmt.Errorf(
				"parsing overlay: remove operation %s must not carry a value", pointerForDisplay(op.Pointer))
		}
	}
	return op, validateOperation(op)
}

// stringScalar reads a required string field, rejecting a YAML value of any
// other type so a malformed artifact cannot silently become a string.
func stringScalar(node *yaml.Node, field string) (string, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", fmt.Errorf("parsing overlay: field %q must be a string, got %s", field, nodeKind(node))
	}
	return node.Value, nil
}

func intScalar(node *yaml.Node, field string) (int, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, fmt.Errorf("parsing overlay: field %q must be an integer, got %s", field, nodeKind(node))
	}
	value, err := strconv.Atoi(node.Value)
	if err != nil {
		return 0, fmt.Errorf("parsing overlay: field %q is not an integer: %q", field, node.Value)
	}
	return value, nil
}

func nodeKind(node *yaml.Node) string {
	if node == nil {
		return "nothing"
	}
	if node.Tag != "" {
		return node.Tag
	}
	switch node.Kind {
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a sequence"
	default:
		return "a scalar"
	}
}

func validateOperation(op Operation) error {
	if _, err := ParsePointer(op.Pointer); err != nil {
		return newError(op, ownerLabel(op), "%s", err)
	}
	switch op.Channel {
	case ChannelExtension:
		if !strings.HasPrefix(op.Key, "x-") {
			return newError(op, ownerLabel(op),
				"channel %s requires an extension key starting with \"x-\", got %q", op.Channel, op.Key)
		}
	case ChannelSchema:
		if op.Key == "" {
			return newError(op, ownerLabel(op), "channel %s requires a schema keyword", op.Channel)
		}
		if strings.HasPrefix(op.Key, "x-") {
			return newError(op, ownerLabel(op),
				"channel %s must not carry the vendor extension key %q; use channel %s",
				op.Channel, op.Key, ChannelExtension)
		}
	default:
		return newError(op, ownerLabel(op), "unknown channel %q", op.Channel)
	}
	switch op.Mode {
	case ModeSet:
		if op.Value == nil {
			return newError(op, ownerLabel(op),
				"mode %s requires a value at %s for %s", op.Mode, pointerForDisplay(op.Pointer), op.Key)
		}
	case ModeRemove:
		if op.Value != nil {
			return newError(op, ownerLabel(op),
				"mode %s must not carry a value at %s for %s", op.Mode, pointerForDisplay(op.Pointer), op.Key)
		}
	default:
		return newError(op, ownerLabel(op), "unknown mode %q", op.Mode)
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
// the modified document.
//
// The list is validated as a whole first: an operation list that contains a
// duplicate, a contradiction or an overlapping pair is rejected, so a caller
// cannot get a silently order-dependent result by forgetting to Merge. For
// non-overlapping operations the result does not depend on order.
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

// ApplyToNode applies ops to a parsed document node in place. Like Apply it
// validates the whole list and applies its canonical form.
func ApplyToNode(root *yaml.Node, ops []Operation) error {
	normalized, err := normalizeOperationSet(ops)
	if err != nil {
		return err
	}
	target := documentRoot(root)
	if target == nil {
		return fmt.Errorf("document has no root node")
	}
	for _, op := range normalized {
		node, err := resolvePointer(target, op.Pointer)
		if err != nil {
			return newError(op, ownerLabel(op), "%s", err)
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

// normalizeOperationSet runs ops through the same conflict and overlap checks
// Add applies, then returns the list every exported entrypoint actually
// operates on: deduplicated and in canonical order.
//
// Return-and-apply is the point. Validating a list and then applying a
// *different* one would let two identical remove operations pass validation and
// then fail — the first removes the key, the second finds it gone — even though
// the contract says identical operations deduplicate.
func normalizeOperationSet(ops []Operation) ([]Operation, error) {
	check := &Overlay{}
	for i := range ops {
		if err := check.Add(ops[i]); err != nil {
			return nil, fmt.Errorf("inconsistent operation list: %w", err)
		}
	}
	check.Normalize()
	return check.Operations, nil
}

// Validate reports whether every op can be applied to doc, without modifying
// it. It performs the same checks Apply does, including the whole-list
// consistency check and the requirement that the destination is an object, so
// a list that Validate accepts is a list Apply can perform.
func Validate(doc []byte, ops []Operation) error {
	normalized, err := normalizeOperationSet(ops)
	if err != nil {
		return err
	}
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return fmt.Errorf("parsing document: %w", err)
	}
	target := documentRoot(root)
	if target == nil {
		return fmt.Errorf("document has no root node")
	}
	for i := range normalized {
		op := normalized[i]
		node, err := resolvePointer(target, op.Pointer)
		if err != nil {
			return newError(op, ownerLabel(op), "%s", err)
		}
		if node.Kind != yaml.MappingNode {
			return newError(op, ownerLabel(op),
				"cannot write %s at %s: destination is not an object (%s)",
				op.Key, pointerForDisplay(op.Pointer), nodeKind(node))
		}
		if op.Mode == ModeRemove && findKey(node, op.Key) < 0 {
			return newError(op, ownerLabel(op),
				"cannot remove %s at %s: key is not present", op.Key, pointerForDisplay(op.Pointer))
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
		return newError(op, ownerLabel(op),
			"cannot set %s at %s: destination is not an object (%s)",
			op.Key, pointerForDisplay(op.Pointer), nodeKind(node))
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
		return newError(op, ownerLabel(op),
			"cannot remove %s at %s: destination is not an object (%s)",
			op.Key, pointerForDisplay(op.Pointer), nodeKind(node))
	}
	idx := findKey(node, op.Key)
	if idx < 0 {
		return newError(op, ownerLabel(op),
			"cannot remove %s at %s: key is not present", op.Key, pointerForDisplay(op.Pointer))
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

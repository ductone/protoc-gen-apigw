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
//
// Entity-scoped operations are rendered into a separate document. A generic
// Overlay tool would apply them to every entity, so only ApplyTerraform may
// consume that part.
const (
	// StandardOverlayVersion is the Overlay Specification version Render emits.
	StandardOverlayVersion = "1.1.0"

	// ArtifactSuffix and EntityArtifactSuffix name the two rendered parts beside
	// an OpenAPI document: <base>.<owner>.<suffix>.
	ArtifactSuffix       = "terraform_overlay.yaml"
	EntityArtifactSuffix = "terraform_entity_overlay.yaml"

	extensionMetadata   = "x-apigw-terraform"
	extensionProvenance = "x-apigw-provenance"
	extensionEntity     = "x-apigw-entity"
	extensionReplace    = "x-apigw-replace"
)

// Part says which rendered document an overlay artifact is.
type Part string

const (
	// PartGlobal holds global operations. It is a standard
	// Overlay any tool can apply.
	PartGlobal Part = "global"
	// PartEntities holds entity-scoped operations, for ApplyTerraform only.
	PartEntities Part = "entities"
)

// Rendered is the pair of artifacts Render produces. A part with nothing to
// apply is nil and is not emitted.
type Rendered struct {
	Global   []byte
	Entities []byte
}

// Render serializes the overlay against doc, the document it applies to. The
// document decides how a set is spelled: an update alone when the key is
// absent or holds a scalar of the same type, otherwise a remove followed by an
// update. Rendering does not modify the receiver or doc.
//
// Use VerifyStandard on the document the overlay is finally applied to.
func (o *Overlay) Render(doc []byte) (*Rendered, error) {
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

	out := &Rendered{}
	var global, scoped []overlay.Action
	// Simulate the global part so each set is spelled against the value its
	// destination holds when the action runs.
	for _, op := range ops {
		if op.Entity != nil {
			continue
		}
		actions, err := operationActions(target, op)
		if err != nil {
			return nil, err
		}
		global = append(global, actions...)
		if err := applyOne(target, op); err != nil {
			return nil, err
		}
	}
	// Entity operations apply to copies of the document the global part
	// produced, so they are spelled against it too.
	for _, op := range ops {
		if op.Entity == nil {
			continue
		}
		actions, err := operationActions(target, op)
		if err != nil {
			return nil, err
		}
		scoped = append(scoped, actions...)
	}
	var err error
	if len(global) > 0 {
		if out.Global, err = o.encodeStandard(PartGlobal, global); err != nil {
			return nil, err
		}
	}
	if len(scoped) > 0 {
		if out.Entities, err = o.encodeStandard(PartEntities, scoped); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (o *Overlay) encodeStandard(part Part, actions []overlay.Action) ([]byte, error) {
	title := "Terraform customizations"
	if o.Source != "" {
		title = o.Source + " Terraform customizations"
	}
	if part == PartEntities {
		title += " (entity-scoped; apply with tfoverlay only)"
	}
	meta := map[string]any{"part": string(part)}
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
	if op.Entity != nil {
		entity := map[string]any{"name": op.Entity.Name}
		if op.Entity.Kind != EntityKindAny {
			entity["kind"] = string(op.Entity.Kind)
		}
		ext[extensionEntity] = entity
	}
	keyPath := path + "[" + quoteMember(op.Key) + "]"
	if op.Mode == ModeRemove {
		return []overlay.Action{{Target: keyPath, Remove: true, Extensions: ext}}, nil
	}
	var actions []overlay.Action
	if idx := findKey(node, op.Key); idx >= 0 && !replacesInPlace(node.Content[idx+1], op.Value) {
		replace := overlay.Extensions{extensionReplace: true}
		if op.Entity != nil {
			replace[extensionEntity] = ext[extensionEntity]
		}
		actions = append(actions, overlay.Action{Target: keyPath, Remove: true, Extensions: replace})
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

// Parse reads an artifact written by Render, either part. It is strict: an
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
	part, err := parseMetadata(doc.Extensions, ov)
	if err != nil {
		return nil, err
	}
	for i, a := range doc.Actions {
		if err := parseAction(ov, part, a); err != nil {
			return nil, fmt.Errorf("parsing overlay: action %d (%s): %w", i, a.Target, err)
		}
	}
	ov.Normalize()
	return ov, nil
}

func parseMetadata(ext overlay.Extensions, ov *Overlay) (Part, error) {
	for key := range ext {
		if key != extensionMetadata {
			return "", fmt.Errorf("parsing overlay: unknown extension %q", key)
		}
	}
	meta, ok := ext[extensionMetadata].(map[string]any)
	if !ok {
		return "", fmt.Errorf("parsing overlay: %s is required and must be a mapping", extensionMetadata)
	}
	var part Part
	for key, value := range meta {
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("parsing overlay: %s.%s must be a string", extensionMetadata, key)
		}
		switch key {
		case "part":
			part = Part(text)
		case "source":
			ov.Source = text
		case "kind":
			switch Kind(text) {
			case KindService, KindPackage:
				ov.Kind = Kind(text)
			default:
				return "", fmt.Errorf("parsing overlay: unknown kind %q", text)
			}
		default:
			return "", fmt.Errorf("parsing overlay: unknown field %s.%s", extensionMetadata, key)
		}
	}
	if part != PartGlobal && part != PartEntities {
		return "", fmt.Errorf("parsing overlay: %s.part must be %q or %q", extensionMetadata, PartGlobal, PartEntities)
	}
	return part, nil
}

func parseAction(ov *Overlay, part Part, a overlay.Action) error {
	if a.Copy != "" || a.Description != "" {
		return fmt.Errorf("copy and description are not produced by Render")
	}
	if a.Remove == !a.Update.IsZero() {
		return fmt.Errorf("an action is either an update or a remove")
	}
	var provenance string
	var entity *EntityScope
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
		case extensionEntity:
			e, err := parseEntityExtension(value)
			if err != nil {
				return err
			}
			entity = e
		default:
			return fmt.Errorf("unknown extension %q", key)
		}
	}
	if (entity != nil) != (part == PartEntities) {
		return fmt.Errorf("entity-scoped actions belong in the %s part, and only there", PartEntities)
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
	op := Operation{Provenance: provenance, Entity: entity}
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

func parseEntityExtension(value any) (*EntityScope, error) {
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a mapping", extensionEntity)
	}
	e := &EntityScope{}
	for key, raw := range fields {
		text, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s.%s must be a string", extensionEntity, key)
		}
		switch key {
		case "name":
			e.Name = text
		case "kind":
			e.Kind = EntityKind(text)
			if e.Kind == EntityKindAny {
				return nil, fmt.Errorf("%s.kind must be %q or %q; omit it to address both",
					extensionEntity, EntityKindResource, EntityKindDataSource)
			}
		default:
			return nil, fmt.Errorf("unknown field %s.%s", extensionEntity, key)
		}
	}
	if err := validateEntity(e); err != nil {
		return nil, err
	}
	return e, nil
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

// VerifyStandard applies the rendered global part to doc with the Speakeasy
// Overlay implementation in strict mode and checks that the result equals
// Apply's. It proves that a consumer applying the artifact with standard
// tooling gets the document this package computes. The overlay must not hold
// entity-scoped operations.
func VerifyStandard(doc []byte, ov *Overlay) error {
	for _, op := range ov.Operations {
		if op.Entity != nil {
			return fmt.Errorf("verify: entity-scoped operations have no standard form")
		}
	}
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
	if rendered.Global != nil {
		standard, err := overlay.ParseReader(bytes.NewReader(rendered.Global))
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

// Global returns a copy of the overlay without its entity-scoped operations:
// the part standard tooling can apply.
func (o *Overlay) Global() *Overlay {
	out := &Overlay{Source: o.Source, Kind: o.Kind}
	for _, op := range o.Operations {
		if op.Entity == nil {
			out.Operations = append(out.Operations, op)
		}
	}
	return out
}

// ProjectionOverlay expresses the change from before to after — for example
// from a spec to ApplyTerraform's projection of it — as a standard Overlay
// document (the Speakeasy comparison emits Overlay 1.0.0 with RFC 9535
// JSONPath), and checks that applying it to before yields after.
func ProjectionOverlay(title string, before, after []byte) ([]byte, error) {
	var from, to yaml.Node
	if err := yaml.Unmarshal(before, &from); err != nil {
		return nil, fmt.Errorf("parsing document: %w", err)
	}
	if err := yaml.Unmarshal(after, &to); err != nil {
		return nil, fmt.Errorf("parsing projected document: %w", err)
	}
	diff, err := overlay.Compare(title, documentRoot(&from), *documentRoot(&to))
	if err != nil {
		return nil, fmt.Errorf("comparing documents: %w", err)
	}
	text, err := diff.ToString()
	if err != nil {
		return nil, err
	}
	check := &yaml.Node{}
	if err := yaml.Unmarshal(before, check); err != nil {
		return nil, err
	}
	if len(diff.Actions) > 0 {
		if _, err := diff.ApplyToStrict(check); err != nil {
			return nil, fmt.Errorf("projection overlay does not apply: %w", err)
		}
	}
	var got, want any
	if err := check.Decode(&got); err != nil {
		return nil, err
	}
	if err := to.Decode(&want); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("projection overlay does not reproduce the projected document")
	}
	return []byte(text), nil
}

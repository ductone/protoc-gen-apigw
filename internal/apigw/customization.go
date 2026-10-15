package apigw

import (
	"fmt"
	"sort"
	"strings"

	pgs "github.com/lyft/protoc-gen-star"
	"gopkg.in/yaml.v3"

	apigw_v1 "github.com/ductone/protoc-gen-apigw/apigw/v1"
	"github.com/ductone/protoc-gen-apigw/tfoverlay"
)

// This file implements the transport half of docs/terraform-overlay-contract.md.
//
// Annotations are resolved to absolute JSON Pointers into the emitted OpenAPI
// document and recorded as tfoverlay operations. Nothing here mutates the
// OpenAPI model: a Terraform-scoped override never touches the shared document,
// and a shared-scoped override is applied to the rendered document afterwards,
// through the same apply path an external consumer uses.

// tfOwnerKind identifies which annotation owner a declaration came from. It
// determines the default target and the legal target set.
type tfOwnerKind int

const (
	tfOwnerFile tfOwnerKind = iota
	tfOwnerService
	tfOwnerOperation
	tfOwnerMessage
	tfOwnerField
)

func (k tfOwnerKind) String() string {
	switch k {
	case tfOwnerFile:
		return "file"
	case tfOwnerService:
		return "service"
	case tfOwnerOperation:
		return "operation"
	case tfOwnerMessage:
		return "message"
	case tfOwnerField:
		return "field"
	default:
		return "unknown"
	}
}

// defaultValue returns the target used when an annotation leaves target
// unspecified.
func (k tfOwnerKind) defaultValue() apigw_v1.OpenAPICustomizationTarget {
	switch k {
	case tfOwnerService, tfOwnerFile:
		return apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_DOCUMENT_ROOT
	case tfOwnerOperation:
		return apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_OPERATION
	default:
		return apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA
	}
}

// legalTargets maps a target to the owner kinds that may use it. A combination
// outside this table is a generation error: the annotation could never be
// applied, and silently dropping it is the failure mode this contract exists to
// prevent. A table rather than a switch keeps the check exhaustive as new
// targets are added.
var legalTargets = map[apigw_v1.OpenAPICustomizationTarget]map[tfOwnerKind]bool{
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA: {
		tfOwnerField:   true,
		tfOwnerMessage: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER: {
		tfOwnerField: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS: {
		tfOwnerField: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES: {
		tfOwnerField: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_OPERATION: {
		tfOwnerOperation: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_REQUEST_SCHEMA: {
		tfOwnerOperation: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_RESPONSE_SCHEMA: {
		tfOwnerOperation: true,
	},
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_DOCUMENT_ROOT: {
		tfOwnerService: true,
		tfOwnerFile:    true,
	},
	// The escape hatch is legal everywhere, so a future placement never needs a
	// new target value or a new apigw release.
	apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER: {
		tfOwnerField:     true,
		tfOwnerMessage:   true,
		tfOwnerOperation: true,
		tfOwnerService:   true,
		tfOwnerFile:      true,
	},
}

// legalTarget reports whether an owner kind may address a target.
func (k tfOwnerKind) legalTarget(t apigw_v1.OpenAPICustomizationTarget) bool {
	return legalTargets[t][k]
}

// patchField is one keyword set by a schema patch.
type patchField struct {
	key   string
	value *yaml.Node
}

// tfDecl is one declared annotation, parsed and validated. A declaration is
// either an extension override (key + value) or a schema patch (a set of
// keyword assignments plus removals); a patch expands to one operation per
// keyword.
type tfDecl struct {
	id string
	// descriptor is the annotated descriptor's identity, e.g.
	// "field c1.api.app.v1.AppEntitlement.provisioner_policy". It travels with
	// every operation the declaration produces, so a consumer or linter can
	// report which annotation wrote a destination without re-deriving it.
	descriptor string
	// describe is the human-readable label used in diagnostics.
	describe    string
	ownerKind   tfOwnerKind
	target      apigw_v1.OpenAPICustomizationTarget
	scope       apigw_v1.OpenAPICustomizationScope
	mode        apigw_v1.OpenAPICustomizationMode
	key         string
	value       *yaml.Node
	patchSet    []patchField
	patchRemove []string
	jsonPointer string
	// entities restricts the declaration to the Terraform projection of each
	// named entity. Empty means every consumer of the destination.
	entities []*tfoverlay.EntityScope
	// selector is set for a document-wide selector declaration, which is
	// recorded in the overlay's selector list rather than at a pointer.
	selector *tfoverlay.SelectorOperation
	// property is set for a property patch.
	property *propertyPatch
	// applied records the first pointer the declaration was applied at, which
	// is only needed for the unconsumed-annotation diagnostic.
	applied bool
}

// opsFor expands a declaration into overlay operations at pointer. Every
// operation carries the declaration's descriptor identity as provenance.
func (d *tfDecl) opsFor(pointer string) []tfoverlay.Operation {
	base := d.baseOpsFor(pointer)
	if len(d.entities) == 0 {
		return base
	}
	scoped := make([]tfoverlay.Operation, 0, len(base)*len(d.entities))
	for _, entity := range d.entities {
		for _, op := range base {
			e := *entity
			op.Entity = &e
			scoped = append(scoped, op)
		}
	}
	return scoped
}

// propertyPatch adds or removes one property of a schema.
type propertyPatch struct {
	name   string
	schema *yaml.Node
	remove bool
}

func (d *tfDecl) baseOpsFor(pointer string) []tfoverlay.Operation {
	if d.property != nil {
		op := tfoverlay.Operation{
			Pointer:    pointer + "/properties",
			Channel:    tfoverlay.ChannelProperty,
			Key:        d.property.name,
			Mode:       tfoverlay.ModeSet,
			Value:      d.property.schema,
			Provenance: d.descriptor,
		}
		if d.property.remove {
			op.Mode, op.Value = tfoverlay.ModeRemove, nil
		}
		return []tfoverlay.Operation{op}
	}
	if d.patchSet != nil || d.patchRemove != nil {
		ops := make([]tfoverlay.Operation, 0, len(d.patchSet)+len(d.patchRemove))
		for _, f := range d.patchSet {
			ops = append(ops, tfoverlay.Operation{
				Pointer:    pointer,
				Channel:    tfoverlay.ChannelSchema,
				Key:        f.key,
				Mode:       tfoverlay.ModeSet,
				Value:      f.value,
				Provenance: d.descriptor,
			})
		}
		for _, key := range d.patchRemove {
			ops = append(ops, tfoverlay.Operation{
				Pointer:    pointer,
				Channel:    tfoverlay.ChannelSchema,
				Key:        key,
				Mode:       tfoverlay.ModeRemove,
				Provenance: d.descriptor,
			})
		}
		return ops
	}
	op := tfoverlay.Operation{
		Pointer:    pointer,
		Channel:    tfoverlay.ChannelExtension,
		Key:        d.key,
		Mode:       tfoverlay.ModeSet,
		Value:      d.value,
		Provenance: d.descriptor,
	}
	if d.mode == apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE {
		op.Mode = tfoverlay.ModeRemove
		op.Value = nil
	}
	return []tfoverlay.Operation{op}
}

// docOverlay accumulates the operations for one emitted document.
type docOverlay struct {
	// terraform is rendered as the companion artifact.
	terraform *tfoverlay.Overlay
	// shared is applied to the rendered document instead of being emitted.
	shared *tfoverlay.Overlay
}

func newDocOverlay(source string, kind tfoverlay.Kind) *docOverlay {
	return &docOverlay{
		terraform: &tfoverlay.Overlay{Source: source, Kind: kind},
		shared:    &tfoverlay.Overlay{Source: source, Kind: kind},
	}
}

// add records op, rejecting a contradiction either within one scope or across
// the two scopes. A Terraform-scoped and a shared-scoped assertion of the same
// key at the same destination are allowed only when they agree; otherwise the
// Terraform pipeline would silently disagree with the published document.
func (o *docOverlay) add(op tfoverlay.Operation, scope apigw_v1.OpenAPICustomizationScope) error {
	target, other := o.terraform, o.shared
	if scope == apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_SHARED {
		target, other = o.shared, o.terraform
	}
	if err := target.Add(op); err != nil {
		return err
	}
	for _, existing := range other.Operations {
		if !tfoverlay.Overlaps(existing, op) {
			continue
		}
		if !sameOperation(existing, op) {
			return fmt.Errorf(
				"conflicting scopes at %s for %s: a Terraform-scoped and a shared-scoped override disagree",
				op.Pointer, op.Key)
		}
	}
	return nil
}

func sameOperation(a, b tfoverlay.Operation) bool {
	if a.Mode != b.Mode {
		return false
	}
	if a.Value == nil || b.Value == nil {
		return a.Value == nil && b.Value == nil
	}
	ab, errA := yaml.Marshal(a.Value)
	bb, errB := yaml.Marshal(b.Value)
	return errA == nil && errB == nil && string(ab) == string(bb)
}

// tfCollector holds every declaration in one generated proto file plus the
// bookkeeping that proves each one was applied.
type tfCollector struct {
	decls  map[string]*tfDecl
	order  []string
	active *docOverlay
	// blockedReasons records why a declaration has not been applied anywhere,
	// so verify can explain the failure instead of just naming the annotation.
	blockedReasons map[string]string
}

func newTFCollector() *tfCollector {
	return &tfCollector{decls: map[string]*tfDecl{}}
}

func (c *tfCollector) beginDoc(source string, kind tfoverlay.Kind) {
	if c == nil {
		return
	}
	c.active = newDocOverlay(source, kind)
}

func (c *tfCollector) endDoc() *docOverlay {
	if c == nil {
		return nil
	}
	ov := c.active
	c.active = nil
	return ov
}

// declare validates and registers one annotation.
func (c *tfCollector) declare(d *tfDecl) {
	if _, exists := c.decls[d.id]; !exists {
		c.order = append(c.order, d.id)
	}
	c.decls[d.id] = d
}

func (c *tfCollector) failf(format string, args ...interface{}) {
	panic(tfError{fmt.Sprintf(format, args...)})
}

// tfError carries a customization diagnostic out of the emitter, which has no
// error return on the paths it is reached from.
type tfError struct{ msg string }

func (e tfError) Error() string { return e.msg }

// emit resolves decls against the destinations available at the emission site
// and records the resulting operations. Declarations whose destination is not
// available here are left for another site or reported by verify.
func (c *tfCollector) emit(decls []*tfDecl, places map[apigw_v1.OpenAPICustomizationTarget]string, unavailable map[apigw_v1.OpenAPICustomizationTarget]string) {
	if c == nil || c.active == nil {
		return
	}
	for _, d := range decls {
		registered, ok := c.decls[d.id]
		if !ok {
			continue
		}
		if registered.selector != nil {
			sel := *registered.selector
			if err := c.active.terraform.AddSelector(sel); err != nil {
				c.failf("%s: %v", registered.describe, err)
			}
			registered.applied = true
			continue
		}
		pointer, available := places[registered.target]
		if !available {
			if reason, known := unavailable[registered.target]; known {
				c.blockReason(registered.id, reason)
			}
			continue
		}
		if registered.target == apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER {
			pointer = registered.jsonPointer
		}
		if len(registered.entities) > 0 && !inComponentSchemas(pointer) {
			// The same schema can also be emitted as an HTTP parameter, which is
			// not an entity-addressable component. The component emission
			// carries the override; verify fails if there is none.
			c.blockReason(registered.id, "an entity-scoped annotation needs a destination under /components/schemas, and this document emits the field only as a parameter")
			continue
		}
		for _, op := range registered.opsFor(pointer) {
			if err := c.active.add(op, registered.scope); err != nil {
				c.failf("%s: %v", registered.describe, err)
			}
		}
		registered.applied = true
	}
}

// parseSelector validates a document-wide selector declaration. A selector is
// Terraform-only; the consumer resolves it against the shared spec.
func (c *tfCollector) parseSelector(descriptor string, k tfOwnerKind, base string, index int, sc *apigw_v1.OpenAPISelectorCustomization) *tfDecl {
	id := declID(base, descriptor, index)
	d := &tfDecl{
		id:         id,
		descriptor: descriptor,
		describe:   descriptor + " selector customization",
		ownerKind:  k,
		scope:      sc.GetScope(),
		target:     apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_DOCUMENT_ROOT,
	}
	if d.scope == apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_SHARED {
		c.failf("%s: a selector is Terraform-only and cannot use the shared scope", d.describe)
	}
	sel := tfoverlay.SelectorOperation{Provenance: descriptor, Key: sc.GetKey()}
	sel.Selector.Path = sc.GetSelector().GetJsonPath()
	switch sc.GetMode() {
	case apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_UNSPECIFIED,
		apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_SET:
		sel.Mode = tfoverlay.ModeSet
		if sc.GetValueJson() == "" {
			c.failf("%s: value_json is required when mode is SET", d.describe)
		}
		value, err := tfoverlay.ParseJSONValue(sc.GetValueJson())
		if err != nil {
			c.failf("%s: %v", d.describe, err)
		}
		sel.Value = value
		sel.Channel = tfoverlay.ChannelSchema
		if strings.HasPrefix(sel.Key, "x-") {
			sel.Channel = tfoverlay.ChannelExtension
		}
	case apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE:
		sel.Mode = tfoverlay.ModeRemove
		if sc.GetValueJson() != "" || sc.GetKey() != "" {
			c.failf("%s: key and value_json must be empty when mode is REMOVE", d.describe)
		}
	default:
		c.failf("%s: unknown mode %v", d.describe, sc.GetMode())
	}
	if err := sel.Validate(); err != nil {
		c.failf("%s: %v", d.describe, err)
	}
	d.selector = &sel
	return d
}

// parsePropertyPatch validates a property patch on a message.
func (c *tfCollector) parsePropertyPatch(descriptor string, k tfOwnerKind, base string, index int, pp *apigw_v1.OpenAPIPropertyPatch) *tfDecl {
	id := declID(base, descriptor, index)
	d := &tfDecl{
		id:          id,
		descriptor:  descriptor,
		describe:    fmt.Sprintf("%s property patch %q", descriptor, pp.GetName()),
		ownerKind:   k,
		scope:       pp.GetScope(),
		target:      pp.GetTarget(),
		jsonPointer: pp.GetJsonPointer(),
	}
	if d.scope == apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_SHARED {
		c.failf("%s: a property patch is Terraform-only and cannot use the shared scope", d.describe)
	}
	if d.target == apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_UNSPECIFIED {
		d.target = apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA
	}
	switch d.target {
	case apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA:
		if d.jsonPointer != "" {
			c.failf("%s: json_pointer is only valid with target JSON_POINTER", d.describe)
		}
	case apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER:
		if !inComponentSchemas(d.jsonPointer) {
			c.failf("%s: json_pointer must address a schema under /components/schemas/<name>", d.describe)
		}
	default:
		c.failf("%s: target %s is not valid for a property patch", d.describe, d.target)
	}
	if pp.GetName() == "" {
		c.failf("%s: a property patch needs a name", d.describe)
	}
	patch := &propertyPatch{name: pp.GetName()}
	switch pp.GetMode() {
	case apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_UNSPECIFIED,
		apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_SET:
		if pp.GetSchemaJson() == "" {
			c.failf("%s: schema_json is required when mode is SET", d.describe)
		}
		value, err := tfoverlay.ParseJSONValue(pp.GetSchemaJson())
		if err != nil {
			c.failf("%s: schema_json: %v", d.describe, err)
		}
		if value.Kind != yaml.MappingNode {
			c.failf("%s: schema_json must be a JSON object", d.describe)
		}
		patch.schema = value
	case apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE:
		if pp.GetSchemaJson() != "" {
			c.failf("%s: schema_json must be empty when mode is REMOVE", d.describe)
		}
		patch.remove = true
	default:
		c.failf("%s: unknown mode %v", d.describe, pp.GetMode())
	}
	d.property = patch
	d.entities = c.parseEntities(d, pp.GetEntities())
	return d
}

// inComponentSchemas reports whether pointer addresses a node inside a
// component schema, the only place an entity-scoped override can land.
func inComponentSchemas(pointer string) bool {
	tokens, err := tfoverlay.ParsePointer(pointer)
	return err == nil && len(tokens) >= 3 && tokens[0] == "components" && tokens[1] == "schemas"
}

// blocked records why a declaration has not been applied anywhere yet.
func (c *tfCollector) blockReason(id, reason string) {
	if c.blockedReasons == nil {
		c.blockedReasons = map[string]string{}
	}
	if _, exists := c.blockedReasons[id]; !exists {
		c.blockedReasons[id] = reason
	}
}

// verify fails generation when a declared annotation was never applied. An
// annotation that vanishes silently is worse than one that fails the build: the
// author believes Terraform behavior was customized when it was not.
func (c *tfCollector) verify() error {
	if len(c.decls) == 0 {
		return nil
	}
	unapplied := make([]string, 0)
	for _, id := range c.order {
		d := c.decls[id]
		if d.applied {
			continue
		}
		reason := c.blockedReasons[id]
		if reason == "" {
			reason = "no emitted document contains the annotated entity"
		}
		unapplied = append(unapplied, fmt.Sprintf("%s (%s)", d.describe, reason))
	}
	if len(unapplied) == 0 {
		return nil
	}
	sort.Strings(unapplied)
	return fmt.Errorf("%d annotation(s) could not be applied:\n  - %s", len(unapplied), strings.Join(unapplied, "\n  - "))
}

// ---------------------------------------------------------------------------
// declaration extraction
// ---------------------------------------------------------------------------

func declID(prefix, owner string, index int) string {
	return fmt.Sprintf("%s:%s#%d", prefix, owner, index)
}

func (c *tfCollector) parseCustomization(descriptor string, k tfOwnerKind, base string, index int, cust *apigw_v1.OpenAPICustomization) *tfDecl {
	id := declID(base, descriptor, index)
	d := &tfDecl{
		id:          id,
		descriptor:  descriptor,
		describe:    fmt.Sprintf("%s annotation %q", descriptor, cust.GetKey()),
		ownerKind:   k,
		scope:       cust.GetScope(),
		mode:        cust.GetMode(),
		key:         cust.GetKey(),
		target:      cust.GetTarget(),
		jsonPointer: cust.GetJsonPointer(),
	}
	if d.target == apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_UNSPECIFIED {
		d.target = k.defaultValue()
	}
	if !k.legalTarget(d.target) {
		c.failf("%s: target %s is not valid on a %s", d.describe, d.target, k)
	}
	if d.jsonPointer != "" && d.target != apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER {
		c.failf("%s: json_pointer is only valid with target JSON_POINTER", d.describe)
	}
	if !strings.HasPrefix(d.key, "x-") {
		c.failf("%s: extension key %q must start with \"x-\"", d.describe, d.key)
	}
	switch d.mode {
	case apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_UNSPECIFIED,
		apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_SET:
		if cust.GetValueJson() == "" {
			c.failf("%s: value_json is required when mode is SET", d.describe)
		}
		value, err := tfoverlay.ParseJSONValue(cust.GetValueJson())
		if err != nil {
			c.failf("%s: %v", d.describe, err)
		}
		d.value = value
	case apigw_v1.OpenAPICustomizationMode_OPEN_API_CUSTOMIZATION_MODE_REMOVE:
		if cust.GetValueJson() != "" {
			c.failf("%s: value_json must be empty when mode is REMOVE", d.describe)
		}
	default:
		c.failf("%s: unknown mode %v", d.describe, d.mode)
	}
	if _, err := tfoverlay.ParsePointer(d.jsonPointer); err != nil {
		c.failf("%s: %v", d.describe, err)
	}
	d.entities = c.parseEntities(d, cust.GetEntities())
	return d
}

// parseEntities validates the entity selector of a declaration. An entity-scoped
// override is Terraform-only and must land in a component schema, because the
// projection copies component schemas; anything else has no per-entity copy to
// land in. apigw does not check that the entity exists: the emitted document may
// not contain the entity's operations, so the consumer resolves it.
func (c *tfCollector) parseEntities(d *tfDecl, entities []*apigw_v1.OpenAPICustomizationEntity) []*tfoverlay.EntityScope {
	if len(entities) == 0 {
		return nil
	}
	if d.scope == apigw_v1.OpenAPICustomizationScope_OPEN_API_CUSTOMIZATION_SCOPE_SHARED {
		c.failf("%s: entities requires the Terraform scope; the shared document has no entities", d.describe)
	}
	switch d.target {
	case apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA,
		apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS,
		apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES:
	case apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER:
		if !inComponentSchemas(d.jsonPointer) {
			c.failf("%s: an entity-scoped json_pointer must address a node under /components/schemas/<name>", d.describe)
		}
	default:
		c.failf("%s: target %s cannot be entity-scoped; only component schema destinations can", d.describe, d.target)
	}
	scopes := make([]*tfoverlay.EntityScope, 0, len(entities))
	seen := map[tfoverlay.EntityScope]bool{}
	for _, entity := range entities {
		if entity.GetName() == "" {
			c.failf("%s: an entity selector needs a name", d.describe)
		}
		var kind tfoverlay.EntityKind
		switch entity.GetKind() {
		case apigw_v1.OpenAPICustomizationEntityKind_OPEN_API_CUSTOMIZATION_ENTITY_KIND_UNSPECIFIED:
			kind = tfoverlay.EntityKindAny
		case apigw_v1.OpenAPICustomizationEntityKind_OPEN_API_CUSTOMIZATION_ENTITY_KIND_RESOURCE:
			kind = tfoverlay.EntityKindResource
		case apigw_v1.OpenAPICustomizationEntityKind_OPEN_API_CUSTOMIZATION_ENTITY_KIND_DATA_SOURCE:
			kind = tfoverlay.EntityKindDataSource
		default:
			c.failf("%s: entity %q has unknown kind %v", d.describe, entity.GetName(), entity.GetKind())
		}
		scope := tfoverlay.EntityScope{Name: entity.GetName(), Kind: kind}
		if seen[scope] {
			c.failf("%s: duplicate entity selector %s", d.describe, scope.String())
		}
		seen[scope] = true
		scopes = append(scopes, &scope)
	}
	return scopes
}

func (c *tfCollector) parsePatch(descriptor string, k tfOwnerKind, base string, index int, patch *apigw_v1.OpenAPISchemaPatch) *tfDecl {
	id := declID(base, descriptor, index)
	d := &tfDecl{
		id:          id,
		descriptor:  descriptor,
		describe:    descriptor + " schema patch",
		ownerKind:   k,
		scope:       patch.GetScope(),
		target:      patch.GetTarget(),
		jsonPointer: patch.GetJsonPointer(),
	}
	if d.target == apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_UNSPECIFIED {
		d.target = k.defaultValue()
	}
	if !k.legalTarget(d.target) {
		c.failf("%s: target %s is not valid on a %s", d.describe, d.target, k)
	}
	// A schema patch replaces a schema object; the operation target is the
	// schema's object, not an operation or a parameter.
	if d.target == apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER {
		c.failf("%s: target PARAMETER is not valid for a schema patch", d.describe)
	}
	if d.jsonPointer != "" && d.target != apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER {
		c.failf("%s: json_pointer is only valid with target JSON_POINTER", d.describe)
	}
	if _, err := tfoverlay.ParsePointer(d.jsonPointer); err != nil {
		c.failf("%s: %v", d.describe, err)
	}
	if patch.GetPatchJson() != "" {
		value, err := tfoverlay.ParseJSONValue(patch.GetPatchJson())
		if err != nil {
			c.failf("%s: %v", d.describe, err)
		}
		if value.Kind != yaml.MappingNode {
			c.failf("%s: patch_json must be a JSON object", d.describe)
		}
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i].Value
			if key == "" {
				c.failf("%s: patch_json contains an empty keyword", d.describe)
			}
			if strings.HasPrefix(key, "x-") {
				c.failf("%s: patch_json keyword %q is a vendor extension; use customizations instead", d.describe, key)
			}
			d.patchSet = append(d.patchSet, patchField{key: key, value: value.Content[i+1]})
		}
	}
	for _, key := range patch.GetRemoveKeys() {
		if key == "" {
			c.failf("%s: remove_keys contains an empty keyword", d.describe)
		}
		if strings.HasPrefix(key, "x-") {
			c.failf("%s: remove_keys keyword %q is a vendor extension; use customizations with mode REMOVE instead", d.describe, key)
		}
		d.patchRemove = append(d.patchRemove, key)
	}
	if len(d.patchSet) == 0 && len(d.patchRemove) == 0 {
		c.failf("%s: a schema patch needs patch_json or remove_keys", d.describe)
	}
	d.entities = c.parseEntities(d, patch.GetEntities())
	return d
}

// declareField registers the annotations on one field.
func (c *tfCollector) declareField(f pgs.Field) {
	owner := "field " + nicerFQN(f)
	index := 0
	for _, opt := range getFieldOptionList(f) {
		for _, cust := range opt.GetCustomizations() {
			c.declare(c.parseCustomization(owner, tfOwnerField, "field", index, cust))
			index++
		}
		for _, patch := range opt.GetSchemaPatches() {
			c.declare(c.parsePatch(owner, tfOwnerField, "fieldp", index, patch))
			index++
		}
	}
}

// declareMessage registers the annotations on one message.
func (c *tfCollector) declareMessage(m pgs.Message) {
	owner := "message " + nicerFQN(m)
	index := 0
	for _, opt := range getMessageOptionList(m) {
		for _, cust := range opt.GetCustomizations() {
			c.declare(c.parseCustomization(owner, tfOwnerMessage, "message", index, cust))
			index++
		}
		for _, patch := range opt.GetSchemaPatches() {
			c.declare(c.parsePatch(owner, tfOwnerMessage, "messagep", index, patch))
			index++
		}
		for _, pp := range opt.GetPropertyPatches() {
			c.declare(c.parsePropertyPatch(owner, tfOwnerMessage, "messagepp", index, pp))
			index++
		}
	}
}

// declareOperation registers the annotations on one HTTP operation of a method.
func (c *tfCollector) declareOperation(method pgs.Method, operation *apigw_v1.Operation, opIndex int) {
	owner := "operation " + nicerFQN(method)
	if opIndex > 0 {
		if len(operation.GetCustomizations()) > 0 || len(operation.GetSchemaPatches()) > 0 {
			c.failf("%s: operations[%d] carries annotations but apigw emits only operations[0]", owner, opIndex)
		}
		return
	}
	index := 0
	for _, cust := range operation.GetCustomizations() {
		c.declare(c.parseCustomization(owner, tfOwnerOperation, "operation", index, cust))
		index++
	}
	for _, patch := range operation.GetSchemaPatches() {
		c.declare(c.parsePatch(owner, tfOwnerOperation, "operationp", index, patch))
		index++
	}
}

// declareService registers the annotations on a service.
func (c *tfCollector) declareService(svc pgs.Service, s *apigw_v1.Service) {
	if s == nil {
		return
	}
	owner := "service " + nicerFQN(svc)
	index := 0
	for _, cust := range s.GetCustomizations() {
		c.declare(c.parseCustomization(owner, tfOwnerService, "service", index, cust))
		index++
	}
	for _, patch := range s.GetSchemaPatches() {
		c.declare(c.parsePatch(owner, tfOwnerService, "servicep", index, patch))
		index++
	}
	for _, sc := range s.GetSelectorCustomizations() {
		c.declare(c.parseSelector(owner, tfOwnerService, "services", index, sc))
		index++
	}
}

// declareFile registers the file-level annotations, which apply to every
// document generated from the file.
func (c *tfCollector) declareFile(f pgs.File) {
	opts := &apigw_v1.FileOptions{}
	if _, err := f.Extension(apigw_v1.E_File, opts); err != nil {
		return
	}
	owner := "file " + nicerFQN(f)
	index := 0
	for _, cust := range opts.GetCustomizations() {
		c.declare(c.parseCustomization(owner, tfOwnerFile, "file", index, cust))
		index++
	}
	for _, patch := range opts.GetSchemaPatches() {
		c.declare(c.parsePatch(owner, tfOwnerFile, "filep", index, patch))
		index++
	}
	for _, sc := range opts.GetSelectorCustomizations() {
		c.declare(c.parseSelector(owner, tfOwnerFile, "files", index, sc))
		index++
	}
}

// declareFileEntities walks every entity the generator can emit from f and
// registers its annotations, so that verify can prove each one was applied.
func (c *tfCollector) declareFileEntities(f pgs.File) {
	c.declareFile(f)
	for _, m := range f.Messages() {
		c.declareMessageTree(m)
	}
	for _, svc := range f.Services() {
		sext := &apigw_v1.ServiceOptions{}
		if _, err := svc.Extension(apigw_v1.E_Service, sext); err == nil {
			c.declareService(svc, sext.GetService())
		}
		for _, method := range svc.Methods() {
			mext := &apigw_v1.MethodOptions{}
			if _, err := method.Extension(apigw_v1.E_Method, mext); err != nil {
				continue
			}
			for i, op := range mext.GetOperations() {
				c.declareOperation(method, op, i)
			}
		}
	}
}

func (c *tfCollector) declareMessageTree(m pgs.Message) {
	c.declareMessage(m)
	for _, f := range m.Fields() {
		c.declareField(f)
	}
	for _, nested := range m.Messages() {
		c.declareMessageTree(nested)
	}
}

// fieldDeclsFor returns the registered declarations for one field, in the same
// order declareField used. The emitter calls this to find what applies to the
// node it is about to emit.
func fieldDeclsFor(c *tfCollector, f pgs.Field) []*tfDecl {
	if c == nil {
		return nil
	}
	owner := "field " + nicerFQN(f)
	index := 0
	rv := make([]*tfDecl, 0)
	for _, opt := range getFieldOptionList(f) {
		for range opt.GetCustomizations() {
			if d, ok := c.decls[declID("field", owner, index)]; ok {
				rv = append(rv, d)
			}
			index++
		}
		for range opt.GetSchemaPatches() {
			if d, ok := c.decls[declID("fieldp", owner, index)]; ok {
				rv = append(rv, d)
			}
			index++
		}
	}
	return rv
}

func messageDeclsFor(c *tfCollector, m pgs.Message) []*tfDecl {
	if c == nil {
		return nil
	}
	owner := "message " + nicerFQN(m)
	index := 0
	rv := make([]*tfDecl, 0)
	for _, opt := range getMessageOptionList(m) {
		for range opt.GetCustomizations() {
			if d, ok := c.decls[declID("message", owner, index)]; ok {
				rv = append(rv, d)
			}
			index++
		}
		for range opt.GetSchemaPatches() {
			if d, ok := c.decls[declID("messagep", owner, index)]; ok {
				rv = append(rv, d)
			}
			index++
		}
		for range opt.GetPropertyPatches() {
			if d, ok := c.decls[declID("messagepp", owner, index)]; ok {
				rv = append(rv, d)
			}
			index++
		}
	}
	return rv
}

func operationDeclsFor(c *tfCollector, method pgs.Method, operation *apigw_v1.Operation) []*tfDecl {
	if c == nil {
		return nil
	}
	owner := "operation " + nicerFQN(method)
	index := 0
	rv := make([]*tfDecl, 0)
	for range operation.GetCustomizations() {
		if d, ok := c.decls[declID("operation", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	for range operation.GetSchemaPatches() {
		if d, ok := c.decls[declID("operationp", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	return rv
}

func serviceDeclsFor(c *tfCollector, svc pgs.Service, s *apigw_v1.Service) []*tfDecl {
	if c == nil || s == nil {
		return nil
	}
	owner := "service " + nicerFQN(svc)
	index := 0
	rv := make([]*tfDecl, 0)
	for range s.GetCustomizations() {
		if d, ok := c.decls[declID("service", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	for range s.GetSchemaPatches() {
		if d, ok := c.decls[declID("servicep", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	for range s.GetSelectorCustomizations() {
		if d, ok := c.decls[declID("services", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	return rv
}

func fileDeclsFor(c *tfCollector, f pgs.File) []*tfDecl {
	if c == nil {
		return nil
	}
	owner := "file " + nicerFQN(f)
	index := 0
	rv := make([]*tfDecl, 0)
	opts := &apigw_v1.FileOptions{}
	if _, err := f.Extension(apigw_v1.E_File, opts); err != nil {
		return nil
	}
	for range opts.GetCustomizations() {
		if d, ok := c.decls[declID("file", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	for range opts.GetSchemaPatches() {
		if d, ok := c.decls[declID("filep", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	for range opts.GetSelectorCustomizations() {
		if d, ok := c.decls[declID("files", owner, index)]; ok {
			rv = append(rv, d)
		}
		index++
	}
	return rv
}

// getMessageOptionList returns every MessageOption on a message. The typed
// options read only the first entry; generic annotations honor all of them.
func getMessageOptionList(m pgs.Message) []*apigw_v1.MessageOption {
	mopt := &apigw_v1.MessageOptions{}
	_, err := m.Extension(apigw_v1.E_Message, mopt)
	if err != nil {
		return nil
	}
	return mopt.GetMessageOptions()
}

func getFieldOptionList(f pgs.Field) []*apigw_v1.FieldOption {
	return getFieldOptions(f)
}

// schemaLoc describes where a field's schema is being emitted so that
// field-level annotations resolve to absolute pointers. The zero value means
// the site does not participate in annotation collection.
type schemaLoc struct {
	ok     bool
	schema string
	param  string
}

// fieldPropertyLoc is the location of a field emitted as a property of the
// component named component.
func fieldPropertyLoc(component, jsonName string) schemaLoc {
	return schemaLoc{
		ok:     true,
		schema: tfoverlay.JoinPointer("components", "schemas", component, "properties", jsonName),
	}
}

// fieldParamLoc is the location of a field emitted as an HTTP parameter: the
// parameter object, and the schema inside it.
func fieldParamLoc(paramPointer string) schemaLoc {
	return schemaLoc{ok: true, schema: paramPointer + "/schema", param: paramPointer}
}

// fieldSchemaPlaces returns the destinations available at a field emission
// site, and the reason each unavailable one is unavailable.
func fieldSchemaPlaces(f pgs.Field, loc schemaLoc) (map[apigw_v1.OpenAPICustomizationTarget]string, map[apigw_v1.OpenAPICustomizationTarget]string) {
	places := map[apigw_v1.OpenAPICustomizationTarget]string{
		apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_SCHEMA: loc.schema,
	}
	unavailable := map[apigw_v1.OpenAPICustomizationTarget]string{}
	if f.Type().IsRepeated() {
		places[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS] = loc.schema + "/items"
	} else {
		unavailable[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS] = "field is not repeated"
	}
	if f.Type().IsMap() {
		places[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES] = loc.schema + "/additionalProperties"
	} else {
		unavailable[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES] = "field is not a map"
	}
	if loc.param != "" {
		places[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER] = loc.param
	} else {
		unavailable[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_PARAMETER] = "field is not emitted as a path or query parameter in any document"
	}
	places[apigw_v1.OpenAPICustomizationTarget_OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER] = ""
	return places, unavailable
}

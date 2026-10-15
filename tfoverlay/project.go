package tfoverlay

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// This file implements ApplyTerraform, the consumer half of entity-scoped
// operations.
//
// A customization addressed at a schema component lands in every entity that
// reaches the component through $ref. To apply it to one entity only, that
// entity needs its own copy of the schema nodes between its operations and the
// target. ApplyTerraform builds the minimal such projection on the final
// Terraform input document: only components on a path from the entity's
// operations to the target are copied, and only when something outside the
// entity also refers to them. Everything below the target stays shared.

const schemaRefPrefix = "#/components/schemas/"

// entityExtension is the vendor extension binding a schema to Terraform
// entities.
const entityExtension = "x-speakeasy-entity"

const (
	entityOperationExtension = "x-speakeasy-entity-operation"
	nameOverrideExtension    = "x-speakeasy-name-override"
	terraformResourceKey     = "terraform-resource"
	terraformDatasourceKey   = "terraform-datasource"
)

var operationMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// ProjectionReport describes what ApplyTerraform changed, so CI and linters can
// compare a projection against expectations without diffing documents.
type ProjectionReport struct {
	// Entities has one entry per projected entity, in name order.
	Entities []EntityProjection
	// Selectors has one entry per selector operation, in canonical order. It is
	// filled by ApplyOverlay.
	Selectors []SelectorReport
}

// EntityProjection is the projection of one entity.
type EntityProjection struct {
	// Name and Kind are the entity selector the projection was built for.
	Name string
	Kind EntityKind
	// Operations are the entity's operations, as "METHOD /path".
	Operations []string
	// Clones are the components copied for the entity.
	Clones []ComponentClone
	// InPlace are the components only the entity reaches. They are edited in
	// place, with their $refs pointed at the clones.
	InPlace []string
	// Rebinding is set when the entity's root schema was copied: the entity name
	// moves from the original onto the copy.
	Rebinding *Rebinding
	// Pointers are the scoped operations' destinations, as written and as
	// applied.
	Pointers []PointerRewrite
}

// ComponentClone records one copied component schema.
type ComponentClone struct {
	From string
	To   string
	// NameOverride is the x-speakeasy-name-override given to the copy.
	NameOverride string
}

// Rebinding records the move of x-speakeasy-entity.
type Rebinding struct {
	Name string
	From string
	To   string
}

// PointerRewrite maps a scoped operation's destination to where it applied.
type PointerRewrite struct {
	From string
	To   string
}

// ApplyTerraform applies ops, including entity-scoped ones, to the final
// Terraform input document and returns the result with a report of the
// projections it performed.
//
//  1. Global operations are applied first, so every copy inherits them.
//  2. Each entity's scoped operations are then applied to that entity's
//     projection only.
//
// The result is a pure function of doc and the canonical operation set: groups
// are processed in name order and components in key order, so permuting ops
// cannot change the output.
//
// ApplyTerraform fails, rather than ignoring a scoped operation, when the
// entity or its root schema does not resolve, when the entity has no
// operations of the selected kind, when the destination is not reachable from
// the entity's operations, and when a resource and a data source that share
// operations receive differing scoped operations. The projection changes the
// generated types of the operations it re-points; callers that share those
// operations outside the entity must account for that.
func ApplyTerraform(doc []byte, ops []Operation) ([]byte, *ProjectionReport, error) {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return nil, nil, fmt.Errorf("parsing document: %w", err)
	}
	report, err := ApplyTerraformToNode(root, ops)
	if err != nil {
		return nil, nil, err
	}
	out, err := encode(root)
	if err != nil {
		return nil, nil, err
	}
	return out, report, nil
}

// ApplyTerraformToNode is ApplyTerraform on a parsed document, modified in
// place.
func ApplyTerraformToNode(root *yaml.Node, ops []Operation) (*ProjectionReport, error) {
	check := &Overlay{}
	for i := range ops {
		if err := check.Add(ops[i]); err != nil {
			return nil, fmt.Errorf("inconsistent operation list: %w", err)
		}
	}
	check.Normalize()

	var global []Operation
	scoped := map[string][]Operation{}
	for _, op := range check.Operations {
		if op.Entity == nil {
			global = append(global, op)
			continue
		}
		scoped[op.Entity.Name] = append(scoped[op.Entity.Name], op)
	}
	if err := ApplyToNode(root, global); err != nil {
		return nil, err
	}

	report := &ProjectionReport{}
	names := make([]string, 0, len(scoped))
	for name := range scoped {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		kind, groupOps, err := resolveSelector(name, scoped[name])
		if err != nil {
			return nil, err
		}
		projection, err := projectEntity(documentRoot(root), name, kind, groupOps)
		if err != nil {
			return nil, err
		}
		report.Entities = append(report.Entities, *projection)
	}
	return report, nil
}

// ValidateTerraform is Validate for a list that may hold entity-scoped
// operations. Global operations get the full Validate checks. A scoped
// operation is only checked for a destination that resolves to an object in
// doc: whether it is reachable from its entity, and whether a key it removes
// exists after the global operations, is decided by ApplyTerraform.
func ValidateTerraform(doc []byte, ops []Operation) error {
	var global, scoped []Operation
	check := &Overlay{}
	for i := range ops {
		if err := check.Add(ops[i]); err != nil {
			return fmt.Errorf("inconsistent operation list: %w", err)
		}
		if ops[i].Entity == nil {
			global = append(global, ops[i])
		} else {
			scoped = append(scoped, ops[i])
		}
	}
	if err := Validate(doc, global); err != nil {
		return err
	}
	if len(scoped) == 0 {
		return nil
	}
	root := &yaml.Node{}
	if err := yaml.Unmarshal(doc, root); err != nil {
		return fmt.Errorf("parsing document: %w", err)
	}
	target := documentRoot(root)
	if target == nil {
		return fmt.Errorf("document has no root node")
	}
	for _, op := range scoped {
		node, err := resolvePointer(target, op.Pointer)
		if err != nil {
			return newError(op, ownerLabel(op), "%s", err)
		}
		if node.Kind != yaml.MappingNode {
			return newError(op, ownerLabel(op),
				"cannot write %s at %s: destination is not an object (%s)",
				op.Key, pointerForDisplay(op.Pointer), nodeKind(node))
		}
	}
	return nil
}

// resolveSelector reduces the selector kinds used for one entity name to one.
// Selectors whose operations agree are the same selector; selectors that
// differ cannot be applied separately, because the resource and the data
// source would need distinct copies of the same entity.
func resolveSelector(name string, ops []Operation) (EntityKind, []Operation, error) {
	bySelector := map[EntityKind][]Operation{}
	for _, op := range ops {
		bySelector[op.Entity.Kind] = append(bySelector[op.Entity.Kind], op)
	}
	if len(bySelector) == 1 {
		for kind, kindOps := range bySelector {
			return kind, kindOps, nil
		}
	}
	signatures := map[string]bool{}
	var kinds []string
	for kind, kindOps := range bySelector {
		kinds = append(kinds, kindLabel(kind))
		signatures[operationSignature(kindOps)] = true
	}
	sort.Strings(kinds)
	if len(signatures) > 1 {
		return "", nil, fmt.Errorf(
			"entity %q receives differing scoped operations for %s; a resource and a data source that share operations cannot be customized differently",
			name, strings.Join(kinds, " and "))
	}
	// Agreeing selectors: apply the operations once, to the whole entity.
	merged := make([]Operation, 0, len(ops))
	seen := map[string]bool{}
	for _, op := range ops {
		key := opOriginKey(Operation{Pointer: op.Pointer, Channel: op.Channel, Key: op.Key})
		if seen[key] {
			continue
		}
		seen[key] = true
		op.Entity = &EntityScope{Name: name}
		merged = append(merged, op)
	}
	return EntityKindAny, merged, nil
}

func kindLabel(kind EntityKind) string {
	if kind == EntityKindAny {
		return "the resource and data source"
	}
	return string(kind)
}

func operationSignature(ops []Operation) string {
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		parts = append(parts, strings.Join([]string{
			op.Pointer, string(op.Channel), op.Key, string(op.Mode), displayValue(op.Value),
		}, "\x00"))
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x01")
}

// ---------------------------------------------------------------------------
// projection
// ---------------------------------------------------------------------------

type entityOperation struct {
	label string // "METHOD /path"
	node  *yaml.Node
	// resource and datasource report which Terraform constructs of the entity the
	// operation serves.
	resource   bool
	datasource bool
}

func projectEntity(doc *yaml.Node, name string, kind EntityKind, ops []Operation) (*EntityProjection, error) {
	describe := fmt.Sprintf("entity %q", name)
	if kind != EntityKindAny {
		describe = fmt.Sprintf("entity %q (%s)", name, kind)
	}
	fail := func(format string, args ...interface{}) error {
		return fmt.Errorf("%s: %s", describe, fmt.Sprintf(format, args...))
	}
	failOp := func(op Operation, format string, args ...interface{}) error {
		return newError(op, ownerLabel(op), "%s: %s", describe, fmt.Sprintf(format, args...))
	}

	schemas := mappingChild(mappingChild(doc, "components"), "schemas")
	if schemas == nil {
		return nil, fail("document has no components/schemas")
	}

	entityOps, err := entityOperations(doc, name)
	if err != nil {
		return nil, fail("%v", err)
	}
	selected := map[string]bool{}
	var selectedOps []*entityOperation
	for _, eo := range entityOps {
		serves := false
		switch kind {
		case EntityKindResource:
			serves = eo.resource
		case EntityKindDataSource:
			serves = eo.datasource
		default:
			serves = eo.resource || eo.datasource
		}
		if !serves {
			continue
		}
		if kind == EntityKindResource && eo.datasource || kind == EntityKindDataSource && eo.resource {
			return nil, fail("cannot split resource and data source that share operation %s; "+
				"give it an explicit terraform-resource or terraform-datasource of null, or scope the override to the whole entity", eo.label)
		}
		selected[eo.label] = true
		selectedOps = append(selectedOps, eo)
	}
	if len(selectedOps) == 0 {
		return nil, fail("no operation of the entity is bound by %s", entityOperationExtension)
	}

	// The entity's root schema is the one component bound to the entity name.
	var roots []string
	for i := 0; i+1 < len(schemas.Content); i += 2 {
		if entityNames(mappingChild(schemas.Content[i+1], entityExtension))[name] {
			roots = append(roots, schemas.Content[i].Value)
		}
	}
	if len(roots) != 1 {
		return nil, fail("expected exactly one schema with %s %q, found %d", entityExtension, name, len(roots))
	}
	entityRoot := roots[0]

	// Component graph.
	graph := map[string]map[string]bool{}
	reverse := map[string]map[string]bool{}
	var keys []string
	for i := 0; i+1 < len(schemas.Content); i += 2 {
		key := schemas.Content[i].Value
		keys = append(keys, key)
		graph[key] = map[string]bool{}
	}
	sort.Strings(keys)
	for i := 0; i+1 < len(schemas.Content); i += 2 {
		from := schemas.Content[i].Value
		walkRefs(schemas.Content[i+1], func(ref *yaml.Node) {
			if to, _, ok := splitSchemaRef(ref.Value); ok {
				graph[from][to] = true
				if reverse[to] == nil {
					reverse[to] = map[string]bool{}
				}
				reverse[to][from] = true
			}
		})
	}

	// Entity operation schemas: request bodies and successful responses.
	var opSchemas []*yaml.Node
	external := map[string]bool{}
	reach := map[string]bool{}
	var queue []string
	for _, eo := range selectedOps {
		nodes, err := operationSchemaNodes(eo)
		if err != nil {
			return nil, fail("%v", err)
		}
		opSchemas = append(opSchemas, nodes...)
		for _, n := range nodes {
			walkRefs(n, func(ref *yaml.Node) {
				if to, _, ok := splitSchemaRef(ref.Value); ok && !reach[to] {
					reach[to] = true
					queue = append(queue, to)
				}
			})
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for next := range graph[cur] {
			if !reach[next] {
				reach[next] = true
				queue = append(queue, next)
			}
		}
	}

	// Destination components.
	targets := map[string]bool{}
	for _, op := range ops {
		tokens, err := ParsePointer(op.Pointer)
		if err != nil {
			return nil, failOp(op, "%v", err)
		}
		if len(tokens) < 3 || tokens[0] != "components" || tokens[1] != "schemas" {
			return nil, failOp(op, "destination %s is not inside a component schema; an entity-scoped operation must address /components/schemas/<name>",
				pointerForDisplay(op.Pointer))
		}
		if _, ok := graph[tokens[2]]; !ok {
			return nil, failOp(op, "destination %s does not resolve: no component schema %q", pointerForDisplay(op.Pointer), tokens[2])
		}
		if !reach[tokens[2]] {
			return nil, failOp(op, "destination %s is not reachable from the entity's operations, so the operation would have no effect",
				pointerForDisplay(op.Pointer))
		}
		targets[tokens[2]] = true
	}

	// Spine: components on a path from the entity's operations to a destination.
	spine := map[string]bool{}
	for target := range targets {
		back := map[string]bool{target: true}
		queue = []string{target}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for prev := range reverse[cur] {
				if !back[prev] {
					back[prev] = true
					queue = append(queue, prev)
				}
			}
		}
		for key := range back {
			if reach[key] {
				spine[key] = true
			}
		}
	}

	// References from outside the schemas, other than the entity's own request
	// and response schemas, keep a component shared.
	collectExternalRefs(doc, selected, func(ref *yaml.Node) {
		if to, _, ok := splitSchemaRef(ref.Value); ok {
			external[to] = true
		}
	})

	// A spine component is edited in place only when every referrer is the
	// entity's operations or another in-place spine component: the greatest such
	// set. Everything else on the spine is copied; because membership is a
	// fixpoint, a cycle with one externally referenced member is copied whole.
	inPlace := map[string]bool{}
	for key := range spine {
		if !external[key] {
			inPlace[key] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for key := range inPlace {
			for referrer := range reverse[key] {
				if !inPlace[referrer] {
					delete(inPlace, key)
					changed = true
					break
				}
			}
		}
	}

	suffix := pascalCase(name)
	var cloneKeys []string
	for key := range spine {
		if !inPlace[key] {
			cloneKeys = append(cloneKeys, key)
		}
	}
	sort.Strings(cloneKeys)

	existing := map[string]bool{}
	overrides := map[string]string{}
	for i := 0; i+1 < len(schemas.Content); i += 2 {
		existing[schemas.Content[i].Value] = true
		if v := mappingChild(schemas.Content[i+1], nameOverrideExtension); v != nil && v.Kind == yaml.ScalarNode {
			overrides[v.Value] = schemas.Content[i].Value
		}
	}

	projection := &EntityProjection{Name: name, Kind: kind}
	for _, eo := range selectedOps {
		projection.Operations = append(projection.Operations, eo.label)
	}
	sort.Strings(projection.Operations)

	cloneOf := map[string]string{}
	copies := map[string]*yaml.Node{}
	for _, key := range cloneKeys {
		cloneKey := key + ".tf." + suffix
		if existing[cloneKey] {
			return nil, fail("cannot copy %q: component %q already exists", key, cloneKey)
		}
		original := mappingChild(schemas, key)
		base := shortName(key)
		if v := mappingChild(original, nameOverrideExtension); v != nil {
			if v.Kind != yaml.ScalarNode {
				return nil, fail("component %q has a non-string %s", key, nameOverrideExtension)
			}
			base = v.Value
		}
		override := base + suffix
		if other, taken := overrides[override]; taken {
			return nil, fail("cannot copy %q: %s %q is already used by %q", key, nameOverrideExtension, override, other)
		}
		overrides[override] = cloneKey
		existing[cloneKey] = true
		cloneOf[key] = cloneKey
		cp := copyNode(original)
		if cp.Kind != yaml.MappingNode {
			return nil, fail("component %q is not an object schema", key)
		}
		setMappingValue(cp, nameOverrideExtension, scalarNode("!!str", override))
		copies[key] = cp
		projection.Clones = append(projection.Clones, ComponentClone{From: key, To: cloneKey, NameOverride: override})
	}

	// Rebind the entity to the copy of its root schema.
	if cloneKey, ok := cloneOf[entityRoot]; ok {
		cp := copies[entityRoot]
		setMappingValue(cp, entityExtension, scalarNode("!!str", name))
		original := mappingChild(mappingChild(schemas, entityRoot), entityExtension)
		removeEntityName(mappingChild(schemas, entityRoot), original, name)
		projection.Rebinding = &Rebinding{Name: name, From: entityRoot, To: cloneKey}
	}

	rewrite := func(node *yaml.Node) {
		walkRefs(node, func(ref *yaml.Node) {
			if to, rest, ok := splitSchemaRef(ref.Value); ok {
				if cloneKey, cloned := cloneOf[to]; cloned {
					ref.Value = schemaRefPrefix + EscapeToken(cloneKey) + rest
				}
			}
		})
	}
	var inPlaceKeys []string
	for key := range inPlace {
		inPlaceKeys = append(inPlaceKeys, key)
	}
	sort.Strings(inPlaceKeys)
	for _, key := range inPlaceKeys {
		rewrite(mappingChild(schemas, key))
	}
	for _, key := range cloneKeys {
		rewrite(copies[key])
		schemas.Content = append(schemas.Content, scalarNode("!!str", cloneOf[key]), copies[key])
	}
	for _, n := range opSchemas {
		rewrite(n)
	}
	projection.InPlace = inPlaceKeys

	// Apply the scoped operations to the projection.
	mapped := make([]Operation, 0, len(ops))
	for _, op := range ops {
		tokens, _ := ParsePointer(op.Pointer)
		if cloneKey, ok := cloneOf[tokens[2]]; ok {
			tokens[2] = cloneKey
		}
		to := JoinPointer(tokens...)
		projection.Pointers = append(projection.Pointers, PointerRewrite{From: op.Pointer, To: to})
		op.Pointer = to
		op.Entity = nil
		mapped = append(mapped, op)
	}
	if err := ApplyToNode(doc, mapped); err != nil {
		return nil, fmt.Errorf("%s: %w", describe, err)
	}
	return projection, nil
}

// entityOperations lists the operations bound to name, with the construct each
// serves.
func entityOperations(doc *yaml.Node, name string) ([]*entityOperation, error) {
	paths := mappingChild(doc, "paths")
	if paths == nil {
		return nil, fmt.Errorf("document has no paths")
	}
	var out []*entityOperation
	for i := 0; i+1 < len(paths.Content); i += 2 {
		path, item := paths.Content[i].Value, paths.Content[i+1]
		if item.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			method, node := item.Content[j].Value, item.Content[j+1]
			if !operationMethods[method] || node.Kind != yaml.MappingNode {
				continue
			}
			binding := mappingChild(node, entityOperationExtension)
			if binding == nil {
				continue
			}
			resource, datasource, err := entityRoles(binding, name)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", strings.ToUpper(method), path, err)
			}
			if !resource && !datasource {
				continue
			}
			out = append(out, &entityOperation{
				label:      strings.ToUpper(method) + " " + path,
				node:       node,
				resource:   resource,
				datasource: datasource,
			})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].label < out[b].label })
	return out, nil
}

// entityRoles reads an x-speakeasy-entity-operation value. The scalar and list
// forms are the implicit form: the operation serves the entity's resource, and
// also its data source when it includes the read operation, the only one a data
// source performs. The object form names each construct separately, and null
// suppresses one.
func entityRoles(node *yaml.Node, name string) (bool, bool, error) {
	switch node.Kind {
	case yaml.ScalarNode, yaml.SequenceNode:
		matched, read := refsEntity(node, name)
		return matched, matched && read, nil
	case yaml.MappingNode:
		var resource, datasource bool
		for i := 0; i+1 < len(node.Content); i += 2 {
			matched, _ := refsEntity(node.Content[i+1], name)
			switch node.Content[i].Value {
			case terraformResourceKey:
				resource = resource || matched
			case terraformDatasourceKey:
				datasource = datasource || matched
			}
		}
		return resource, datasource, nil
	default:
		return false, false, fmt.Errorf("unsupported %s value", entityOperationExtension)
	}
}

// refsEntity reports whether a scalar or list of "Entity#operation[,operation]"
// references names the entity, and whether any such reference includes read.
func refsEntity(node *yaml.Node, name string) (bool, bool) {
	var matched, read bool
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			return false, false
		}
		parts := strings.Split(node.Value, "#")
		if strings.TrimSpace(parts[0]) != name {
			return false, false
		}
		if len(parts) > 1 {
			for _, op := range strings.Split(parts[1], ",") {
				if strings.TrimSpace(op) == "read" {
					read = true
				}
			}
		}
		return true, read
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.MappingNode {
				item = mappingChild(item, "entityOperation")
				if item == nil {
					continue
				}
			}
			m, r := refsEntity(item, name)
			matched, read = matched || m, read || r
		}
	default:
		// Other node kinds name no entity.
	}
	return matched, read
}

// operationSchemaNodes returns the request body and successful response
// schemas of an operation.
func operationSchemaNodes(eo *entityOperation) ([]*yaml.Node, error) {
	var nodes []*yaml.Node
	collect := func(holder *yaml.Node, what string) error {
		if holder == nil {
			return nil
		}
		if mappingChild(holder, "$ref") != nil {
			return fmt.Errorf("%s of %s is a component reference, which is not supported", what, eo.label)
		}
		content := mappingChild(holder, "content")
		if content == nil {
			return nil
		}
		for i := 0; i+1 < len(content.Content); i += 2 {
			if schema := mappingChild(content.Content[i+1], "schema"); schema != nil {
				nodes = append(nodes, schema)
			}
		}
		return nil
	}
	if err := collect(mappingChild(eo.node, "requestBody"), "the request body"); err != nil {
		return nil, err
	}
	if responses := mappingChild(eo.node, "responses"); responses != nil {
		for i := 0; i+1 < len(responses.Content); i += 2 {
			if isSuccessStatus(responses.Content[i].Value) {
				if err := collect(responses.Content[i+1], "response "+responses.Content[i].Value); err != nil {
					return nil, err
				}
			}
		}
	}
	return nodes, nil
}

func isSuccessStatus(code string) bool {
	return len(code) == 3 && code[0] == '2' && (code[1:] == "XX" || code[1:] == "xx" || (code[1] >= '0' && code[1] <= '9' && code[2] >= '0' && code[2] <= '9'))
}

// collectExternalRefs visits every schema $ref outside components/schemas that
// is not part of a selected operation's request or successful response.
func collectExternalRefs(doc *yaml.Node, selected map[string]bool, visit func(*yaml.Node)) {
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key, value := doc.Content[i].Value, doc.Content[i+1]
		switch key {
		case "components":
			for j := 0; j+1 < len(value.Content); j += 2 {
				if value.Content[j].Value != "schemas" {
					walkRefs(value.Content[j+1], visit)
				}
			}
		case "paths":
			for j := 0; j+1 < len(value.Content); j += 2 {
				path, item := value.Content[j].Value, value.Content[j+1]
				if item.Kind != yaml.MappingNode {
					walkRefs(item, visit)
					continue
				}
				for k := 0; k+1 < len(item.Content); k += 2 {
					method, node := item.Content[k].Value, item.Content[k+1]
					label := strings.ToUpper(method) + " " + path
					if !operationMethods[method] || !selected[label] {
						walkRefs(node, visit)
						continue
					}
					for m := 0; m+1 < len(node.Content); m += 2 {
						switch node.Content[m].Value {
						case "requestBody":
						case "responses":
							responses := node.Content[m+1]
							for r := 0; r+1 < len(responses.Content); r += 2 {
								if !isSuccessStatus(responses.Content[r].Value) {
									walkRefs(responses.Content[r+1], visit)
								}
							}
						default:
							walkRefs(node.Content[m+1], visit)
						}
					}
				}
			}
		default:
			walkRefs(value, visit)
		}
	}
}

// walkRefs calls visit for every schema reference under node: each "$ref"
// value and each discriminator mapping value.
func walkRefs(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i].Value, node.Content[i+1]
			switch {
			case key == "$ref" && value.Kind == yaml.ScalarNode:
				visit(value)
			case key == "discriminator" && value.Kind == yaml.MappingNode:
				if mapping := mappingChild(value, "mapping"); mapping != nil && mapping.Kind == yaml.MappingNode {
					for j := 1; j < len(mapping.Content); j += 2 {
						if mapping.Content[j].Kind == yaml.ScalarNode {
							visit(mapping.Content[j])
						}
					}
				}
			default:
				walkRefs(value, visit)
			}
		}
	case yaml.SequenceNode, yaml.DocumentNode:
		for _, child := range node.Content {
			walkRefs(child, visit)
		}
	default:
		// Scalars and aliases hold no references of their own.
	}
}

// splitSchemaRef parses "#/components/schemas/<key>[/rest]" into the component
// key and the unparsed remainder.
func splitSchemaRef(ref string) (string, string, bool) {
	if !strings.HasPrefix(ref, schemaRefPrefix) {
		return "", "", false
	}
	tail := ref[len(schemaRefPrefix):]
	token, rest := tail, ""
	if idx := strings.Index(tail, "/"); idx >= 0 {
		token, rest = tail[:idx], tail[idx:]
	}
	key, err := UnescapeToken(token)
	if err != nil || key == "" {
		return "", "", false
	}
	return key, rest, true
}

func mappingChild(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	if idx := findKey(node, key); idx >= 0 {
		return node.Content[idx+1]
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	if idx := findKey(node, key); idx >= 0 {
		node.Content[idx+1] = value
		return
	}
	node.Content = append(node.Content, scalarNode("!!str", key), value)
}

func copyNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	cp := *node
	cp.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		cp.Content[i] = copyNode(child)
	}
	return &cp
}

// entityNames reads an x-speakeasy-entity value, a name or a list of names.
func entityNames(node *yaml.Node) map[string]bool {
	names := map[string]bool{}
	if node == nil {
		return names
	}
	switch node.Kind {
	case yaml.ScalarNode:
		names[node.Value] = true
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				names[item.Value] = true
			}
		}
	default:
		// Other node kinds bind no entity.
	}
	return names
}

// removeEntityName drops name from the x-speakeasy-entity of schema. A list
// left with one name collapses to that name, and a schema left with none loses
// the key.
func removeEntityName(schema, value *yaml.Node, name string) {
	if value == nil {
		return
	}
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Value == name {
			if idx := findKey(schema, entityExtension); idx >= 0 {
				schema.Content = append(schema.Content[:idx], schema.Content[idx+2:]...)
			}
		}
	case yaml.SequenceNode:
		kept := make([]*yaml.Node, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind == yaml.ScalarNode && item.Value == name {
				continue
			}
			kept = append(kept, item)
		}
		switch len(kept) {
		case 0:
			if idx := findKey(schema, entityExtension); idx >= 0 {
				schema.Content = append(schema.Content[:idx], schema.Content[idx+2:]...)
			}
		case 1:
			setMappingValue(schema, entityExtension, kept[0])
		default:
			value.Content = kept
		}
	default:
		// Other node kinds hold no entity names.
	}
}

// pascalCase turns an entity name into an identifier fragment:
// "Custom App Entitlement" becomes "CustomAppEntitlement".
func pascalCase(name string) string {
	var out strings.Builder
	upper := true
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			upper = true
			continue
		}
		if upper {
			r = unicode.ToUpper(r)
			upper = false
		}
		out.WriteRune(r)
	}
	return out.String()
}

func shortName(key string) string {
	if idx := strings.LastIndex(key, "."); idx >= 0 {
		return key[idx+1:]
	}
	return key
}

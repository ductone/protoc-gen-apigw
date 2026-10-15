# Terraform customization overlay contract

`protoc-gen-apigw` transports OpenAPI customization annotations from proto to a
**Terraform-scoped companion artifact** without changing the shared OpenAPI
document. This document is the normative contract for the proto surface, the
destination model and the emitted artifact.

It exists so that a new `x-speakeasy-*` extension, or a new OpenAPI placement,
never requires a change to — or a release of — `protoc-gen-apigw`.

## Why a companion artifact

Speakeasy's Terraform customizations are OpenAPI extensions and schema keywords
that only the Terraform generator consumes. The same OpenAPI document also
feeds the public Go/TypeScript/etc. SDKs. Emitting a Terraform-only override
into the shared document would rename public SDK symbols, so Terraform-scoped
overrides travel beside the document instead:

```
proto annotations ──▶ protoc-gen-apigw ──┬──▶ <pkg>.<service>.oas31.yaml                     (shared, unchanged)
                                         ├──▶ <pkg>.<service>.terraform_overlay.yaml         (Terraform only)
                                         └──▶ <pkg>.<service>.terraform_entity_overlay.yaml  (Terraform only, per entity)
```

Both artifacts are [OpenAPI Overlay 1.1.0](https://spec.openapis.org/overlay/v1.1.0.html)
documents. A consumer (a spec aggregation step, then the provider-generation
pipeline) applies them to the merged OpenAPI document immediately before
invoking the Terraform generator, and not to the document it publishes.

## Proto surface

Status: implemented and covered by tests. The proto surface, the destination
table, the artifact format and the precedence rules below are exercised by the
tests named in each section and by
[docs/terraform-customization-coverage.md](terraform-customization-coverage.md).
Which attribute name the pinned generator derives from a given override value
is not asserted here: the transport guarantees the annotation arrives at its
destination, and the generator decides what to do with it.

Five annotation owner kinds, each carrying the same two lists:

| Owner | Option | Added fields |
| --- | --- | --- |
| file | `(apigw.v1.file)` on `google.protobuf.FileOptions` | `customizations`, `schema_patches` |
| service | `(apigw.v1.service).service` | `customizations` (5), `schema_patches` (6) |
| method operation | `(apigw.v1.method).operations[]` | `customizations` (15), `schema_patches` (16) |
| message | `(apigw.v1.message).message_options[]` | `customizations` (6), `schema_patches` (7) |
| field | `(apigw.v1.field).field_options[]` | `customizations` (6), `schema_patches` (7) |

Both lists accept an optional `entities` selector that restricts an override to
one Terraform entity; see [Entity scope](#entity-scope).

Two further kinds of declaration are Terraform-only and described in
[Selectors and property operations](#selectors-and-property-operations):

| Owner | Field | Purpose |
| --- | --- | --- |
| file, service | `selector_customizations` (file 3, service 7) | one override for every node of the whole document a selector matches |
| message | `property_patches` (8) | add or remove one property of a schema |

Note that `operations[]` is a repeated field but the generator emits only
`operations[0]` today. An annotation on `operations[1..]` is therefore a
generation error, not a silent drop, until the generator emits every operation.

Repetition is meaningful: every entry of every `message_options` /
`field_options` entry contributes.

### `OpenAPICustomization`

```proto
message OpenAPICustomization {
  string key = 1;                    // vendor extension key, must start with "x-"
  string value_json = 2;             // JSON-encoded value
  OpenAPICustomizationTarget target = 3;
  string json_pointer = 4;           // RFC 6901, absolute in the emitted document
  OpenAPICustomizationScope scope = 5;
  OpenAPICustomizationMode mode = 6;
  repeated OpenAPICustomizationEntity entities = 7;  // empty = every consumer
}
```

### `OpenAPISchemaPatch`

```proto
message OpenAPISchemaPatch {
  OpenAPICustomizationTarget target = 1;
  string json_pointer = 2;
  string patch_json = 3;             // JSON object of schema keywords to set
  repeated string remove_keys = 4;   // schema keywords to remove
  OpenAPICustomizationScope scope = 5;
  repeated OpenAPICustomizationEntity entities = 6;  // empty = every consumer
}
```

Vendor extensions and standard schema keywords are deliberately **separate
channels**. A schema keyword (`default`, `example`, `required`, `deprecated`,
`oneOf`, `contentMediaType`, ...) must not be disguised as an `x-` extension,
and a vendor extension must not be smuggled through the patch channel: the
generation fails if a `customizations[*].key` does not start with `x-`, or if a
`schema_patches[*]` keyword does.

### Proto syntax example

```proto
import "apigw/v1/apigw.proto";

message Book {
  option (apigw.v1.message).message_options = {
    terraform_entity: {name: "Book"}
    customizations: {
      key: "x-speakeasy-name-override"
      value_json: "\"Book\""
      scope: OPEN_API_CUSTOMIZATION_SCOPE_TERRAFORM
    }
  };

  string shelf_id = 1 [(apigw.v1.field).field_options = {
    customizations: {
      key: "x-speakeasy-param-readonly"
      value_json: "true"
    }
  }];

  // A rename that applies to one Terraform entity only. The Book component is
  // also the schema of a second entity, "Archived Book". Without `entities`
  // the same annotation would rename the property in every entity that reaches
  // the Book component through $ref, including entities whose attribute must
  // not change. The value is data: the contract guarantees it reaches the
  // overlay at this property's pointer, not that the pinned generator derives
  // a particular attribute name from it.
  string author = 2 [(apigw.v1.field).field_options = {
    customizations: {
      key: "x-speakeasy-name-override"
      value_json: "\"writer\""
      entities: {
        name: "Archived Book"
        kind: OPEN_API_CUSTOMIZATION_ENTITY_KIND_RESOURCE
      }
    }
  }];

  // Standard schema keywords go through the patch channel.
  repeated string quotes = 3 [(apigw.v1.field).field_options = {
    schema_patches: {
      patch_json: "{\"default\": []}"
    }
  }];
}
```

An *unscoped* `x-speakeasy-name-override` on a shared message's field renames
that property for every entity that reaches the component. That is a global
change by construction; use `entities` when only one entity should change.

## Value model

`value_json` and `patch_json` are strict JSON. The parser:

- preserves integers as integers and decimals as decimals (no float round-trip
  through `float64`, so `9007199254740993` and `1.50` survive);
- preserves `false`, `0`, `""`, `[]` and `{}` — a falsy value is a value;
- **rejects** malformed JSON, trailing content after the first value, duplicate
  object keys at any level, and an absent value where one is required. Each
  failure is reported against the owning descriptor (message/field/operation)
  and the offending key;
- preserves arbitrary nesting depth.

`null` is a value. It is never an instruction to delete; deletion is
`mode: REMOVE` (or `remove_keys`).

## Destinations

`target` resolves relative to the annotated entity. `OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER`
uses `json_pointer` verbatim as an absolute pointer into the emitted document
(the escape hatch for placements this contract does not enumerate). Supplying
`json_pointer` with any other target, or omitting it for the pointer target, is
a generation error. The pointer escape hatch is legal on **every** owner kind,
including service and file, so a future document-level placement never needs a
new target value or a new `apigw` release.

An empty pointer string addresses the document root. It is therefore *equal* to
`DOCUMENT_ROOT`, not the absence of a destination, and the two are told apart by
`target`: `DOCUMENT_ROOT` is the default when `json_pointer` is empty, and
`json_pointer: ""` with `OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER` is the same
node reached explicitly. Omitting `target` and omitting `json_pointer` selects
the owner default. Both spellings resolve to the document root and both are
covered by tests.

| Owner | Legal targets | Default | Emitted destination |
| --- | --- | --- | --- |
| field | `OPEN_API_CUSTOMIZATION_TARGET_SCHEMA` | this | the schema node emitted for the field |
| field | `OPEN_API_CUSTOMIZATION_TARGET_PARAMETER` | — | the `parameters[i]` object when the field is a path/query parameter |
| field | `OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS` | — | `items` of a repeated field's schema |
| field | `OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES` | — | `additionalProperties` of a map field's schema |
| field | `OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER` | — | `json_pointer` |
| message | `OPEN_API_CUSTOMIZATION_TARGET_SCHEMA` | this | `/components/schemas/<message>` |
| message | `OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER` | — | `json_pointer` |
| operation | `OPEN_API_CUSTOMIZATION_TARGET_OPERATION` | this | `/paths/<route>/<method>` |
| operation | `OPEN_API_CUSTOMIZATION_TARGET_REQUEST_SCHEMA` | — | `/paths/<route>/<method>/requestBody/content/application~1json/schema` |
| operation | `OPEN_API_CUSTOMIZATION_TARGET_RESPONSE_SCHEMA` | — | `/paths/<route>/<method>/responses/200/content/application~1json/schema` |
| operation | `OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER` | — | `json_pointer` |
| service/file | `OPEN_API_CUSTOMIZATION_TARGET_DOCUMENT_ROOT` | this | the document root (pointer `""`) |
| service/file | `OPEN_API_CUSTOMIZATION_TARGET_JSON_POINTER` | — | `json_pointer`, including `""` for the document root |

A file-level annotation applies to every document generated from that file, so a
file with two services contributes the same root operation to both overlays.

An illegal owner/target combination (for example `ARRAY_ITEMS` on a
non-repeated field, `PARAMETER` on a field that is never a path or query
parameter, `SCHEMA` on an operation) fails generation, as does an annotation
whose destination does not exist in the emitted document. Annotations are never
silently dropped: every annotation declared in a generated file must be
consumed by at least one emitted document, and generation fails listing any
annotation that was not.

### Field schema placement

`OPEN_API_CUSTOMIZATION_TARGET_SCHEMA` (and the owner default for a field) means
"the schema node emitted for this field", wherever that field's schema is
emitted — as a property of its message component, or as a parameter's `schema`.
The node is:

- the **inline schema** for scalar, enum, repeated and map fields;
- the **`oneOf: [ $ref, {type: null} ]` wrapper** for a singular message-typed
  field. The override attaches to the wrapper, not to the referenced component,
  so one field's override does not leak onto every other use of that message;
- the **inline well-known-type schema** for a WKT-typed field, with `null`
  already folded into its type set.

`OPEN_API_CUSTOMIZATION_TARGET_ARRAY_ITEMS` and
`OPEN_API_CUSTOMIZATION_TARGET_MAP_VALUES` additionally descend one level. For a
repeated message-typed field the item node is the element's `$ref`; the override
is attached as a sibling of `$ref`, which OpenAPI 3.1 permits.

## Precedence

1. Baseline emission runs first: generated defaults and the existing typed
   annotations (`required_spec`, `read_only_spec`, `terraform_entity`, pagination,
   `name_override`, `group_override`, stability, deprecation, `annotation_bag`)
   are applied exactly as before.
2. Generic `customizations` and `schema_patches` are applied afterwards, in a
   deterministic order (owner order, then declaration order within an owner).
3. A `SET` replaces the **complete** value at its exact key. There is no
   recursive array or object merge: overriding `x-speakeasy-pagination.outputs`
   replaces `outputs` wholesale.
4. `REMOVE` deletes the key. If the key is not present at the destination, the
   generation fails — a removal that removes nothing is a bug in the annotation.
5. Two explicit assignments of the same key at the same destination:
   - identical values **deduplicate** into one operation;
   - different values are a **generation error** naming both owners. There is
     no silent last-writer-wins.
6. Operations that write to **overlapping destinations** are rejected, because
   no ordering of them can be shown to preserve both. Two operations overlap
   when their write paths — the destination pointer's reference tokens followed
   by the key — are token-wise equal, or when one is a strict ancestor of the
   other. For example, setting the schema keyword `properties` on
   `/components/schemas/X` and setting `default` on
   `/components/schemas/X/properties/foo` both have unique
   `(pointer, channel, key)` triples, yet applying the first then the second
   keeps the second and the reverse order discards it. Comparison is
   token-aware, so `/X/properties/bar` does not overlap `/X/properties/foo`,
   and two different keys on one node do not overlap.
   **Containment ignores channels.** A write to an ancestor destroys its
   descendants whatever channel either is on, so the schema-channel write above
   also overlaps an extension-channel write to
   `/components/schemas/X/properties/foo` with key `x-test`. Removing a key is a
   write at the same location and overlaps the same way.

Byte-for-byte identity of the shared document when no customization is declared
is a **test requirement of this work**, not an established property of the
current generator: `TestNoAnnotationsLeaveOutputUnchanged` regenerates the
bookstore example and compares the `.oas31.yaml` (and the emitted Go body)
against the committed artifacts.

One behavior does change deliberately, and it is not a no-op: the
required field-metadata propagation fix described under "Field schema
placement" makes typed field annotations survive the nullable-`$ref` wrapper
and the inline well-known-type path, where they used to be dropped. Fixtures
that carry no typed annotation on such a field are unaffected; the bookstore
example gained exactly one key (`x-stability-level: stable` on
`bookstore.v1.Author.createdAt`) and the tfcustomize fixture gains
(`x-stability-level: stable` on the singular message field `child` and on the
well-known-typed `createdAt`). Both are asserted by
`TestEmbeddedFieldExtensionsSurvive`. The change adds metadata the proto author
already declared; it renames no symbol and removes nothing.

Deterministic sorting is a rendering property only. Because at most one
operation survives per `(pointer, channel, key)`, contradictory operations fail
generation, and overlapping operations fail generation, applying an overlay
never depends on operation order for its result. `TestApply` asserts that
directly by applying a set of operations forward and backward and requiring
identical output.

## Overlay artifact

For each emitted OpenAPI document, `protoc-gen-apigw` emits up to two companion
artifacts beside it. A part with nothing in it is not emitted.

| OpenAPI artifact | Global part | Entity part |
| --- | --- | --- |
| `<dir>/<base>.<service>.oas31.yaml` | `<dir>/<base>.<service>.terraform_overlay.yaml` | `<dir>/<base>.<service>.terraform_entity_overlay.yaml` |
| `<dir>/<base>.<package>.oas31.yaml` | `<dir>/<base>.<package>.terraform_overlay.yaml` | `<dir>/<base>.<package>.terraform_entity_overlay.yaml` |

Both are **[OpenAPI Overlay 1.1.0](https://spec.openapis.org/overlay/v1.1.0.html)
documents** whose targets are [RFC 9535](https://www.rfc-editor.org/rfc/rfc9535)
JSONPath queries.

- The **global part** holds selectors and global operations. Standard tooling,
  for example `speakeasy overlay apply` or the
  [`speakeasy-api/openapi` overlay package](https://github.com/speakeasy-api/openapi/tree/main/overlay),
  applies it directly, and gets exactly the document `tfoverlay` computes:
  `VerifyStandard` checks that equality against the document the overlay is
  applied to.
- The **entity part** holds entity-scoped operations. A generic Overlay tool would
  apply them to every entity that reaches the component, so only `ApplyTerraform`
  may consume it. Its title says so, and `x-apigw-terraform.part: entities`
  marks it.

No artifact is emitted for a document that has no operations, so a repository
that has not adopted the annotations sees no new files.

```yaml
overlay: 1.1.0
info:
  title: bookstore.v1.BookstoreService Terraform customizations
  version: 0.0.0
actions:
  - target: $['components']['schemas']['bookstore.v1.Book']
    update:
      x-speakeasy-entity: Book
    x-apigw-provenance: message bookstore.v1.Book
  - target: $['components']['schemas']['bookstore.v1.Book']['properties']['quotes']
    update:
      default: []
    x-apigw-provenance: field bookstore.v1.Book.quotes
  - target: $['paths']['/shelves/{shelf}/books/{book}']['get']['x-speakeasy-entity-missing-codes']
    remove: true
    x-apigw-replace: true
  - target: $['paths']['/shelves/{shelf}/books/{book}']['get']
    update:
      x-speakeasy-entity-missing-codes:
        - 403
        - 410
    x-apigw-provenance: method bookstore.v1.BookstoreService.GetBook
x-apigw-terraform:
  kind: service
  part: global
  source: bookstore.v1.BookstoreService
```

How an operation is spelled:

- An exact destination is an RFC 9535 **normalized path** (`$['a'][0]['b']`),
  derived from the RFC 6901 pointer the generator resolved. The emitted document
  decides whether a token is an array index.
- A `set` is an `update` of one key on its destination. An Overlay `update` merges
  objects, concatenates arrays and keeps the tag of an existing scalar, so when
  the destination already holds that key and the update would not simply replace
  it — the existing value or the new value is an object or array, or the scalar
  type differs — the set is preceded by a `remove` of that key marked
  `x-apigw-replace: true`. The pair replaces the complete value, as the
  precedence rules require.
- A `remove` is a `remove` of the key's normalized path. Speakeasy's strict
  application fails when a target matches nothing, so a remove of an absent key
  fails there too.
- A `property` operation (see below) is an `update` or `remove` on the
  `properties` object, marked `x-apigw-property: true`.
- A selector is an action whose target is the selector's JSONPath, marked
  `x-apigw-selector: true`. Selectors come first, in canonical order, then exact
  operations in canonical order `(pointer, channel, key)`.
- `x-apigw-provenance` is the descriptor identity of the annotation that produced
  the action. It is advisory metadata, never interpreted.
- `x-apigw-entity` (entity part only) restricts the action to one Terraform entity.
  See [Entity scope](#entity-scope).
- `x-apigw-terraform` records the owning `source`, its `kind` (`service` or
  `package`) and the `part`.

Rendering needs the document the overlay applies to, because whether a set needs
its `remove` depends on what the destination holds. Two runs over identical input
produce identical bytes.

### Consumer contract

The format is read and written through the importable Go package
`github.com/ductone/protoc-gen-apigw/tfoverlay`, which also holds the checks
the Overlay Specification does not define (conflicts, overlaps and entity
projection):

| Function | Contract |
| --- | --- |
| `Parse([]byte) (*Overlay, error)` | Read either part. Strict: an action shape `Render` does not produce (a `copy`, an update of several keys, an exact target that is not a normalized path, an unknown extension), a duplicate mapping key, a wrong Overlay version and a trailing second YAML document are all errors. Parsed operations go through the same validation as generated ones, so a duplicate, contradictory or overlapping set is rejected here. |
| `Merge(...*Overlay) (*Overlay, error)` | Combine the per-service overlays of a merged document. Identical operations deduplicate; contradictory and overlapping ones fail with **both** contributing sources named, never only the incoming one. |
| `(*Overlay).Render(doc []byte) (*Rendered, error)` | Write the global and entity parts, spelled against `doc`. |
| `VerifyStandard(doc []byte, ov *Overlay) error` | Apply the rendered global part to `doc` with the Speakeasy Overlay implementation in strict mode and require the same document as `ApplyOverlay`. A consumer runs it on the merged document. |
| `ApplyOverlay(doc []byte, ov *Overlay) ([]byte, *ProjectionReport, error)` | The full two-stage application: selectors against the document, then `ApplyTerraform` for the exact operations. This is the entry point for a consumer that applies a merged overlay that may hold entity-scoped operations. See [Selectors and property operations](#selectors-and-property-operations). |
| `ApplyTerraform(doc []byte, ops []Operation) ([]byte, *ProjectionReport, error)` | Apply a list that may hold entity-scoped operations: global operations first, then each entity's operations on that entity's projection. See [Entity scope](#entity-scope). |
| `ProjectionOverlay(title string, before, after []byte) ([]byte, error)` | Express a projection (or any change) as a standard Overlay generated by the Speakeasy comparison, checked to reproduce `after` from `before`. |
| `ValidateTerraform(doc []byte, ops []Operation) error` | `Validate` for global operations; for scoped operations, checks that the destination resolves to an object. Reachability from the entity is decided by `ApplyTerraform`. |
| `Apply(doc []byte, ops []Operation) ([]byte, error)` | Validate the list as a whole, then apply **its canonical deduplicated form**. Both entrypoints operate on that same list, so two identical `remove` operations are accepted by both and remove the key once. A list that is internally contradictory or overlapping fails, so a caller cannot get an order-dependent result by forgetting `Merge`. An entity-scoped operation is rejected ("use ApplyTerraform") so a scoped override can never be applied globally by accident. A pointer that does not resolve, a destination that is not an object, or a `remove` of an absent key is an error. |
| `Validate(doc []byte, ops []Operation) error` | The same checks on the same canonical list, without modifying the document: whole-list consistency and an object destination. A list `Validate` accepts is a list `Apply` can perform. |

Validating one list and applying another is the bug class this rules out:
deduplication happens once, and the result is what every entrypoint uses.

### Diagnostics

Failures are `*tfoverlay.Error`, which carries `Pointer`, `Channel`, `Key`,
`Owners` (the contributing annotations, by descriptor identity) and a human
message. Match it with `errors.As`; do not parse the message. Every operation
also carries `Provenance` — the descriptor identity of the annotation that
produced it, for example `field bookstore.v1.Book.author`
— so a checker can report which proto declaration wrote a destination without
re-deriving it.

A proto **source location** (`file:line`) is not part of the artifact: it would
change the artifact on unrelated edits. This is a known gap; the descriptor
identity is stable and sufficient to locate the declaration.

`Merge` is the cross-document check the per-file generator cannot perform:
root-level (document root) operations contributed by several services either
agree or are rejected, and are never silently ordered by file name. Pointers are
absolute in the service document and stay valid in the merged document because
`apigw` derives component names and paths itself and the merger concatenates
both `/paths` and `/components/schemas`; two services that contribute the same
pointer and key must therefore agree, which `Merge` enforces.

## Entity scope

An override addressed at a schema component lands in every entity that reaches
the component through `$ref`. `entities` restricts it to one entity's
projection of the document. Scope and entity scope are independent dimensions
of every customization: `scope` says *which output*, `entities` says *which
consumers within the Terraform output*.

```proto
message OpenAPICustomizationEntity {
  string name = 1;                            // exact x-speakeasy-entity name
  OpenAPICustomizationEntityKind kind = 2;    // unspecified | resource | data source
}
```

Generation-time rules (a violation is a generation error):

- A non-empty `entities` requires the Terraform scope. The shared document has
  no entities.
- The destination must be inside `/components/schemas/<name>`: a field `SCHEMA`,
  `ARRAY_ITEMS` or `MAP_VALUES`, a message `SCHEMA`, or a `JSON_POINTER` below
  `/components/schemas/<name>`. Operation, parameter and document-root targets
  are rejected, because an operation already belongs to specific entities.
- `name` must be non-empty, `kind` must be known, and a `(name, kind)` pair may
  appear once per annotation.
- apigw does **not** check that the entity exists: one emitted document may not
  contain the entity's operations. The consumer resolves the entity.
- A field emitted only as an HTTP parameter in a document carries no scoped
  operation there; generation fails if no document applies the annotation.

Each entity selector produces one action in the **entity part**, which carries
an `x-apigw-entity` mapping (`kind` is omitted when it addresses both the
resource and the data source):

```yaml
overlay: 1.1.0
info:
  title: bookstore.v1.BookstoreService Terraform customizations (entity-scoped; apply with tfoverlay only)
  version: 0.0.0
actions:
  - target: $['components']['schemas']['bookstore.v1.Book']['properties']['author']
    update:
      x-speakeasy-name-override: writer
    x-apigw-entity:
      kind: resource
      name: Archived Book
    x-apigw-provenance: field bookstore.v1.Book.author
x-apigw-terraform:
  kind: service
  part: entities
  source: bookstore.v1.BookstoreService
```

Global and entity actions never share a file, so a consumer that applies the
global part with standard tooling cannot apply a scoped override to every
entity by accident. `Parse` rejects an entity action in the global part and a
global action in the entity part.

### Conflicts

Conflict and overlap detection is per entity scope:

| Case | Rule |
| --- | --- |
| Two global operations | Unchanged. |
| Global and scoped at the same destination and key | Allowed. Global operations apply first, the entity's copy inherits them, and the scoped operation replaces the value on that copy only. |
| Two scoped operations, same entity, same destination and key | Identical values deduplicate; different values are an error naming both owners. |
| Scoped operations, same entity, overlapping destinations | Error, by the same token-aware rule, evaluated per entity. |
| Scoped operations for different entities at one destination | Never a conflict. They land on different copies. |
| An unspecified-kind selector and a kind selector of one entity | Treated as the same entity: differing values are an error. |
| A resource and a data source of one entity that share operations and receive differing operations | An error in `ApplyTerraform`. They cannot be customized differently yet. |

### Projection: `ApplyTerraform`

`ApplyTerraform` runs on the final Terraform input document, after every
provider-side overlay, and before the Terraform generator. It builds the minimal
projection of each entity named by a scoped operation:

1. Apply the global operations, so every copy inherits them.
2. Find the entity's operations: those whose `x-speakeasy-entity-operation`
   names it. The scalar and list forms bind the resource, and the data source
   too when they include `read`; the object form binds each construct
   separately, and `null` suppresses one. A kind selector with an operation that
   also serves the other kind is an error ("cannot split resource and data
   source that share operation"); give the operation an explicit
   `terraform-resource` or `terraform-datasource` of `null`, or scope the
   override to the whole entity.
3. Find the entity's root schema: exactly one component whose
   `x-speakeasy-entity` contains the name.
4. Find the spine: the components on a path from the entity's request and
   successful response schemas to a destination component. A destination the
   entity cannot reach is an error, because the override would do nothing.
5. Copy on write. A spine component is edited in place only when every referrer
   is the entity's operations or another in-place spine component; otherwise it
   is copied, and a cycle containing an externally referenced member is copied
   whole. Components below the destination stay shared.
   A copy is named `<key>.tf.<EntityName>` (for example
   `bookstore.v1.Book.tf.ArchivedBook`) and gets
   `x-speakeasy-name-override: <existing override or short name><EntityName>`. A
   collision with an existing key or override is an error.
6. Rewrite the `$ref`s and discriminator mappings of spine components and of the
   entity's operations to the copies.
7. Move the entity binding. The copy of the root schema carries
   `x-speakeasy-entity: <name>`; the name is removed from the original, and a
   one-element list collapses to a string.
8. Apply the entity's operations at the copies' pointers.

The returned `ProjectionReport` lists, per entity, the operations, the copies,
the in-place components, the rebinding and the pointer rewrites, so CI can
compare a projection with an expectation.

The result is a pure function of the document and the operation set: entities
are processed in name order and components in key order, and permuting the
operations cannot change the output. Applying the same operations to a
projection's own output fails, because the destination is no longer reachable
from the entity's operations; apply once, to the unprojected document.

**A projection changes generated SDK types.** The entity's operations are
re-pointed at the copies, so the generator emits new types for them and the
types those operations used before no longer apply to them. A hand-written
caller that shares those operations with the projected entity (for example a
resource listed in `.genignore` that calls the same SDK operations) stops
compiling or changes behavior and must be updated alongside the projection. The
projection never changes any other operation or any other entity's schema.

The `cmd/tfoverlay` command wraps these functions for builds and CI:

```
tfoverlay merge    -spec openapi.yaml -o merged.yaml -entities merged.entities.yaml a.terraform_overlay.yaml b.terraform_entity_overlay.yaml
tfoverlay validate -spec openapi.yaml merged.yaml merged.entities.yaml
tfoverlay apply    -spec openapi.yaml -o terraform.yaml -report report.json -projection-overlay projection.yaml merged.yaml merged.entities.yaml
```

`merge` and `validate` run `VerifyStandard` on the global part. `apply
-projection-overlay` writes the change from the spec to the output as a standard
Overlay, so the projection can be reviewed and applied with standard tooling.

## Selectors and property operations

Some rules belong to the document rather than to one declaration: every property
named `deletedAt` is a soft-delete property; no schema is open
(`additionalProperties: true`) unless it says so; a generated `@type`
discriminator property is not a Terraform attribute. Annotating every site
would regress whenever a new message forgets it. A **selector** states the rule
once. A **property operation** adds or removes one child of `properties`, which
neither the extension nor the schema channel can express.

Both are Terraform-only (the shared scope is a generation error) and
deterministic. They exist so that every action of a provider-side overlay can be
expressed in proto.

### Selectors

`selector_customizations` on a file or service carries an `OpenAPISelector`
holding one [RFC 9535](https://www.rfc-editor.org/rfc/rfc9535) JSONPath query,
and an action. The query is the target of the emitted Overlay action, so the
matching semantics are those of the standard tooling that applies it, for
example:

| Rule | `json_path` |
| --- | --- |
| every property named `deletedAt` | `$..deletedAt` |
| every schema that is open | `$..[?(@.additionalProperties == true)]` |
| every generated `@type` property | `$..['@type']` |

Actions: `SET` writes `key` (an `x-` key is the extension channel, anything else
a schema keyword) on each matched node, which must be an object; the value must
be a scalar, because an Overlay update merges an object or array into what is
already there. `REMOVE` deletes each matched node from its parent. A query that
selects the document root is rejected.

In the global part:

```yaml
actions:
  - target: $..[?(@.additionalProperties == true)]
    update:
      additionalProperties: false
    x-apigw-selector: true
  - target: $..['@type']
    remove: true
    x-apigw-selector: true
  - target: $..deletedAt
    update:
      x-speakeasy-soft-delete-property: true
    x-apigw-selector: true
```

**Matching and count.** A selector is resolved against the document the overlay
is applied to, which for a consumer is the merged shared spec, not the
single-service document that carried the annotation. apigw therefore cannot know
the match count at generation time and does not check it. `ApplyOverlay` fails
when a selector matches no node, as Speakeasy's strict application does, so a
rule that stopped applying is not silent, and returns a `SelectorReport` per
selector with `Matches` and `Overridden`.

**Stages and precedence.** `ApplyOverlay` runs in two stages. Stage one resolves
every selector against the unmodified document and then writes; stage two
applies the exact operations (`ApplyTerraform`, including global and
entity-scoped operations) to the result. The rendered Overlay lists the
selectors before the exact operations, so sequential standard application has
the same order, and `VerifyStandard` checks the results agree. The contract for
an exact operation on a node a selector matched is fixed:

> **The exact operation wins, explicitly.** It sees the document the selectors
> produced and replaces what they wrote at its destination. This is not an
> overlap and not an error. It is how a rule with exceptions is spelled: a
> selector sets `additionalProperties: false` everywhere, and two exact
> operations set it back to `true` on the two nodes that need it.

The consequences are exact:

- An exact `set` on a node and key a selector wrote replaces the value. The
  report counts it in the selector's `Overridden`.
- An exact `remove` of a key a selector removed or never wrote fails, as any
  remove of an absent key does.
- A property operation that adds a property a selector removed succeeds: the
  property is absent at stage two. This is how `@type` is removed everywhere and
  then added back on two nodes.
- The overlap rule (`Add`) is unchanged and applies among exact operations only.
  Selectors are not exact operations and never overlap with them.

Selectors are checked against each other, since stage one does not order them.
Two selector operations for one query and key with different values, or a set
together with a remove of one query, conflict in `AddSelector` and `Merge`;
identical ones deduplicate. At application, two selectors that write one node and
key with different values, or one selector that removes a node another selector
writes inside, are errors.

### Property operations

A property operation addresses the `properties` object of a schema: the target is
that object, the key is the property name, and a set carries the property's
schema (an object). It is marked `x-apigw-property: true`. Unlike a keyword set,
it never merges:

- `set` **adds** the property and fails if it already exists;
- `remove` deletes it and fails if it is absent.

`property_patches` on a message produces them, with `name`, `schema_json`
(required for `SET`, empty for `REMOVE`) and a target of the message's own schema
or a `JSON_POINTER` to any schema under `/components/schemas/<name>`, so a
property of a nested inline schema is addressable. They compose with
[Entity scope](#entity-scope). Because they use the same
`(pointer, key)` identity as other operations, a property operation and an exact
write inside the property it adds overlap and are rejected.

`Validate` checks the destination and, for
a remove, that the property exists; the add-must-be-absent check is made only
by application, because the property may legitimately exist until a selector
removes it.

## Scope

`OPEN_API_CUSTOMIZATION_SCOPE_TERRAFORM` (also `UNSPECIFIED`, the default)
writes to the overlay only. The shared `.oas31.yaml` document is unchanged.

`OPEN_API_CUSTOMIZATION_SCOPE_SHARED` applies the override to the shared
document **instead of** recording it in the overlay. It is never a no-op and
never a duplicate: a shared-scoped operation appears in the `.oas31.yaml` and
not in the `.terraform_overlay.yaml`. Because it can rename public SDK symbols
it is an explicit opt-in, and when any shared-scoped customization is present the
generator re-parses the rendered document and fails generation if the result is
not a valid OpenAPI document. No consumer in the current Terraform pipeline uses
shared scope.

A mixed document is legal: a `SHARED` operation and a `TERRAFORM` operation may
target the same pointer and key, in which case the shared document carries the
value and the overlay carries it again for the Terraform pipeline. Within one
document an operation is recorded exactly once — either in the shared document
or in the overlay — and a `SHARED` and a `TERRAFORM` assertion of the *same*
key at the *same* pointer with *different* values fails generation, because the
Terraform pipeline would otherwise silently disagree with the published
document.

## Test evidence

| Concern | Test |
| --- | --- |
| Value model: every shape, precision, duplicate keys, malformed, trailing | `tfoverlay.TestParseJSONValue` |
| Pointer escaping and parsing | `tfoverlay.TestPointerRoundTrip` |
| Deduplication, contradiction, overlap, channel separation | `tfoverlay.TestOverlayAddDeduplicatesAndConflicts`, `tfoverlay.TestOverlayRejectsOverlappingDestinations` |
| Canonical ordering; standard Overlay rendering, replace pairs, normalized paths and round trip; Speakeasy application equals `Apply` | `tfoverlay.TestOverlayNormalizeIsOrderIndependent`, `tfoverlay.TestRenderMatchesStandardApplication`, `tfoverlay.TestNormalizedPathRoundTrip` |
| Operation validation (keys, channels, modes, pointers) | `tfoverlay.TestOverlayValidateOperation` |
| Cross-document merge, root conflicts, schema conflicts, both-source diagnostics | `tfoverlay.TestMerge`, `tfoverlay.TestMergeNamesBothSources` |
| Artifact strictness: shapes `Render` does not produce, unknown extensions, versions, duplicate keys, trailing documents | `tfoverlay.TestParseRejectsShapesRenderDoesNotProduce`, `tfoverlay.TestEntityPartFormat` |
| Entrypoints reject inconsistent lists and non-object destinations | `tfoverlay.TestApplyAndValidateRejectInconsistentLists`, `tfoverlay.TestValidateRequiresObjectDestination` |
| Structured errors and per-operation provenance | `tfoverlay.TestErrorsAreStructured` |
| Duplicate operations behave identically through every entrypoint | `tfoverlay.TestDuplicateOperationsHaveEntrypointParity` |
| Apply and validate against a document, order independence | `tfoverlay.TestApply`, `tfoverlay.TestValidate` |
| Entity selector validation and scoped operation emission | `apigw.TestEntitySelectorValidation`, `apigw.TestEntityScopedAnnotationEmitsScopedOperations`, `apigw.TestEntityScopedAnnotationAtAParameterIsNotApplied`, `apigw.TestEntityScopedAndSharedOperationsAtOneDestinationCoexist`, `apigw.TestFinishDocumentChecksScopedDestinations` |
| Entity part format, separation from the global part, committed artifacts round-trip | `tfoverlay.TestEntityPartFormat`, `tfoverlay.TestVerifyStandardRejectsEntityOperations` |
| Per-entity conflicts and merge | `tfoverlay.TestEntityScopedConflicts` |
| Projection equals the hand-written reference overlay applied by Speakeasy when two entities share a component, and is expressible as a generated standard overlay; other entities untouched | `tfoverlay.TestApplyTerraformMatchesTheReferenceProjection` |
| Determinism under every operation order, precedence, inheritance, cycles, discriminators | `tfoverlay.TestApplyTerraformIsDeterministic`, `tfoverlay.TestApplyTerraformPrecedenceAndInheritance`, `tfoverlay.TestApplyTerraformScopedRemoveOfGlobalKey`, `tfoverlay.TestApplyTerraformClonesAWholeCycle` |
| Projection errors: unknown entity, split kinds, unreachable or unresolved destination, collisions | `tfoverlay.TestApplyTerraformProjectionErrors`, `tfoverlay.TestApplyTerraformCollidingCloneIsAnError`, `tfoverlay.TestApplyTerraformReapplicationIsRejected` |
| `Apply` and `Validate` reject scoped operations | `tfoverlay.TestApplyAndValidateRejectScopedOperations` |
| Selectors and property operations: stages, exact-wins precedence, match counts, conflicts, determinism, artifact round trip | `tfoverlay.TestSelectorsAndPropertyOperations`, `tfoverlay.TestSelectorOperationsAreDeterministic`, `tfoverlay.TestSelectorErrors`, `tfoverlay.TestSelectorOverlayConflictsAndValidation`, `tfoverlay.TestSelectorArtifactRoundTrip`, `tfoverlay.TestPropertyOperationsThroughApply` |
| The selector and property actions of a real provider overlay, in apigw form and as rendered, equal the provider overlay applied by Speakeasy to a real spec (needs `TFOVERLAY_REAL_SPEC`) | `tfoverlay.TestClassCActionsMatchSpeakeasy` |
| Selector and property patch declarations and emission | `apigw.TestSelectorDeclarationValidation`, `apigw.TestPropertyPatchDeclarationValidation`, `apigw.TestSelectorAndPropertyAnnotationsEmit` |
| Command line | `cmd/tfoverlay` tests |
| Declaration validation: keys, values, pointers, owner/target legality | `apigw.TestCustomizationDeclarationValidation`, `apigw.TestSchemaPatchDeclarationValidation` |
| Unapplied annotations fail, with a reason | `apigw.TestUnappliedAnnotationsFailGeneration`, `apigw.TestUnappliedAnnotationReportsReason` |
| Every documented destination, and the full emitted operation list | `apigw.TestAnnotationTransportOverlay` |
| Future key, shared scope, determinism | `apigw.TestAnnotationTransportOverlay` subtests |
| Effective values, precedence, OpenAPI validity of the applied document | `apigw.TestOverlayAppliesToTheEmittedDocument` |
| No annotations leaves committed output unchanged | `apigw.TestNoAnnotationsLeaveOutputUnchanged` |
| Field metadata survives references and well-known types | `apigw.TestEmbeddedFieldExtensionsSurvive` |
| The committed descriptor fixture is current | `apigw.TestDescriptorFixtureIsCurrent` |

The generator tests drive the real plugin in process from a committed
descriptor set (`internal/apigw/testdata/example.fds.bin`, refreshed with
`make testdata`), so `go test ./...` needs no protoc or buf.

## What this mechanism is not

- It does not execute anything. Values are data. jq expressions, Go
  expressions, validator functions, custom default implementations and plan
  modifier functions remain the provider's or the generator's
  responsibility; this transport only carries the reference object.
- It does not add `gen.yaml` settings. Generator configuration and environment
  variable mapping are `gen.yaml` inputs, not OpenAPI annotations, and are not
  expressible here.
- It does not make a Speakeasy version support an extension it does not
  support. Transport is guaranteed; execution is not. A fabricated future
  `x-*` key travels proto → generator → overlay unchanged, and the pinned
  generator decides what to do with it.
- It does not interpret or validate the semantics of a vendor extension key
  beyond the `x-` prefix. There is no allowlist, so a new key never requires an
  `apigw` release.

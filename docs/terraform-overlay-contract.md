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
proto annotations ──▶ protoc-gen-apigw ──┬──▶ <pkg>.<service>.oas31.yaml    (shared, unchanged)
                                         └──▶ <pkg>.<service>.terraform_overlay.yaml  (Terraform only)
```

A consumer (C1's spec aggregation, then the provider-generation pipeline)
applies the overlay to the merged OpenAPI document immediately before invoking
the Terraform generator, and not to the document it publishes.

## Proto surface

Status: implemented and covered by tests. The proto surface, the destination
table, the artifact format and the precedence rules below are exercised by the
tests named in each section and by
[docs/terraform-customization-coverage.md](terraform-customization-coverage.md).
The one deliberately unsettled claim is the IGA-4347 entitlement mapping
(see the example below), which the pinned-generator experiment decides.

Five annotation owner kinds, each carrying the same two lists:

| Owner | Option | Added fields |
| --- | --- | --- |
| file | `(apigw.v1.file)` on `google.protobuf.FileOptions` | `customizations`, `schema_patches` |
| service | `(apigw.v1.service).service` | `customizations` (5), `schema_patches` (6) |
| method operation | `(apigw.v1.method).operations[]` | `customizations` (15), `schema_patches` (16) |
| message | `(apigw.v1.message).message_options[]` | `customizations` (6), `schema_patches` (7) |
| field | `(apigw.v1.field).field_options[]` | `customizations` (6), `schema_patches` (7) |

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

message AppEntitlement {
  option (apigw.v1.message).message_options = {
    terraform_entity: {name: "AppEntitlement"}
    customizations: {
      key: "x-speakeasy-name-override"
      value_json: "\"AppEntitlement\""
      scope: OPEN_API_CUSTOMIZATION_SCOPE_TERRAFORM
    }
  };

  string app_id = 1 [(apigw.v1.field).field_options = {
    customizations: {
      key: "x-speakeasy-param-readonly"
      value_json: "true"
    }
  }];

  // An API property whose Terraform attribute name must differ from the JSON
  // property name. `terraform_attribute` below is a placeholder: the contract
  // guarantees that this annotation reaches the overlay at this property's
  // exact pointer. It does not assert that the pinned generator accepts the
  // value, nor which value produces a given Terraform attribute name; that is
  // established by the generator experiment, not by this document.
  string provisioner_policy = 2 [(apigw.v1.field).field_options = {
    customizations: {
      key: "x-speakeasy-name-override"
      value_json: "\"terraform_attribute\""
    }
  }];

  // Standard schema keywords go through the patch channel.
  repeated string tags = 3 [(apigw.v1.field).field_options = {
    schema_patches: {
      patch_json: "{\"default\": []}"
    }
  }];
}
```

The `provision_policy` mapping that IGA-4347 needs is **not** settled by this
document. The transport guarantees the annotation arrives; whether
`x-speakeasy-name-override` on `provisionerPolicy` unifies the create/read/update
projections under the pinned generator is the cycle-2 experiment's result. The
working hypothesis is `provisionPolicy` (the serializer's property name) rather
than `provision_policy`, and it stays a hypothesis until the generated provider
proves it.

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
   and extension and schema channels never collide with each other. Removing a
   key is a write at the same location and overlaps the same way.

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

For each emitted OpenAPI document, `protoc-gen-apigw` emits one companion
overlay beside it:

| OpenAPI artifact | Overlay artifact |
| --- | --- |
| `<dir>/<base>.<service>.oas31.yaml` | `<dir>/<base>.<service>.terraform_overlay.yaml` |
| `<dir>/<base>.<package>.oas31.yaml` | `<dir>/<base>.<package>.terraform_overlay.yaml` |

**This is an `apigw`-defined patch format — "apigw Terraform customization
overlay, version 1" — not [the OpenAPI Overlay
Specification](https://spec.openapis.org/overlay/latest.html).** The two differ
deliberately: the OpenAPI Overlay specification addresses targets with JSONPath
and its `update` action merges recursively, whereas Terraform customization
needs exact, non-merging destinations (`SET` replaces a complete value; `REMOVE`
is explicit). An `apigw` overlay addresses one destination with an RFC 6901
JSON Pointer and performs one set-or-remove of one key there, which is the
semantics the annotations require and the semantics the provider pipeline can
apply deterministically. Do not feed these files to Overlay-spec tooling.

No artifact is emitted for a document that has no operations, so a repository
that has not adopted the annotations sees no new files.

```yaml
version: 1
source: c1.api.app.v1.AppEntitlementService   # descriptor FQN of the document owner
kind: service                                 # service | package
operations:
  - pointer: /components/schemas/c1.api.app.v1.AppEntitlement
    channel: extension
    key: x-speakeasy-entity
    mode: set
    value: AppEntitlement
  - pointer: /components/schemas/c1.api.app.v1.AppEntitlement/properties/provisionerPolicy
    channel: extension
    key: x-speakeasy-param-readonly
    mode: set
    value: false
  - pointer: /components/schemas/c1.api.app.v1.AppEntitlement/properties/tags
    channel: schema
    key: default
    mode: set
    value: []
  - pointer: /paths/~1v1~1apps~1{app_id}~1entitlements/get
    channel: extension
    key: x-speakeasy-entity-missing-codes
    mode: set
    value:
      - 403
      - 410
```

- `pointer` is an RFC 6901 JSON Pointer, absolute in the emitted document.
  `~0`/`~1` escaping is applied by the generator, so a pointer is always
  machine-resolvable and never a hand-maintained component name.
- `channel` is `extension` for `customizations` and `schema` for
  `schema_patches`.
- `mode` is `set` or `remove`. `remove` operations omit `value`.
- `value` is the parsed JSON value with its exact type; `null` serializes as
  `null`, not as an empty key.
- Operations are sorted by `(pointer, channel, key)` and deduplicated, so two
  runs over identical input produce identical bytes.

### Consumer contract

The format is consumed through the importable Go package
`github.com/ductone/protoc-gen-apigw/tfoverlay`, which is the single
implementation of these semantics — a consumer does not re-derive them:

| Function | Contract |
| --- | --- |
| `Parse([]byte) (*Overlay, error)` | Read an emitted artifact; rejects an unknown `version`. |
| `Merge(...*Overlay) (*Overlay, error)` | Combine the per-service overlays of a merged document. Identical operations deduplicate; contradictory ones (same `pointer + channel + key`, different `mode` or `value`) fail with both sources named, as do operations with overlapping destinations. |
| `Apply(doc []byte, ops []Operation) ([]byte, error)` | Apply in order to a rendered OpenAPI document. A pointer that does not resolve, or a `remove` of an absent key, is an error. |
| `Validate(doc []byte, ops []Operation) error` | Resolve every pointer against a document without modifying it. |

`Merge` is the cross-document check the per-file generator cannot perform:
root-level (document root) operations contributed by several services either
agree or are rejected, and are never silently ordered by file name. Pointers are
absolute in the service document and stay valid in the merged document because
`apigw` derives component names and paths itself and the merger concatenates
both `/paths` and `/components/schemas`; two services that contribute the same
pointer and key must therefore agree, which `Merge` enforces.

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
| Canonical ordering, render/parse round trip, version rejection | `tfoverlay.TestOverlayNormalizeIsOrderIndependent`, `tfoverlay.TestOverlayRenderParseRoundTrip` |
| Operation validation (keys, channels, modes, pointers) | `tfoverlay.TestOverlayValidateOperation` |
| Cross-document merge, root conflicts, schema conflicts | `tfoverlay.TestMerge` |
| Apply and validate against a document, order independence | `tfoverlay.TestApply`, `tfoverlay.TestValidate` |
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

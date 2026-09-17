# Terraform customization coverage matrix

Acceptance checklist for the annotation transport defined in
[terraform-overlay-contract.md](terraform-overlay-contract.md). Sources were
fetched from the live Speakeasy documentation on 2026-09-17; the page inventory
is at the end of this document.

This matrix is an implementation checklist, **not** a frozen allowlist. The
generator carries any `x-` key and any schema keyword; the matrix records which
documented items are exercised by a fixture, and classifies the items the
transport deliberately does not carry.

Every `FX-*` region named below is implemented and passing. The fixture protos
are `example/tfcustomize/v1/tfcustomize.proto` (annotations) and
`example/bookstore/v1/bookstore.proto` (the no-annotation control), and the
committed overlay for the former is
`example/tfcustomize/v1/tfcustomize.pb.customize_service.terraform_overlay.yaml`.
`apigw.TestAnnotationTransportOverlay` asserts the complete emitted operation
list, so a documented destination cannot quietly stop being transported.

## How each row is covered

Every row states the *destination class* the documented placement maps to, and
the fixture class that exercises it. Fixture classes are named `FX-*` and are
implemented in `internal/apigw` and `tfoverlay` tests.

| Fixture | What it proves | Implemented by |
| --- | --- | --- |
| `FX-SCHEMA-PROP` | field-level `SCHEMA` destination on scalar, enum, repeated, map and singular-message fields | `TestAnnotationTransportOverlay`; `TestEmbeddedFieldExtensionsSurvive` |
| `FX-PARAM` | field-level `PARAMETER` destination (parameter object, not its schema) | `TestAnnotationTransportOverlay` |
| `FX-ARRAY-ITEMS` | repeated field `ARRAY_ITEMS` destination | `TestAnnotationTransportOverlay` |
| `FX-MAP-VALUES` | map field `MAP_VALUES` destination | `TestAnnotationTransportOverlay` |
| `FX-MESSAGE` | message-level `SCHEMA` destination | `TestAnnotationTransportOverlay` |
| `FX-OP` | operation-level `OPERATION` destination | `TestAnnotationTransportOverlay` |
| `FX-OP-REQ` / `FX-OP-RESP` | operation request / 200-response schema destinations | `TestAnnotationTransportOverlay` (both destinations are asserted); no committed fixture currently declares one, so the assertion is against the destination resolution, not a fixture value |
| `FX-ROOT` | service- and file-level `DOCUMENT_ROOT` destination | `TestAnnotationTransportOverlay` |
| `FX-POINTER` | `JSON_POINTER` escape hatch on every owner kind, including `""` for the root | `TestAnnotationTransportOverlay`; `tfoverlay.TestApply` |
| `FX-PATCH` | schema-keyword set and remove at a destination | `TestAnnotationTransportOverlay`; `TestSchemaPatchDeclarationValidation` |
| `FX-VALUE` | value-shape matrix: string, bool, `false`, integer, decimal, `null`, `""`, `[]`, `{}`, nested object, nested array, escaped pointer tokens, multiline string | `TestAnnotationTransportOverlay`; `TestOverlayAppliesToTheEmittedDocument`; `tfoverlay.TestParseJSONValue` |
| `FX-FUTURE` | a fabricated, previously unused `x-` key with a nested value travels proto → generator → overlay unchanged | `TestAnnotationTransportOverlay` |
| `FX-PRECEDENCE` | generic override replaces a baseline typed annotation at the same key | `TestOverlayAppliesToTheEmittedDocument` |
| `FX-CONFLICT` | equal assignments deduplicate; contradictory assignments fail; ancestor/descendant writes fail **across channels too** | `tfoverlay.TestOverlayAddDeduplicatesAndConflicts`; `tfoverlay.TestOverlayRejectsOverlappingDestinations`; `tfoverlay.TestMergeNamesBothSources` |
| `FX-SCOPE` | `TERRAFORM` scope never touches the `.oas31.yaml`; `SHARED` scope applies there and is absent from the overlay | `TestAnnotationTransportOverlay` subtests |
| `FX-MISSING-TARGET` | unresolved destinations, illegal owner/target pairs, unconsumed annotations, malformed artifacts and non-object destinations all fail | `TestCustomizationDeclarationValidation`; `TestSchemaPatchDeclarationValidation`; `TestUnappliedAnnotationsFailGeneration`; `tfoverlay.TestValidate`; `tfoverlay.TestValidateRequiresObjectDestination`; `tfoverlay.TestParseRejectsMalformedArtifacts` |
| `FX-MERGE` | `Merge` deduplicates and rejects cross-service contradictions | `tfoverlay.TestMerge` |
| `FX-APPLY-VALID` | applying the overlay to the emitted document yields a document that still validates as OpenAPI | `TestOverlayAppliesToTheEmittedDocument` |
| `FX-NOOP` | a document with no annotations produces byte-identical `.oas31.yaml` and no overlay artifact | `TestNoAnnotationsLeaveOutputUnchanged` |
| `FX-DETERMINISM` | repeat generation is byte-identical | `TestAnnotationTransportOverlay`; `TestNoAnnotationsLeaveOutputUnchanged`; `tfoverlay.TestOverlayNormalizeIsOrderIndependent` |

## Extension channel

An `apigw` annotation whose destination is the property schema, a message
schema, a parameter object, an operation, a request/response schema or the
document root can express every placement below. `terraform-only` marks
extensions the docs present as Terraform-generation specific; `shared` marks
extensions the docs also present as general SDK extensions. The transport is
identical for both — the distinction only affects which scope an author should
choose.

### Entity and operation mapping

| key | scope | accepted value shapes | documented placement | destination class | fixture |
| --- | --- | --- | --- | --- | --- |
| `x-speakeasy-entity` | terraform-only | `string`; `array of strings` | schema: `components.schemas.<Name>`, or inline on a request/response/items schema | message `SCHEMA`, or field `SCHEMA`/`ARRAY_ITEMS` | `FX-MESSAGE`, `FX-SCHEMA-PROP` |
| `x-speakeasy-entity-operation` | terraform-only | `"entity#op[#order]"`; `array of strings`; `array of {entityOperation, options:{polling,patch}}`; `object {terraform-datasource: null\|"E#op", terraform-resource: null\|"E#op"}` | operation object | `FX-OP` | `FX-OP`, `FX-VALUE` |
| `x-speakeasy-entity-description` | terraform-only | `string` (Markdown) | entity schema | `FX-MESSAGE` | `FX-VALUE` |
| `x-speakeasy-entity-version` | terraform-only | `integer` | entity schema | `FX-MESSAGE` | `FX-VALUE` |
| `x-speakeasy-entity-missing-codes` | terraform-only | `array of integers` | Read operation object | `FX-OP`; replaces the typed `terraform_entity`-derived value | `FX-PRECEDENCE` |
| `x-speakeasy-pagination` | shared | `object {type: offsetLimit\|cursor\|url, inputs: [...], outputs: {...}}` | operation object | `FX-OP`; replaces the typed pagination value | `FX-PRECEDENCE`, `FX-VALUE` |
| `x-speakeasy-polling` | shared | `array of {name, successCriteria:[{condition,context?,type?}], failureCriteria, delaySeconds, intervalSeconds, limitCount}` | operation object | `FX-OP` | `FX-VALUE` |
| `x-speakeasy-wrapped-attribute` | terraform-only | `string` | response schema; an `allOf` list item; an array schema | `FX-OP-RESP`, field `SCHEMA` | `FX-OP-RESP` |
| `x-speakeasy-soft-delete-property` | terraform-only | `bool` | response property | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-transform-from-api` | shared | `object {jq: string}` | schema (request/response body) | `FX-SCHEMA-PROP`, `FX-OP-REQ`, `FX-OP-RESP` | `FX-OP-REQ` |
| `x-speakeasy-transform-to-api` | shared | `object {jq: string}` | schema (request body) | `FX-OP-REQ` | `FX-OP-REQ` |

### Property customization

| key | scope | accepted value shapes | documented placement | destination class | fixture |
| --- | --- | --- | --- | --- | --- |
| `x-speakeasy-name-override` | shared | property: `string`; global: `array of {operationId, methodNameOverride}` | property schema; parameter; operation; document root | `FX-SCHEMA-PROP`, `FX-PARAM`, `FX-OP`, `FX-ROOT` | `FX-SCHEMA-PROP`, `FX-ROOT` |
| `x-speakeasy-match` | terraform-only | `string` | `parameters[i]` object | `FX-PARAM` | `FX-PARAM` |
| `x-speakeasy-param-sensitive` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-terraform-write-only` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-deprecation-message` | shared | `string` | property; operation; parameter | `FX-SCHEMA-PROP`, `FX-OP`, `FX-PARAM` | `FX-SCHEMA-PROP` |
| `x-speakeasy-terraform-ignore` | terraform-only | `bool`; the literal `schema` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-ignore` | shared | `bool` | property; operation | `FX-SCHEMA-PROP`, `FX-OP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-terraform-custom-type` | terraform-only | `object {imports: [string], schemaType: string, valueType: string}` | property schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-type-override` | shared | `string` (`any`) | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-terraform-custom-default` | terraform-only | `object {imports?: [string], schemaDefinition: string}` | property schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-param-readonly` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-param-optional` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-param-force-new` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-param-computed` | terraform-only | `bool` | property schema (catalogued; no worked example) | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-param-suppress-computed-diff` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-terraform-plan-only` | terraform-only | `bool` | property schema | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-response-filter` | terraform-only | `bool` | response schema property | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-unknown-values` | terraform-only | `string` (`allow`\|`disallow`) | enum schema | `FX-SCHEMA-PROP`; replaces the typed enum default | `FX-PRECEDENCE` |
| `x-speakeasy-terraform-alias-to` | terraform-only | `string` (catalog entry only; shape not stated in the fetched docs) | not documented on any fetched page | any | `FX-VALUE` |

### Validation, plan modification and advanced controls

| key | scope | accepted value shapes | documented placement | destination class | fixture |
| --- | --- | --- | --- | --- | --- |
| `x-speakeasy-conflicts-with` | terraform-only | `string`; `array of strings` (relative refs) | property schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-xor-with` | terraform-only | `array of strings` | property schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-required-with` | terraform-only | `array of strings` | property schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-plan-validators` | terraform-only | `string`; `array of strings` (validator type names) | resource attribute (not the entity root; ignored for data sources) | `FX-SCHEMA-PROP`; the reference object only, code is the provider's | `FX-VALUE` |
| `x-speakeasy-plan-modifiers` | terraform-only | `string`; `array of strings` (modifier type names) | resource attribute (not the entity root; ignored for data sources) | `FX-SCHEMA-PROP`; the reference object only, code is the provider's | `FX-VALUE` |

### Provider configuration, security and globals

| key | scope | accepted value shapes | documented placement | destination class | fixture |
| --- | --- | --- | --- | --- | --- |
| `x-speakeasy-globals` | shared | `object {parameters: [{name, in: path\|query\|header, schema} \| {$ref}]}` | document root | `FX-ROOT` | `FX-ROOT`, `FX-VALUE` |
| `x-speakeasy-globals-hidden` | shared | `bool` | global parameter definition, or the matching operation parameter | `FX-PARAM` | `FX-PARAM` |
| `x-speakeasy-custom-security-scheme` | shared | `object {schema: <JSON Schema, ≥1 property>}` | `components.securitySchemes.<name>` on a `type: http, scheme: custom` scheme | `FX-POINTER` (there is no component-security-scheme owner kind) | `FX-POINTER` |

### General SDK extensions

These are documented for SDK generation generally rather than for Terraform.
The docs note `x-speakeasy-name-override` "also has other SDK customization
capabilities" and that they are generally unnecessary for Terraform providers.
They are carried identically by the transport; an author selecting one in
Terraform scope is responsible for whether the pinned Speakeasy version acts on
it.

| key | accepted value shapes | documented placement | destination class | fixture |
| --- | --- | --- | --- | --- |
| `x-speakeasy-group` | `string` | operation; document root | `FX-OP`, `FX-ROOT` | `FX-OP` |
| `x-speakeasy-model-namespace` | `string` | not detailed on fetched pages | any | `FX-VALUE` |
| `x-speakeasy-include` | `bool` | `components` schema | `FX-MESSAGE` | `FX-MESSAGE` |
| `x-speakeasy-enums` | `map {value: name}`; `array` | enum schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-enum-descriptions` | `array`; `map` | enum schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-enum-format` | `string` (`enum`\|`union`) | enum schema | `FX-SCHEMA-PROP` | `FX-VALUE` |
| `x-speakeasy-retries` | not detailed on fetched pages | global; per-request | `FX-ROOT`, `FX-OP` | `FX-VALUE` |
| `x-speakeasy-usage-example` | not detailed on fetched pages | method | `FX-OP` | `FX-OP` |
| `x-speakeasy-example` | not detailed on fetched pages | security scheme | `FX-POINTER` | `FX-POINTER` |
| `x-speakeasy-docs` | per-language comment map | annotated element | any | `FX-VALUE` |
| `x-speakeasy-errors` | not detailed on fetched pages | `paths`, path item, operation | `FX-POINTER`, `FX-OP` | `FX-POINTER` |
| `x-speakeasy-error-message` | not detailed on fetched pages | field of an error response | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-server-id` | `string` | each `servers[]` entry | `FX-POINTER` | `FX-POINTER` |
| `x-speakeasy-deprecation-replacement` | not detailed on fetched pages | deprecated operation | `FX-OP` | `FX-OP` |
| `x-speakeasy-max-method-params` | `integer` | not detailed on fetched pages | `FX-ROOT` | `FX-VALUE` |
| `x-speakeasy-mcp` | `object {disabled, name, title, scopes, description, destructiveHint, idempotentHint, openWorldHint, readOnlyHint}` | operation | `FX-OP` | `FX-VALUE` |
| `x-speakeasy-overridable-scopes` | not detailed on fetched pages | security config | `FX-POINTER` | `FX-POINTER` |
| `x-speakeasy-param-encoding-override` | `string` (`allowReserved`) | path parameter | `FX-PARAM` | `FX-PARAM` |
| `x-speakeasy-token-endpoint-additional-properties` | not detailed on fetched pages | security / token config | `FX-POINTER` | `FX-POINTER` |
| `x-speakeasy-discriminator` | not detailed on fetched pages | discriminator mapping | `FX-POINTER` | `FX-POINTER` |
| `x-speakeasy-base64-input-mode` | not detailed on fetched pages | JSON string field (`format: byte`, `contentEncoding: base64`); Python only | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-extension-rewrite` | `map {<foreign extension>: <x-speakeasy-extension>}` | document root | `FX-ROOT` | `FX-VALUE` |

### Not documented upstream

| key | status |
| --- | --- |
| `x-speakeasy-terraform-plan-modifier` | **Not documented** on any fetched page. The singular form does not exist in the live docs; `apigw` emits this key today from the typed `annotation_bag` option and the C1 provider consumes it. It is a pre-existing `apigw` extension, not a documented Speakeasy one. A generic override can still replace it at the same destination (`FX-PRECEDENCE`). |
| `x-speakeasy-plan-modifiers` (plural) | The documented key for attribute-level plan modification; it is a custom-code reference and is carried by the extension channel. |

## Schema-keyword channel

These are standard OpenAPI / JSON Schema keywords and travel through
`schema_patches`, never through the extension channel. The transport carries
any of them; `patch_json` keys must not start with `x-`, which is enforced.

| keyword | accepted value shapes | documented placement | fixture |
| --- | --- | --- | --- |
| `default` | typed value matching the schema type | schema property | `FX-PATCH` |
| `example`, `examples` | typed value / array | schema property | `FX-PATCH` |
| `deprecated` | `bool` | property, operation | `FX-PATCH` |
| `required` | `array of strings` | object schema | `FX-PATCH` |
| `nullable` | `bool` | schema property | `FX-PATCH` |
| `readOnly`, `writeOnly` | `bool` | schema property | `FX-PATCH` |
| `const` | typed value | schema property | `FX-PATCH` |
| `enum` | `array` | string/integer schema | `FX-PATCH` |
| `oneOf`, `anyOf`, `allOf` | `array of schemas` | schema | `FX-PATCH` |
| `type` | `string` / `array` | schema | `FX-PATCH` |
| `format` | `string` | schema | `FX-PATCH` |
| `minLength`, `maxLength`, `pattern` | `integer`, `integer`, regex | string schema | `FX-PATCH` |
| `minimum`, `maximum` | `number` | integer schema | `FX-PATCH` |
| `minItems`, `maxItems`, `uniqueItems` | `integer`, `integer`, `bool` | array schema | `FX-PATCH` |
| `additionalProperties` | `bool` / schema | object schema | field `MAP_VALUES`, or `FX-PATCH` |
| `contentMediaType` | `application/json` | property | `FX-PATCH` |
| `security`, `securitySchemes` | OpenAPI security objects | root / `components.securitySchemes` | `FX-POINTER` |
| `servers` | `array of {url, variables}` | document root | `FX-ROOT` |

Removing a keyword an author added by mistake, or that a typed annotation
generated, is `remove_keys` / a `remove` operation (`FX-PATCH`). Because they
are standard document content rather than vendor extensions, patches are
validated by applying the overlay and re-checking the resulting document
(`FX-APPLY-VALID`).

## Deliberately not carried

These are documented customizations the transport does not and must not
express, because they are generator configuration or hand-written code rather
than OpenAPI annotations.

### gen.yaml settings

Configured in `gen.yaml` under `terraform:`, never as an OpenAPI annotation.
The transport provides no channel for them and this is intentional:
mislabeling a generator setting as an annotation is the failure mode the
contract forbids.

`version`, `packageName`, `author`, `providerTypeNameOverride`,
`environmentVariables`, `additionalProviderAttributes.httpHeaders`,
`additionalProviderAttributes.tlsSkipVerify`, `additionalResources`,
`additionalDataSources`, `additionalEphemeralResources`,
`additionalListResources`, `additionalActions`, `additionalFunctions`,
`additionalDependencies`, `forwardCompatibleEnumsByDefault`,
`enableTypeDeduplication`, and `generation.baseServerUrl`.

### Custom code

The extension carries the *reference object*; the Go code itself is the
provider's and is never generated or executed by the transport.

| key | reference object | code location |
| --- | --- | --- |
| `x-speakeasy-terraform-custom-default` | `{imports?, schemaDefinition}` | any location implementing `resource/schema/defaults` |
| `x-speakeasy-plan-validators` | `string` \| `[]string` | `internal/validators/<type>validators/` |
| `x-speakeasy-plan-modifiers` | `string` \| `[]string` | `internal/planmodifiers/<type>planmodifier/` |
| `x-speakeasy-terraform-custom-type` | `{imports, schemaType, valueType}` | an existing terraform-plugin-framework custom type |
| `x-speakeasy-entity-version` | `integer` | `internal/stateupgraders/` |

`.genignore` and monkey-patching are hand-maintained files, not annotations.

## Recipe placements

Non-obvious placements the transport must be able to reach, each with the
destination class that reaches it.

| recipe | destination class | fixture |
| --- | --- | --- |
| `x-speakeasy-terraform-ignore: schema` (differs from `true`) | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-match` on a `parameters[i]` object rather than the schema | `FX-PARAM` | `FX-PARAM` |
| `x-speakeasy-ignore: true` on one body only, to break drift detection | property `SCHEMA` of that message | `FX-SCHEMA-PROP` |
| entity-operation array mixing a bare string and an `{entityOperation, options:{polling}}` object | `FX-OP` | `FX-VALUE` |
| `options: {patch: {style: only-send-changed-attributes}}` inside a create/update entry | `FX-OP` | `FX-VALUE` |
| entity-operation object form with `terraform-datasource: null` / `terraform-resource: null` | `FX-OP`; `null` must survive as a value | `FX-VALUE` |
| `x-speakeasy-wrapped-attribute` on the second `allOf` element | `FX-OP-RESP` plus `FX-PATCH` for the `allOf` array | `FX-OP-RESP` |
| `x-speakeasy-param-readonly: true` on a create+update identifier | `FX-SCHEMA-PROP` | `FX-SCHEMA-PROP` |
| `x-speakeasy-pagination` beside `x-speakeasy-entity-operation` on one operation | `FX-OP` with two keys | `FX-OP` |
| `x-speakeasy-response-filter` hoisted from array items to the data-source root | `FX-SCHEMA-PROP`, `FX-ARRAY-ITEMS` | `FX-ARRAY-ITEMS` |

## Page inventory

| URL | Outcome |
| --- | --- |
| https://www.speakeasy.com/docs/terraform/customize-terraform | 200 |
| https://www.speakeasy.com/docs/terraform/customize-terraform/entity-mapping | 200 |
| .../provider-configuration | 200 |
| .../resource-configuration | 200 |
| .../property-customization | 200 |
| .../validation-dependencies | 200 |
| .../plan-modification | 200 |
| .../advanced-features | 200 |
| .../schema-keywords | 200 |
| .../common-troubleshooting | 200 |
| https://www.speakeasy.com/docs/speakeasy-reference/generation/terraform-config | 200 |
| https://www.speakeasy.com/docs/speakeasy-reference/extensions | 200 |
| https://www.speakeasy.com/docs/speakeasy-reference/supported/terraform | 200 |
| https://www.speakeasy.com/docs/customize/runtime/pagination | 200 (redirect) |
| https://www.speakeasy.com/docs/customize/runtime/polling | 200 (redirect) |
| https://www.speakeasy.com/docs/terraform/terraform-guides/async-polling | 200 |
| https://www.speakeasy.com/docs/sdks/customize/data-transforms | 200 |
| https://www.speakeasy.com/docs/customize/globals | 200 (redirect) |
| https://www.speakeasy.com/docs/customize-sdks/methods | 200 (redirect) |
| https://www.speakeasy.com/docs/customize/authentication/custom-security-schemes | 200 (redirect) |
| https://www.speakeasy.com/docs/customize/code/sdk-hooks | 200 (redirect) |
| https://www.speakeasy.com/docs/customize/code/monkey-patching | 200 (redirects to the genignore page) |
| https://www.speakeasy.com/docs/terraform/customize-terraform.md | 200 (markdown alternate) |
| https://www.speakeasy.com/docs/terraform/customize-terraform/customize-terraform | 404 |

## Unresolved and out of scope

- `x-speakeasy-terraform-alias-to` has a catalog description ("remap API
  response data to another property") but no documented placement or value
  shape on any fetched page. The transport carries it wherever an author
  chooses; no semantic claim is made.
- `x-speakeasy-param-computed` is catalogued but has no worked example.
- Several general SDK keys are described without a value shape or placement on
  the fetched pages; their rows say so.
- The probe of the *pinned* generator's behaviour for any individual key is
  provider/pipeline work, not transport work. Transport coverage in this matrix
  does not assert that the pinned Speakeasy version executes a key.
- `x-speakeasy-pagination` and `x-speakeasy-polling` are two separate keys with
  two separate wirings: `x-speakeasy-polling` is what `options.polling` in an
  entity-operation entry refers to, while `x-speakeasy-pagination` is its own
  extension. `apigw` already emits a typed `x-speakeasy-pagination` from proto
  pagination options, so a generic `x-speakeasy-pagination` replaces that value
  at the same destination (`FX-PRECEDENCE`); the fixture carries the generic
  form on `ListThings`. The fixture's `x-speakeasy-polling` is a plain
  operation-level array in the documented shape; it is transported, not
  executed.
- The tfcustomize fixture is deliberately not a realistic API description. It
  exists to place one annotation at every documented destination, and its
  values assert transport, not Speakeasy behavior. `TestAnnotationTransportOverlay`
  asserts the full emitted operation list, so a change in what a documented
  destination receives is a deliberate edit to that list.

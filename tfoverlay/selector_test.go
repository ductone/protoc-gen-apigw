package tfoverlay

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi/overlay"
	"gopkg.in/yaml.v3"
)

const selectorSpec = `
openapi: 3.1.0
components:
  schemas:
    A:
      type: object
      additionalProperties: true
      properties:
        deletedAt:
          type: string
        '@type':
          type: string
        config:
          type: object
          additionalProperties: true
          properties:
            '@type':
              type: string
            keep:
              type: string
    B:
      type: object
      additionalProperties: false
      properties:
        deletedAt:
          type: string
        '@type':
          type: string
        stringSliceField:
          type: array
`

func sel(t *testing.T, mode Mode, match Selector, channel Channel, key, valueJSON string) SelectorOperation {
	t.Helper()
	op := SelectorOperation{Selector: match, Mode: mode, Channel: channel, Key: key}
	if valueJSON != "" {
		v, err := ParseJSONValue(valueJSON)
		if err != nil {
			t.Fatal(err)
		}
		op.Value = v
	}
	return op
}

func whereTrue(t *testing.T) Selector {
	t.Helper()
	return Selector{Path: "$..[?(@.additionalProperties == true)]"}
}

func sevenActions(t *testing.T) []SelectorOperation {
	return []SelectorOperation{
		sel(t, ModeSet, whereTrue(t), ChannelSchema, "additionalProperties", "false"),
		sel(t, ModeRemove, Selector{Path: "$..['@type']"}, "", "", ""),
		sel(t, ModeSet, Selector{Path: "$..deletedAt"}, ChannelExtension, "x-speakeasy-soft-delete-property", "true"),
	}
}

func propOp(t *testing.T, pointer, name string, mode Mode, schemaJSON string) Operation {
	t.Helper()
	op := Operation{Pointer: pointer, Channel: ChannelProperty, Key: name, Mode: mode}
	if schemaJSON != "" {
		v, err := ParseJSONValue(schemaJSON)
		if err != nil {
			t.Fatal(err)
		}
		op.Value = v
	}
	return op
}

func overlayOf(t *testing.T, sels []SelectorOperation, ops []Operation) *Overlay {
	t.Helper()
	ov := &Overlay{Source: "test", Kind: KindPackage}
	for _, s := range sels {
		if err := ov.AddSelector(s); err != nil {
			t.Fatal(err)
		}
	}
	for _, op := range ops {
		if err := ov.Add(op); err != nil {
			t.Fatal(err)
		}
	}
	return ov
}

func TestSelectorsAndPropertyOperations(t *testing.T) {
	ov := overlayOf(t, sevenActions(t), []Operation{
		// Exact operations see the selectors' result: the exception on A.config
		// replaces the selector's write, and the removed @type is added back.
		{Pointer: "/components/schemas/A/properties/config", Channel: ChannelSchema, Key: "additionalProperties", Mode: ModeSet, Value: mustJSON(t, "true")},
		propOp(t, "/components/schemas/A/properties/config/properties", "@type", ModeSet, `{"type":"string","description":"d"}`),
		propOp(t, "/components/schemas/A/properties", "added", ModeSet, `{"type":"integer"}`),
		propOp(t, "/components/schemas/B/properties", "stringSliceField", ModeRemove, ""),
	})
	out, report, err := ApplyOverlay([]byte(selectorSpec), ov)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	var want map[string]interface{}
	if err := yaml.Unmarshal([]byte(`
openapi: 3.1.0
components:
  schemas:
    A:
      type: object
      additionalProperties: false
      properties:
        deletedAt:
          type: string
          x-speakeasy-soft-delete-property: true
        config:
          type: object
          additionalProperties: true
          properties:
            keep:
              type: string
            '@type':
              type: string
              description: d
        added:
          type: integer
    B:
      type: object
      additionalProperties: false
      properties:
        deletedAt:
          type: string
          x-speakeasy-soft-delete-property: true
`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("result differs\nwant %v\n got %v", want, got)
	}

	byName := map[string]SelectorReport{}
	for _, r := range report.Selectors {
		byName[r.Selector] = r
	}
	if r := byName["$..['@type']"]; r.Matches != 3 || r.Overridden != 1 {
		t.Errorf("@type report = %+v, want 3 matches, 1 overridden", r)
	}
	if r := byName["$..[?(@.additionalProperties == true)]"]; r.Matches != 2 || r.Overridden != 1 {
		t.Errorf("additionalProperties report = %+v, want 2 matches, 1 overridden", r)
	}
	if r := byName["$..deletedAt"]; r.Matches != 2 || r.Overridden != 0 {
		t.Errorf("deletedAt report = %+v", r)
	}
}

func mustJSON(t *testing.T, s string) *yaml.Node {
	t.Helper()
	v, err := ParseJSONValue(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSelectorOperationsAreDeterministic(t *testing.T) {
	ops := []Operation{
		propOp(t, "/components/schemas/A/properties", "added", ModeSet, `{"type":"integer"}`),
		propOp(t, "/components/schemas/B/properties", "stringSliceField", ModeRemove, ""),
	}
	sels := sevenActions(t)
	first, _, err := ApplyOverlay([]byte(selectorSpec), overlayOf(t, sels, ops))
	if err != nil {
		t.Fatal(err)
	}
	permute(len(sels), func(order []int) {
		shuffled := make([]SelectorOperation, 0, len(sels))
		for _, i := range order {
			shuffled = append(shuffled, sels[i])
		}
		got, _, err := ApplyOverlay([]byte(selectorSpec), overlayOf(t, shuffled, ops))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("selector order %v changed the output", order)
		}
	})
}

func TestSelectorErrors(t *testing.T) {
	apply := func(sels []SelectorOperation, ops ...Operation) error {
		_, _, err := ApplyOverlay([]byte(selectorSpec), overlayOf(t, sels, ops))
		return err
	}
	t.Run("a selector that matches nothing", func(t *testing.T) {
		err := apply([]SelectorOperation{sel(t, ModeSet, Selector{Path: "$..nothingHere"}, ChannelExtension, "x-a", "1")})
		if err == nil || !strings.Contains(err.Error(), "matches no node") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("an exact remove of a key a selector removed", func(t *testing.T) {
		err := apply(sevenActions(t), propOp(t, "/components/schemas/B/properties", "@type", ModeRemove, ""))
		if err == nil || !strings.Contains(err.Error(), "not present") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("adding a property that exists", func(t *testing.T) {
		err := apply(nil, propOp(t, "/components/schemas/A/properties", "deletedAt", ModeSet, `{"type":"string"}`))
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("removing a property that is absent", func(t *testing.T) {
		err := apply(nil, propOp(t, "/components/schemas/A/properties", "ghost", ModeRemove, ""))
		if err == nil || !strings.Contains(err.Error(), "not present") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("a selector removing a node another selector writes", func(t *testing.T) {
		err := apply([]SelectorOperation{
			sel(t, ModeRemove, Selector{Path: "$..config"}, "", "", ""),
			sel(t, ModeSet, whereTrue(t), ChannelSchema, "additionalProperties", "false"),
		})
		if err == nil || !strings.Contains(err.Error(), "removes") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("two selectors writing one node differently", func(t *testing.T) {
		err := apply([]SelectorOperation{
			sel(t, ModeSet, Selector{Path: "$..deletedAt"}, ChannelExtension, "x-a", "1"),
			sel(t, ModeSet, Selector{Path: "$..[?(@.type == 'string')]"}, ChannelExtension, "x-a", "2"),
		})
		if err == nil || !strings.Contains(err.Error(), "disagree") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("a property add and an exact write inside it overlap", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(propOp(t, "/components/schemas/A/properties", "p", ModeSet, `{"type":"string"}`)); err != nil {
			t.Fatal(err)
		}
		inner := Operation{Pointer: "/components/schemas/A/properties/p", Channel: ChannelExtension, Key: "x-a", Mode: ModeSet, Value: mustJSON(t, "1")}
		if err := ov.Add(inner); err == nil || !strings.Contains(err.Error(), "overlapping") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestSelectorOverlayConflictsAndValidation(t *testing.T) {
	ov := &Overlay{}
	a := sel(t, ModeSet, Selector{Path: "$..deletedAt"}, ChannelExtension, "x-a", "1")
	if err := ov.AddSelector(a); err != nil {
		t.Fatal(err)
	}
	if err := ov.AddSelector(a); err != nil || len(ov.Selectors) != 1 {
		t.Fatalf("identical selector must deduplicate: %v (%d)", err, len(ov.Selectors))
	}
	if err := ov.AddSelector(sel(t, ModeSet, Selector{Path: "$..deletedAt"}, ChannelExtension, "x-a", "2")); err == nil {
		t.Fatal("a differing value for one selector and key must conflict")
	}
	if err := ov.AddSelector(sel(t, ModeSet, Selector{Path: "$..deletedAt"}, ChannelExtension, "x-b", "2")); err != nil {
		t.Fatalf("a different key is a different assignment: %v", err)
	}
	if err := ov.AddSelector(sel(t, ModeRemove, Selector{Path: "$..deletedAt"}, "", "", "")); err == nil {
		t.Fatal("set and remove of one selector must conflict")
	}

	invalid := map[string]SelectorOperation{
		"empty selector":       sel(t, ModeSet, Selector{}, ChannelExtension, "x-a", "1"),
		"not a JSONPath":       sel(t, ModeSet, Selector{Path: "deletedAt"}, ChannelExtension, "x-a", "1"),
		"malformed filter":     sel(t, ModeSet, Selector{Path: "$..[?(@.a ==]"}, ChannelExtension, "x-a", "1"),
		"non-scalar value":     sel(t, ModeSet, Selector{Path: "$..k"}, ChannelExtension, "x-a", "[1]"),
		"extension key not x-": sel(t, ModeSet, Selector{Path: "$..k"}, ChannelExtension, "nope", "1"),
		"schema key is x-":     sel(t, ModeSet, Selector{Path: "$..k"}, ChannelSchema, "x-a", "1"),
		"property channel":     sel(t, ModeSet, Selector{Path: "$..k"}, ChannelProperty, "a", "1"),
		"set without value":    {Selector: Selector{Path: "$..k"}, Mode: ModeSet, Channel: ChannelExtension, Key: "x-a"},
		"remove with a key":    sel(t, ModeRemove, Selector{Path: "$..k"}, "", "x-a", ""),
	}
	for name, s := range invalid {
		if err := (&Overlay{}).AddSelector(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPropertyOperationsThroughApply(t *testing.T) {
	spec := []byte(selectorSpec)
	ops := []Operation{propOp(t, "/components/schemas/A/properties", "added", ModeSet, `{"type":"integer"}`)}
	if err := Validate(spec, ops); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(spec, ops); err != nil {
		t.Fatal(err)
	}
	bad := propOp(t, "/components/schemas/A/properties", "p", ModeSet, `"scalar"`)
	if err := (&Overlay{}).Add(bad); err == nil {
		t.Fatal("a property schema must be an object")
	}
}

func TestSelectorArtifactRoundTrip(t *testing.T) {
	ov := overlayOf(t, sevenActions(t), []Operation{
		propOp(t, "/components/schemas/A/properties", "added", ModeSet, `{"type":"integer"}`),
		propOp(t, "/components/schemas/B/properties", "stringSliceField", ModeRemove, ""),
	})
	ov.Source = "s"
	rendered, err := ov.Render([]byte(selectorSpec))
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered.Global)
	for _, want := range []string{
		"overlay: 1.1.0\n",
		"- target: $..deletedAt\n",
		"x-apigw-selector: true",
		"x-apigw-property: true",
		"- target: $['components']['schemas']['B']['properties']['stringSliceField']\n    remove: true",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered artifact lacks %q:\n%s", want, text)
		}
	}
	if rendered.Entities != nil {
		t.Fatalf("no entity part expected:\n%s", rendered.Entities)
	}
	parsed, err := Parse(rendered.Global)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, text)
	}
	again, err := parsed.Render([]byte(selectorSpec))
	if err != nil || !bytes.Equal(rendered.Global, again.Global) {
		t.Fatalf("round trip differs (%v):\n%s\n---\n%s", err, text, again.Global)
	}
	if err := VerifyStandard([]byte(selectorSpec), ov); err != nil {
		t.Fatalf("standard application differs: %v", err)
	}

	for name, mutate := range map[string]func(string) string{
		"unknown action extension": func(s string) string {
			return strings.Replace(s, "x-apigw-selector: true", "x-apigw-selector: true\n    x-other: 1", 1)
		},
		"selector marker false": func(s string) string {
			return strings.Replace(s, "x-apigw-selector: true", "x-apigw-selector: false", 1)
		},
		"selector with property marker": func(s string) string {
			return strings.Replace(s, "x-apigw-selector: true", "x-apigw-selector: true\n    x-apigw-property: true", 1)
		},
	} {
		if _, err := Parse([]byte(mutate(text))); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	t.Run("merge carries selectors and rejects a conflict", func(t *testing.T) {
		a := overlayOf(t, sevenActions(t), nil)
		b := overlayOf(t, sevenActions(t), nil)
		merged, err := Merge(a, b)
		if err != nil || len(merged.Selectors) != 3 {
			t.Fatalf("merge = %v, %v", merged, err)
		}
		c := overlayOf(t, []SelectorOperation{sel(t, ModeSet, Selector{Path: "$..deletedAt"}, ChannelExtension, "x-speakeasy-soft-delete-property", "false")}, nil)
		if _, err := Merge(a, c); err == nil {
			t.Fatal("merge accepted a conflicting selector")
		}
	})
}

// classCOverlay is the apigw form of the provider overlay actions in
// testdata/selectors/<name>.openapi-overlay.yaml: three selectors and four
// property changes, plus for "nine" two exact exceptions to a selector.
func classCOverlay(t *testing.T, name string) *Overlay {
	t.Helper()
	const (
		connectorConfig = "/components/schemas/c1.api.app.v1.Connector/properties/config"
		createConfig    = "/components/schemas/c1.api.app.v1.ConnectorServiceCreateRequestInput/properties/config"
	)
	typeSchema := `{"description":"The type of the serialized message.","type":"string"}`
	ops := []Operation{
		propOp(t, connectorConfig+"/properties", "@type", ModeSet, typeSchema),
		propOp(t, createConfig+"/properties", "@type", ModeSet, typeSchema),
		propOp(t, "/components/schemas/c1.api.automations.v1.Automation/properties", "webhookHmacSecret", ModeSet,
			`{"description":"If we create a new trigger with an HMAC secret we return the HMAC on this field","readOnly":false,"type":"string"}`),
		propOp(t, "/components/schemas/c1.api.form.v1.Field/properties", "stringSliceField", ModeRemove, ""),
	}
	if name == "nine" {
		ops = append(ops,
			Operation{Pointer: connectorConfig, Channel: ChannelSchema, Key: "additionalProperties", Mode: ModeSet, Value: mustJSON(t, "true")},
			Operation{Pointer: createConfig, Channel: ChannelSchema, Key: "additionalProperties", Mode: ModeSet, Value: mustJSON(t, "true")},
		)
	}
	return overlayOf(t, sevenActions(t), ops)
}

// TestClassCActionsMatchSpeakeasy applies the provider's class (c) actions to a
// real spec two ways — the original provider overlay through the Speakeasy
// library, and the apigw form through ApplyOverlay — and requires the same
// document. It also requires the apigw form, rendered as a standard Overlay, to
// give that document through the Speakeasy library. The spec is too large to
// commit, so the test runs only when TFOVERLAY_REAL_SPEC is set (for example to
// C1's pkg/registry_api/c1oas/openapi.yaml).
func TestClassCActionsMatchSpeakeasy(t *testing.T) {
	specPath := os.Getenv("TFOVERLAY_REAL_SPEC")
	if specPath == "" {
		t.Skip("set TFOVERLAY_REAL_SPEC to compare against the Speakeasy Overlay implementation")
	}
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"seven", "nine"} {
		t.Run(name, func(t *testing.T) {
			reference, err := overlay.Parse(filepath.Join("testdata", "selectors", name+".openapi-overlay.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			want := &yaml.Node{}
			if err := yaml.Unmarshal(spec, want); err != nil {
				t.Fatal(err)
			}
			if _, err := reference.ApplyToStrict(want); err != nil {
				t.Fatal(err)
			}
			ov := classCOverlay(t, name)
			got, _, err := ApplyOverlay(spec, ov)
			if err != nil {
				t.Fatal(err)
			}
			var a, b interface{}
			if err := want.Decode(&a); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(got, &b); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a, b) {
				t.Fatal("ApplyOverlay differs from the provider overlay applied by Speakeasy")
			}
			if err := VerifyStandard(spec, ov); err != nil {
				t.Fatal(err)
			}
		})
	}
}

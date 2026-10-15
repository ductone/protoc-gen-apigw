package tfoverlay

import (
	"strings"
	"testing"
)

const standardDocument = `
openapi: 3.1.0
paths:
  /things/{id}:
    get:
      operationId: GetThing
      x-speakeasy-entity-missing-codes: [404]
      parameters:
        - name: id
          in: path
          schema:
            type: string
components:
  schemas:
    Thing:
      type: object
      title: A Thing
      x-flag: "true"
      x-config:
        a: 1
        b: 2
      properties:
        it's:
          type: string
`

// TestRenderMatchesStandardApplication renders operations whose plain Overlay
// update would merge or keep the old type, and requires the Speakeasy Overlay
// implementation to produce exactly what Apply produces.
func TestRenderMatchesStandardApplication(t *testing.T) {
	get := "/paths/~1things~1{id}/get"
	ops := []Operation{
		// An array replaces the existing array instead of concatenating.
		op(t, get, "x-speakeasy-entity-missing-codes", ModeSet, `[403,410]`),
		// An object replaces the existing object instead of merging into it.
		op(t, "/components/schemas/Thing", "x-config", ModeSet, `{"c":3}`),
		// A boolean replaces a string instead of keeping the string tag.
		op(t, "/components/schemas/Thing", "x-flag", ModeSet, `true`),
		// A scalar of the same type is a plain update.
		schemaOp(t, "/components/schemas/Thing", "title", ModeSet, `"Renamed"`),
		// A new key, an array index in the pointer and a quote in a name.
		op(t, get+"/parameters/0", "x-speakeasy-param-readonly", ModeSet, `true`),
		op(t, "/components/schemas/Thing/properties/it's", "x-null", ModeSet, `null`),
		op(t, "", "x-root", ModeSet, `{"k":[]}`),
		schemaOp(t, "/components/schemas/Thing", "type", ModeRemove, ""),
	}
	ov := &Overlay{Source: "svc", Kind: KindService}
	for _, o := range ops {
		if err := ov.Add(o); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyStandard([]byte(standardDocument), ov); err != nil {
		t.Fatal(err)
	}

	rendered, err := ov.Render([]byte(standardDocument))
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered.Global)
	if got := strings.Count(text, "x-apigw-replace: true"); got != 3 {
		t.Errorf("replace pairs = %d, want 3 (array, object, type change):\n%s", got, text)
	}
	for _, want := range []string{
		"$['paths']['/things/{id}']['get']['parameters'][0]",
		`$['components']['schemas']['Thing']['properties']['it\'s']`,
		"- target: $\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered artifact lacks %q:\n%s", want, text)
		}
	}

	parsed, err := Parse(rendered.Global)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, text)
	}
	if len(parsed.Operations) != len(ops) {
		t.Fatalf("parsed %d operations, want %d", len(parsed.Operations), len(ops))
	}
	again, err := parsed.Render([]byte(standardDocument))
	if err != nil || string(again.Global) != text {
		t.Fatalf("round trip differs (%v):\n%s\n---\n%s", err, text, again.Global)
	}
}

func TestNormalizedPathRoundTrip(t *testing.T) {
	for _, tokens := range [][]string{
		nil,
		{"a"},
		{"a", "0", "b"},
		{"it's", `back\slash`, "~tilde/slash", "\x01"},
	} {
		path := "$"
		for _, token := range tokens {
			path += "[" + quoteMember(token) + "]"
		}
		got, err := pathTokens(path)
		if err != nil {
			t.Fatalf("pathTokens(%s): %v", path, err)
		}
		if strings.Join(got, "\x00") != strings.Join(tokens, "\x00") {
			t.Fatalf("pathTokens(%s) = %q, want %q", path, got, tokens)
		}
	}
	for _, bad := range []string{"", "a", "$.a", "$..a", "$['a'", "$[a]", "$['a']x", "$[?(@.a)]"} {
		if _, err := pathTokens(bad); err == nil {
			t.Errorf("pathTokens(%q) accepted a path that is not normalized", bad)
		}
	}
}

func TestParseRejectsShapesRenderDoesNotProduce(t *testing.T) {
	ov := &Overlay{Source: "svc", Kind: KindService}
	if err := ov.Add(op(t, "/components/schemas/Thing", "x-config", ModeSet, `{"c":3}`)); err != nil {
		t.Fatal(err)
	}
	rendered, err := ov.Render([]byte(standardDocument))
	if err != nil {
		t.Fatal(err)
	}
	valid := string(rendered.Global)
	if _, err := Parse([]byte(valid)); err != nil {
		t.Fatalf("Parse(valid): %v", err)
	}
	for name, mutate := range map[string]func(string) string{
		"overlay 1.0.0":          func(s string) string { return strings.Replace(s, "overlay: 1.1.0", "overlay: 1.0.0", 1) },
		"legacy JSONPath":        func(s string) string { return s + "x-speakeasy-jsonpath: legacy\n" },
		"unknown metadata field": func(s string) string { return strings.Replace(s, "part: global", "part: global\n  other: x", 1) },
		"unknown part":           func(s string) string { return strings.Replace(s, "part: global", "part: both", 1) },
		"unknown kind":           func(s string) string { return strings.Replace(s, "kind: service", "kind: table", 1) },
		"copy action": func(s string) string {
			return strings.Replace(s, "actions:\n", "actions:\n  - target: $['a']\n    copy: $['b']\n", 1)
		},
		"two keys in one update": func(s string) string {
			return strings.Replace(s, "      x-config:\n", "      x-other: 1\n      x-config:\n", 1)
		},
		"descendant target on an exact action": func(s string) string {
			return strings.Replace(s, "- target: $['components']['schemas']['Thing']\n", "- target: $..Thing\n", 1)
		},
		"replace marker on an update": func(s string) string {
			return strings.Replace(s, "      c: 3\n", "      c: 3\n    x-apigw-replace: true\n", 1)
		},
		"duplicate key": func(s string) string { return strings.Replace(s, "      c: 3\n", "      c: 3\n      c: 3\n", 1) },
		"remove of the root": func(s string) string {
			return strings.Replace(s, "actions:\n", "actions:\n  - target: $\n    remove: true\n", 1)
		},
	} {
		if _, err := Parse([]byte(mutate(valid))); err == nil {
			t.Errorf("%s: Parse accepted the artifact", name)
		}
	}
}

func TestVerifyStandardRejectsEntityOperations(t *testing.T) {
	ov := &Overlay{}
	scoped := op(t, "/components/schemas/Thing", "x-a", ModeSet, `1`)
	scoped.Entity = &EntityScope{Name: "Thing"}
	if err := ov.Add(scoped); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStandard([]byte(standardDocument), ov); err == nil {
		t.Fatal("VerifyStandard accepted an entity-scoped operation")
	}
	if got := ov.Global(); len(got.Operations) != 0 {
		t.Fatalf("Global kept %d entity-scoped operations", len(got.Operations))
	}
}

package tfoverlay

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func node(t *testing.T, jsonValue string) *yaml.Node {
	t.Helper()
	n, err := ParseJSONValue(jsonValue)
	if err != nil {
		t.Fatalf("ParseJSONValue(%q) unexpected error: %v", jsonValue, err)
	}
	return n
}

func op(t *testing.T, pointer, key string, mode Mode, jsonValue string) Operation {
	t.Helper()
	return channelOp(t, ChannelExtension, pointer, key, mode, jsonValue)
}

func schemaOp(t *testing.T, pointer, key string, mode Mode, jsonValue string) Operation {
	t.Helper()
	return channelOp(t, ChannelSchema, pointer, key, mode, jsonValue)
}

func channelOp(t *testing.T, channel Channel, pointer, key string, mode Mode, jsonValue string) Operation {
	t.Helper()
	o := Operation{Pointer: pointer, Channel: channel, Key: key, Mode: mode}
	if mode == ModeSet {
		o.Value = node(t, jsonValue)
	}
	return o
}

func TestParseJSONValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string // canonical yaml rendering
		wantErr string
	}{
		{name: "string", input: `"hello"`, want: "hello"},
		{name: "empty string", input: `""`, want: `""`},
		{name: "true", input: `true`, want: "true"},
		{name: "explicit false", input: `false`, want: "false"},
		{name: "integer", input: `7`, want: "7"},
		{
			// A float64 round trip would turn this into 9007199254740992.
			name:  "integer beyond float64 precision",
			input: `9007199254740993`,
			want:  "9007199254740993",
		},
		{name: "decimal keeps its literal scale", input: `1.50`, want: "1.50"},
		{name: "negative", input: `-2.5`, want: "-2.5"},
		{name: "null is a value", input: `null`, want: "null"},
		{name: "empty array", input: `[]`, want: "[]"},
		{name: "empty object", input: `{}`, want: "{}"},
		{
			// A YAML block scalar is how the artifact renders a string with a
			// newline; the value is unchanged.
			name:  "multiline string",
			input: `"a\nb"`,
			want:  "|-\n    a\n    b",
		},
		{
			name:  "nested value keeps every type",
			input: `{"a":[1,2.5,null,false,"x"],"b":{"c":{}}}`,
			want:  "a:\n    - 1\n    - 2.5\n    - null\n    - false\n    - x\nb:\n    c: {}",
		},
		{name: "object keys are ordered", input: `{"b":1,"a":2}`, want: "a: 2\nb: 1"},
		{name: "string that looks like a number stays a string", input: `"1.50"`, want: `"1.50"`},
		{name: "string that looks like null stays a string", input: `"null"`, want: `"null"`},
		{name: "missing value", input: ``, wantErr: "missing JSON value"},
		{name: "malformed", input: `{`, wantErr: "parsing JSON object"},
		{name: "trailing content", input: `{} {}`, wantErr: "unexpected trailing content"},
		{name: "trailing garbage", input: `1 x`, wantErr: "unexpected trailing content"},
		{name: "duplicate top-level key", input: `{"a":1,"a":2}`, wantErr: `duplicate object key "a"`},
		{
			name:    "duplicate nested key",
			input:   `{"a":{"b":1,"b":2}}`,
			wantErr: `duplicate object key "b"`,
		},
		{name: "not JSON", input: `yes`, wantErr: "parsing JSON value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJSONValue(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseJSONValue(%q) = %v, want error containing %q", tt.input, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseJSONValue(%q) error = %v, want it to contain %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseJSONValue(%q) unexpected error: %v", tt.input, err)
			}
			if got := renderNode(t, got); got != tt.want {
				t.Fatalf("ParseJSONValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func renderNode(t *testing.T, n *yaml.Node) string {
	t.Helper()
	data, err := yaml.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return strings.TrimSpace(string(data))
}

func TestPointerRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		pointer string
	}{
		{name: "empty", tokens: nil, pointer: ""},
		{name: "simple", tokens: []string{"a", "b"}, pointer: "/a/b"},
		{name: "escapes slash", tokens: []string{"odd/name"}, pointer: "/odd~1name"},
		{name: "escapes tilde", tokens: []string{"c~d"}, pointer: "/c~0d"},
		{name: "escapes both", tokens: []string{"a~/b"}, pointer: "/a~0~1b"},
		{name: "empty token", tokens: []string{""}, pointer: "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinPointer(tt.tokens...); got != tt.pointer {
				t.Fatalf("JoinPointer(%q) = %q, want %q", tt.tokens, got, tt.pointer)
			}
			got, err := ParsePointer(tt.pointer)
			if err != nil {
				t.Fatalf("ParsePointer(%q) unexpected error: %v", tt.pointer, err)
			}
			if strings.Join(got, "|") != strings.Join(tt.tokens, "|") {
				t.Fatalf("ParsePointer(%q) = %q, want %q", tt.pointer, got, tt.tokens)
			}
		})
	}

	for _, bad := range []string{"a/b", "/~", "/~2"} {
		if _, err := ParsePointer(bad); err == nil {
			t.Fatalf("ParsePointer(%q) = nil error, want failure", bad)
		}
	}
}

func TestOverlayAddDeduplicatesAndConflicts(t *testing.T) {
	t.Run("identical operations deduplicate", func(t *testing.T) {
		ov := &Overlay{}
		first := op(t, "/a", "x-one", ModeSet, `1`)
		second := op(t, "/a", "x-one", ModeSet, `1`)
		if err := ov.Add(first); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := ov.Add(second); err != nil {
			t.Fatalf("Add duplicate: %v", err)
		}
		if len(ov.Operations) != 1 {
			t.Fatalf("got %d operations, want 1", len(ov.Operations))
		}
	})

	t.Run("different value at the same key conflicts", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(op(t, "/a", "x-one", ModeSet, `1`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		err := ov.Add(op(t, "/a", "x-one", ModeSet, `2`))
		if err == nil {
			t.Fatal("Add conflicting value = nil error, want failure")
		}
		if !strings.Contains(err.Error(), "conflicting assignments") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("set and remove of the same key conflict", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(op(t, "/a", "x-one", ModeSet, `1`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		err := ov.Add(Operation{Pointer: "/a", Channel: ChannelExtension, Key: "x-one", Mode: ModeRemove})
		if err == nil {
			t.Fatal("Add conflicting mode = nil error, want failure")
		}
	})

	t.Run("channels do not collide", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(schemaOp(t, "/a", "default", ModeSet, `1`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		extension := op(t, "/a", "x-one", ModeSet, `1`)
		if err := ov.Add(extension); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if len(ov.Operations) != 2 {
			t.Fatalf("got %d operations, want 2", len(ov.Operations))
		}
	})

	t.Run("sibling keys on one node do not overlap", func(t *testing.T) {
		ov := &Overlay{}
		if err := ov.Add(op(t, "/a", "x-one", ModeSet, `1`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := ov.Add(op(t, "/a", "x-two", ModeSet, `2`)); err != nil {
			t.Fatalf("Add sibling: %v", err)
		}
	})
}

// TestOverlayRejectsOverlappingDestinations pins the reason uniqueness of
// (pointer, channel, key) is not enough. Setting "properties" on /X and
// "default" on /X/properties/foo have unique keys, but whichever runs first
// changes the result, so neither ordering can be shown to be correct.
func TestOverlayRejectsOverlappingDestinations(t *testing.T) {
	tests := []struct {
		name  string
		first Operation
		next  Operation
	}{
		{
			name:  "ancestor write then descendant write",
			first: Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.MappingNode}},
			next:  Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelSchema, Key: "default", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "v"}},
		},
		{
			name:  "descendant write then ancestor write",
			first: Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelSchema, Key: "default", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "v"}},
			next:  Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.MappingNode}},
		},
		{
			name:  "ancestor removal against descendant write",
			first: Operation{Pointer: "/a/b", Channel: ChannelExtension, Key: "x-keep", Mode: ModeRemove},
			next:  Operation{Pointer: "/a/b/x-keep/deep", Channel: ChannelExtension, Key: "x-two", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "v"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ov := &Overlay{}
			if err := ov.Add(tt.first); err != nil {
				t.Fatalf("Add first: %v", err)
			}
			err := ov.Add(tt.next)
			if err == nil {
				t.Fatal("Add overlapping operation = nil error, want failure")
			}
			if !strings.Contains(err.Error(), "overlapping destinations") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	t.Run("Overlaps reports the same pairs", func(t *testing.T) {
		ancestor := Operation{Pointer: "/x", Channel: ChannelSchema, Key: "properties"}
		descendant := Operation{Pointer: "/x/properties/foo", Channel: ChannelSchema, Key: "default"}
		sibling := Operation{Pointer: "/x/properties/bar", Channel: ChannelSchema, Key: "default"}
		otherChannel := Operation{Pointer: "/x/properties/foo", Channel: ChannelExtension, Key: "default"}
		if !Overlaps(ancestor, descendant) || !Overlaps(descendant, ancestor) {
			t.Fatal("Overlaps(ancestor, descendant) = false, want true")
		}
		if Overlaps(sibling, descendant) {
			t.Fatal("Overlaps(sibling, descendant) = true, want false")
		}
		if Overlaps(otherChannel, descendant) {
			t.Fatal("Overlaps across channels = true, want false")
		}
	})
}

func TestOverlayNormalizeIsOrderIndependent(t *testing.T) {
	ops := []Operation{
		op(t, "/z", "x-z", ModeSet, `1`),
		op(t, "/a", "x-a", ModeSet, `"one"`),
		op(t, "/a", "x-a", ModeSet, `"two"`), // conflicts; added directly below
	}
	forward := &Overlay{Operations: []Operation{ops[0], ops[1]}}
	backward := &Overlay{Operations: []Operation{ops[1], ops[0]}}
	forward.Normalize()
	backward.Normalize()
	first, err := forward.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	second, err := backward.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("normalized rendering differs by insertion order:\n%s\n%s", first, second)
	}
}

func TestOverlayRenderParseRoundTrip(t *testing.T) {
	ov := &Overlay{Source: "c1.api.v1.FooService", Kind: KindService}
	inputs := []Operation{
		op(t, "/paths/~1things/get", "x-speakeasy-entity-missing-codes", ModeSet, `[403,410]`),
		op(t, "/components/schemas/Foo", "x-future", ModeSet, `{"a":[1,2.50,null,false,"x"]}`),
		schemaOp(t, "/components/schemas/Foo", "title", ModeRemove, ""),
		op(t, "", "x-future-null", ModeSet, `null`),
		op(t, "", "x-future-empty", ModeSet, `""`),
	}
	for _, in := range inputs {
		if err := ov.Add(in); err != nil {
			t.Fatalf("Add(%v): %v", in, err)
		}
	}
	data, err := ov.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.Source != ov.Source || parsed.Kind != ov.Kind {
		t.Fatalf("round trip lost source/kind: %+v", parsed)
	}
	reRendered, err := parsed.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(reRendered) != string(data) {
		t.Fatalf("round trip is not stable:\n%s\n%s", data, reRendered)
	}

	t.Run("unknown version is rejected", func(t *testing.T) {
		if _, err := Parse([]byte("version: 99\noperations: []\n")); err == nil {
			t.Fatal("Parse(version 99) = nil error, want failure")
		}
	})

	t.Run("rendered operations are ordered", func(t *testing.T) {
		for i := 1; i < len(parsed.Operations); i++ {
			prev, cur := parsed.Operations[i-1], parsed.Operations[i]
			less := prev.Pointer < cur.Pointer ||
				(prev.Pointer == cur.Pointer && prev.Channel < cur.Channel) ||
				(prev.Pointer == cur.Pointer && prev.Channel == cur.Channel && prev.Key <= cur.Key)
			if !less {
				t.Fatalf("operations out of canonical order: %+v then %+v", prev, cur)
			}
		}
	})
}

func TestOverlayValidateOperation(t *testing.T) {
	tests := []struct {
		name    string
		op      Operation
		wantErr string
	}{
		{
			name:    "extension key must be an extension",
			op:      Operation{Pointer: "/a", Channel: ChannelExtension, Key: "default", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode}},
			wantErr: `requires an extension key starting with "x-"`,
		},
		{
			name:    "schema channel must not carry an extension",
			op:      Operation{Pointer: "/a", Channel: ChannelSchema, Key: "x-thing", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode}},
			wantErr: "must not carry the vendor extension key",
		},
		{
			name:    "set requires a value",
			op:      Operation{Pointer: "/a", Channel: ChannelExtension, Key: "x-thing", Mode: ModeSet},
			wantErr: "requires a value",
		},
		{
			name:    "remove must not carry a value",
			op:      Operation{Pointer: "/a", Channel: ChannelExtension, Key: "x-thing", Mode: ModeRemove, Value: &yaml.Node{Kind: yaml.ScalarNode}},
			wantErr: "must not carry a value",
		},
		{
			name:    "unknown channel",
			op:      Operation{Pointer: "/a", Channel: "other", Key: "x-thing", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode}},
			wantErr: "unknown channel",
		},
		{
			name:    "invalid pointer",
			op:      Operation{Pointer: "a", Channel: ChannelExtension, Key: "x-thing", Mode: ModeSet, Value: &yaml.Node{Kind: yaml.ScalarNode}},
			wantErr: "invalid JSON Pointer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Overlay{}).Add(tt.op)
			if err == nil {
				t.Fatalf("Add(%+v) = nil error, want %q", tt.op, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Add error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	t.Run("identical contributions deduplicate across sources", func(t *testing.T) {
		first := &Overlay{Source: "svc.a"}
		second := &Overlay{Source: "svc.b"}
		for _, ov := range []*Overlay{first, second} {
			if err := ov.Add(op(t, "", "x-speakeasy-globals", ModeSet, `{"parameters":[]}`)); err != nil {
				t.Fatalf("Add: %v", err)
			}
		}
		merged, err := Merge(first, second)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if len(merged.Operations) != 1 {
			t.Fatalf("got %d operations, want 1", len(merged.Operations))
		}
	})

	t.Run("contradictory root contributions are rejected", func(t *testing.T) {
		first := &Overlay{Source: "svc.a"}
		second := &Overlay{Source: "svc.b"}
		if err := first.Add(op(t, "", "x-speakeasy-globals", ModeSet, `{"parameters":[]}`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := second.Add(op(t, "", "x-speakeasy-globals", ModeSet, `{"parameters":[{"name":"t"}]}`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		_, err := Merge(first, second)
		if err == nil {
			t.Fatal("Merge = nil error, want failure")
		}
		if !strings.Contains(err.Error(), "svc.b") {
			t.Fatalf("Merge error should name the contributing source, got: %v", err)
		}
	})

	t.Run("cross-service schema collisions are rejected", func(t *testing.T) {
		first := &Overlay{Source: "svc.a"}
		second := &Overlay{Source: "svc.b"}
		if err := first.Add(schemaOp(t, "/components/schemas/Shared/properties/id", "default", ModeSet, `"a"`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := second.Add(schemaOp(t, "/components/schemas/Shared/properties/id", "default", ModeSet, `"b"`)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if _, err := Merge(first, second); err == nil {
			t.Fatal("Merge = nil error, want failure")
		}
	})
}

const testDocument = `
openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /things:
    get:
      operationId: ListThings
tags:
  - name: a
  - name: b
components:
  schemas:
    Thing:
      type: object
      title: A Thing
      properties:
        id:
          type: string
        tags:
          type: array
          items:
            type: string
`

func TestApply(t *testing.T) {
	tests := []struct {
		name     string
		ops      []Operation
		want     []string
		wantGone []string
		wantErr  string
	}{
		{
			name: "sets an extension on a nested schema",
			ops:  []Operation{op(t, "/components/schemas/Thing/properties/id", "x-speakeasy-param-readonly", ModeSet, `true`)},
			want: []string{"x-speakeasy-param-readonly: true"},
		},
		{
			name: "sets at the document root",
			ops:  []Operation{op(t, "", "x-speakeasy-globals", ModeSet, `{"parameters":[]}`)},
			want: []string{"x-speakeasy-globals:"},
		},
		{
			name:     "removes an existing key",
			ops:      []Operation{{Pointer: "/components/schemas/Thing", Channel: ChannelSchema, Key: "title", Mode: ModeRemove}},
			wantGone: []string{"A Thing"},
		},
		{
			name: "replaces an existing key",
			ops:  []Operation{schemaOp(t, "/components/schemas/Thing", "title", ModeSet, `"Replaced"`)},
			want: []string{"title: Replaced"},
		},
		{
			name: "array index resolves",
			ops:  []Operation{op(t, "/components/schemas/Thing/properties/tags/items", "x-speakeasy-response-filter", ModeSet, `true`)},
			want: []string{"x-speakeasy-response-filter: true"},
		},
		{
			name:    "missing pointer",
			ops:     []Operation{op(t, "/components/schemas/Missing", "x-a", ModeSet, `1`)},
			wantErr: "does not resolve",
		},
		{
			name:    "removing an absent key",
			ops:     []Operation{{Pointer: "/components/schemas/Thing", Channel: ChannelSchema, Key: "absent", Mode: ModeRemove}},
			wantErr: "key is not present",
		},
		{
			name:    "array index out of range",
			ops:     []Operation{op(t, "/tags/9", "x-a", ModeSet, `1`)},
			wantErr: "not a valid index",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Apply([]byte(testDocument), tt.ops)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Apply = %q, want error containing %q", out, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Apply error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Apply unexpected error: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(out), want) {
					t.Fatalf("applied document does not contain %q:\n%s", want, out)
				}
			}
			for _, gone := range tt.wantGone {
				if strings.Contains(string(out), gone) {
					t.Fatalf("applied document still contains %q:\n%s", gone, out)
				}
			}
		})
	}

	t.Run("removal actually removes", func(t *testing.T) {
		out, err := Apply([]byte(testDocument), []Operation{
			{Pointer: "/components/schemas/Thing", Channel: ChannelSchema, Key: "title", Mode: ModeRemove},
		})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if strings.Contains(string(out), "A Thing") {
			t.Fatalf("removed value is still present:\n%s", out)
		}
	})

	t.Run("apply is order independent for non-overlapping operations", func(t *testing.T) {
		forward := []Operation{
			op(t, "/components/schemas/Thing", "x-one", ModeSet, `1`),
			op(t, "/components/schemas/Thing/properties/id", "x-two", ModeSet, `2`),
		}
		backward := []Operation{forward[1], forward[0]}
		first, err := Apply([]byte(testDocument), forward)
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		second, err := Apply([]byte(testDocument), backward)
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if string(first) != string(second) {
			t.Fatalf("apply order changed the result:\n%s\n%s", first, second)
		}
	})
}

func TestValidate(t *testing.T) {
	good := op(t, "/components/schemas/Thing/properties/id", "x-a", ModeSet, `1`)
	if err := Validate([]byte(testDocument), []Operation{good}); err != nil {
		t.Fatalf("Validate(good) unexpected error: %v", err)
	}
	bad := op(t, "/components/schemas/Nope", "x-a", ModeSet, `1`)
	if err := Validate([]byte(testDocument), []Operation{bad}); err == nil {
		t.Fatal("Validate(missing pointer) = nil error, want failure")
	}
	removeMissing := Operation{Pointer: "/components/schemas/Thing", Channel: ChannelSchema, Key: "nope", Mode: ModeRemove}
	if err := Validate([]byte(testDocument), []Operation{removeMissing}); err == nil {
		t.Fatal("Validate(remove absent) = nil error, want failure")
	}
	if err := Validate([]byte(testDocument), nil); err != nil {
		t.Fatalf("Validate(no operations) unexpected error: %v", err)
	}
}

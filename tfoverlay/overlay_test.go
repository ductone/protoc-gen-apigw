package tfoverlay

import (
	"bytes"
	"errors"
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
//
// The containment check deliberately ignores channels: a write to an ancestor
// destroys its descendants whatever channel either is on, so a schema-channel
// write to /X "properties" and an extension-channel write to
// /X/properties/foo "x-test" still overlap.
func TestOverlayRejectsOverlappingDestinations(t *testing.T) {
	scalar := func(v string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
	}
	mapping := func() *yaml.Node { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }

	tests := []struct {
		name  string
		first Operation
		next  Operation
	}{
		{
			name:  "ancestor write then descendant write",
			first: Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: mapping()},
			next:  Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelSchema, Key: "default", Mode: ModeSet, Value: scalar("v")},
		},
		{
			name:  "descendant write then ancestor write",
			first: Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelSchema, Key: "default", Mode: ModeSet, Value: scalar("v")},
			next:  Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: mapping()},
		},
		{
			name:  "ancestor removal against descendant write",
			first: Operation{Pointer: "/a/b", Channel: ChannelExtension, Key: "x-keep", Mode: ModeRemove},
			next:  Operation{Pointer: "/a/b/x-keep/deep", Channel: ChannelExtension, Key: "x-two", Mode: ModeSet, Value: scalar("v")},
		},
		{
			// The schema channel writes the container the extension channel
			// writes into.
			name:  "schema ancestor against extension descendant",
			first: Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: mapping()},
			next:  Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelExtension, Key: "x-test", Mode: ModeSet, Value: scalar("v")},
		},
		{
			name:  "extension descendant against schema ancestor",
			first: Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelExtension, Key: "x-test", Mode: ModeSet, Value: scalar("v")},
			next:  Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: mapping()},
		},
		{
			name:  "extension ancestor against schema descendant",
			first: Operation{Pointer: "/components/schemas/X", Channel: ChannelExtension, Key: "x-entity", Mode: ModeSet, Value: scalar("X")},
			next:  Operation{Pointer: "/components/schemas/X/x-entity/deep", Channel: ChannelSchema, Key: "default", Mode: ModeSet, Value: scalar("v")},
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
			var structured *Error
			if !errors.As(err, &structured) {
				t.Fatalf("error %T is not a structured *Error", err)
			}
			if structured.Pointer != tt.next.Pointer || structured.Key != tt.next.Key {
				t.Fatalf("structured error points at %q/%q, want %q/%q",
					structured.Pointer, structured.Key, tt.next.Pointer, tt.next.Key)
			}
		})
	}

	t.Run("the same overlap is rejected through Merge", func(t *testing.T) {
		schema := &Overlay{Source: "svc.a"}
		extension := &Overlay{Source: "svc.b"}
		if err := schema.Add(Operation{Pointer: "/components/schemas/X", Channel: ChannelSchema, Key: "properties", Mode: ModeSet, Value: mapping()}); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := extension.Add(Operation{Pointer: "/components/schemas/X/properties/foo", Channel: ChannelExtension, Key: "x-test", Mode: ModeSet, Value: scalar("v")}); err != nil {
			t.Fatalf("Add: %v", err)
		}
		_, err := Merge(schema, extension)
		if err == nil {
			t.Fatal("Merge = nil error, want failure")
		}
		if !strings.Contains(err.Error(), "overlapping destinations") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Overlaps reports the same pairs", func(t *testing.T) {
		ancestor := Operation{Pointer: "/x", Channel: ChannelSchema, Key: "properties"}
		descendant := Operation{Pointer: "/x/properties/foo", Channel: ChannelSchema, Key: "default"}
		sibling := Operation{Pointer: "/x/properties/bar", Channel: ChannelSchema, Key: "default"}
		crossChannel := Operation{Pointer: "/x/properties/foo", Channel: ChannelExtension, Key: "x-test"}
		sameNodeOtherKey := Operation{Pointer: "/x", Channel: ChannelSchema, Key: "title"}
		if !Overlaps(ancestor, descendant) || !Overlaps(descendant, ancestor) {
			t.Fatal("Overlaps(ancestor, descendant) = false, want true")
		}
		if !Overlaps(ancestor, crossChannel) || !Overlaps(crossChannel, ancestor) {
			t.Fatal("Overlaps across channels = false, want true for a containment")
		}
		if Overlaps(sibling, descendant) {
			t.Fatal("Overlaps(sibling, descendant) = true, want false")
		}
		if Overlaps(sameNodeOtherKey, ancestor) {
			t.Fatal("Overlaps(same node, different key) = true, want false")
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

// TestParseRejectsMalformedArtifacts keeps the artifact contract strict: an
// artifact is machine-written, so a missing, unknown or mistyped field means
// producer and consumer disagree rather than that a default applies.
func TestParseRejectsMalformedArtifacts(t *testing.T) {
	const valid = "version: 1\n" +
		"source: svc\n" +
		"kind: service\n" +
		"operations:\n" +
		"  - pointer: /a\n" +
		"    channel: extension\n" +
		"    key: x-one\n" +
		"    mode: set\n" +
		"    value: 1\n"
	if _, err := Parse([]byte(valid)); err != nil {
		t.Fatalf("Parse(valid) unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "missing version", input: "operations: []\n", wantErr: "version is required"},
		{name: "version is a string", input: "version: \"1\"\noperations: []\n", wantErr: `field "version" must be an integer`},
		{name: "unsupported version", input: "version: 2\noperations: []\n", wantErr: "unsupported version"},
		{name: "missing operations", input: "version: 1\n", wantErr: "operations is required"},
		{name: "operations is not a sequence", input: "version: 1\noperations: {}\n", wantErr: "operations must be a sequence"},
		{name: "unknown top-level field", input: "version: 1\noperations: []\nextra: 1\n", wantErr: `unknown field "extra"`},
		{name: "source is not a string", input: "version: 1\nsource: 7\noperations: []\n", wantErr: `field "source" must be a string`},
		{name: "unknown kind", input: "version: 1\nkind: other\noperations: []\n", wantErr: "unknown kind"},
		{
			name: "missing pointer",
			input: "version: 1\noperations:\n" +
				"  - channel: extension\n    key: x-one\n    mode: set\n    value: 1\n",
			wantErr: "missing pointer",
		},
		{
			name: "missing mode",
			input: "version: 1\noperations:\n" +
				"  - pointer: /a\n    channel: extension\n    key: x-one\n    value: 1\n",
			wantErr: "is missing mode",
		},
		{
			name: "missing value for set",
			input: "version: 1\noperations:\n" +
				"  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: set\n",
			wantErr: "is missing value",
		},
		{
			name: "value on remove",
			input: "version: 1\noperations:\n" +
				"  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: remove\n    value: 1\n",
			wantErr: "must not carry a value",
		},
		{
			name: "unknown operation field",
			input: "version: 1\noperations:\n" +
				"  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: set\n    value: 1\n    extra: 1\n",
			wantErr: `unknown operation field "extra"`,
		},
		{
			name: "pointer is not a string",
			input: "version: 1\noperations:\n" +
				"  - pointer: 5\n    channel: extension\n    key: x-one\n    mode: set\n    value: 1\n",
			wantErr: `field "pointer" must be a string`,
		},
		{
			name: "contradictory operations",
			input: valid +
				"  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: set\n    value: 2\n",
			wantErr: "conflicting assignments",
		},
		{
			name: "overlapping operations",
			input: "version: 1\noperations:\n" +
				"  - pointer: /a\n    channel: schema\n    key: properties\n    mode: set\n    value: {}\n" +
				"  - pointer: /a/properties/b\n    channel: extension\n    key: x-two\n    mode: set\n    value: 1\n",
			wantErr: "overlapping destinations",
		},
		{name: "document root is not a mapping", input: "- 1\n", wantErr: "expected a mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want %q", tt.input, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Parse error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}

	t.Run("identical operations deduplicate", func(t *testing.T) {
		input := valid + "  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: set\n    value: 1\n"
		ov, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(ov.Operations) != 1 {
			t.Fatalf("got %d operations, want 1", len(ov.Operations))
		}
	})

	t.Run("an empty operation list is valid", func(t *testing.T) {
		ov, err := Parse([]byte("version: 1\noperations: []\n"))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if !ov.Empty() {
			t.Fatalf("got %d operations, want none", len(ov.Operations))
		}
	})

	t.Run("provenance round trips", func(t *testing.T) {
		input := valid + "    provenance: " + testFieldProvenance + "\n"
		ov, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if ov.Operations[0].Provenance != testFieldProvenance {
			t.Fatalf("provenance = %q", ov.Operations[0].Provenance)
		}
	})
}

// TestApplyAndValidateRejectInconsistentLists pins that the public entrypoints
// do not depend on the caller remembering to Merge: an order-dependent list is
// rejected wherever it enters.
func TestApplyAndValidateRejectInconsistentLists(t *testing.T) {
	lists := map[string][]Operation{
		"contradictory": {
			op(t, "/components/schemas/Thing", "x-one", ModeSet, "1"),
			op(t, "/components/schemas/Thing", "x-one", ModeSet, "2"),
		},
		"overlapping across channels": {
			schemaOp(t, "/components/schemas/Thing", "properties", ModeSet, "{}"),
			op(t, "/components/schemas/Thing/properties/id", "x-test", ModeSet, "1"),
		},
	}
	for name, ops := range lists {
		t.Run(name, func(t *testing.T) {
			if _, err := Apply([]byte(testDocument), ops); err == nil {
				t.Fatal("Apply = nil error, want failure")
			} else if !strings.Contains(err.Error(), "inconsistent operation list") {
				t.Fatalf("Apply error = %v, want an inconsistent-list failure", err)
			}
			if err := Validate([]byte(testDocument), ops); err == nil {
				t.Fatal("Validate = nil error, want failure")
			} else if !strings.Contains(err.Error(), "inconsistent operation list") {
				t.Fatalf("Validate error = %v, want an inconsistent-list failure", err)
			}
		})
	}
}

// TestValidateRequiresObjectDestination keeps Validate and Apply agreeing: a
// destination that is a scalar or a sequence cannot take a key, so Validate
// rejects it rather than accepting a list Apply would then fail on.
func TestValidateRequiresObjectDestination(t *testing.T) {
	scalarTarget := op(t, "/components/schemas/Thing/type", "x-test", ModeSet, "1")
	err := Validate([]byte(testDocument), []Operation{scalarTarget})
	if err == nil {
		t.Fatal("Validate(scalar destination) = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "destination is not an object") {
		t.Fatalf("Validate error = %v, want an object-destination failure", err)
	}
	if _, err := Apply([]byte(testDocument), []Operation{scalarTarget}); err == nil {
		t.Fatal("Apply(scalar destination) = nil error, want failure")
	}
}

// TestMergeNamesBothSources requires the diagnostic to identify both
// contributions, not only the incoming one.
func TestMergeNamesBothSources(t *testing.T) {
	first := &Overlay{Source: "svc.alpha"}
	second := &Overlay{Source: "svc.beta"}
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
	assertNamesBoth(t, err, "svc.alpha", "svc.beta")

	t.Run("overlap across sources names both", func(t *testing.T) {
		a := &Overlay{Source: "svc.alpha"}
		b := &Overlay{Source: "svc.beta"}
		if err := a.Add(schemaOp(t, "/components/schemas/Thing", "properties", ModeSet, "{}")); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := b.Add(op(t, "/components/schemas/Thing/properties/id", "x-test", ModeSet, "1")); err != nil {
			t.Fatalf("Add: %v", err)
		}
		_, err := Merge(a, b)
		if err == nil {
			t.Fatal("Merge = nil error, want failure")
		}
		assertNamesBoth(t, err, "svc.alpha", "svc.beta")
	})
}

func assertNamesBoth(t *testing.T, err error, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not name %q", err, name)
		}
	}
}

// TestErrorsAreStructured requires failures to be machine-readable: a linter
// needs the destination and the contributing annotations, not a message to
// parse.
const testFieldProvenance = "field test.Thing.id"

func TestErrorsAreStructured(t *testing.T) {
	withOwner := op(t, "/a", "x-one", ModeSet, "1")
	withOwner.Provenance = testFieldProvenance
	conflicting := op(t, "/a", "x-one", ModeSet, "2")
	ov := &Overlay{}
	if err := ov.Add(withOwner); err != nil {
		t.Fatalf("Add: %v", err)
	}
	err := ov.Add(conflicting)
	var structured *Error
	if !errors.As(err, &structured) {
		t.Fatalf("error %T is not a structured *Error", err)
	}
	if structured.Pointer != "/a" || structured.Channel != ChannelExtension || structured.Key != "x-one" {
		t.Fatalf("structured error = %+v", structured)
	}
	if len(structured.Owners) != 1 || structured.Owners[0] != testFieldProvenance {
		t.Fatalf("structured owners = %v, want the contributing annotation", structured.Owners)
	}

	t.Run("provenance is carried into Apply and Validate failures", func(t *testing.T) {
		missing := op(t, "/components/schemas/Missing", "x-one", ModeSet, "1")
		missing.Provenance = testFieldProvenance
		applyErr := Validate([]byte(testDocument), []Operation{missing})
		var applyStructured *Error
		if !errors.As(applyErr, &applyStructured) {
			t.Fatalf("Validate error %T is not a structured *Error", applyErr)
		}
		if applyStructured.Pointer != "/components/schemas/Missing" {
			t.Fatalf("structured pointer = %q", applyStructured.Pointer)
		}
		if len(applyStructured.Owners) != 1 || applyStructured.Owners[0] != testFieldProvenance {
			t.Fatalf("structured owners = %v", applyStructured.Owners)
		}
	})

	t.Run("provenance survives render and parse", func(t *testing.T) {
		ov := &Overlay{Source: "svc"}
		annotated := op(t, "/a", "x-one", ModeSet, "1")
		annotated.Provenance = "message test.Thing"
		if err := ov.Add(annotated); err != nil {
			t.Fatalf("Add: %v", err)
		}
		data, err := ov.Render()
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if !strings.Contains(string(data), "provenance: message test.Thing") {
			t.Fatalf("rendered artifact lost provenance:\n%s", data)
		}
		parsed, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if parsed.Operations[0].Provenance != "message test.Thing" {
			t.Fatalf("parsed provenance = %q", parsed.Operations[0].Provenance)
		}
	})
}

func byteIdentical(t *testing.T, want, got []byte, context string) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Fatalf("%s differs:\n--- want\n%s\n--- got\n%s", context, want, got)
	}
}

// TestDuplicateOperationsHaveEntrypointParity pins the bug the review caught:
// validation used to build a deduplicated list and throw it away while Apply
// iterated the original, so two identical remove operations passed validation
// and then failed — the first removed the key, the second did not find it.
// Every exported entrypoint must agree, and must operate on the canonical list.
func TestDuplicateOperationsHaveEntrypointParity(t *testing.T) {
	duplicateRemove := []Operation{
		{Pointer: "/components/schemas/Thing", Channel: ChannelSchema, Key: "title", Mode: ModeRemove},
		{Pointer: "/components/schemas/Thing", Channel: ChannelSchema, Key: "title", Mode: ModeRemove},
	}
	duplicateSet := []Operation{
		op(t, "/components/schemas/Thing", "x-one", ModeSet, `"v"`),
		op(t, "/components/schemas/Thing", "x-one", ModeSet, `"v"`),
	}

	t.Run("duplicate removes are accepted and remove once", func(t *testing.T) {
		if err := Validate([]byte(testDocument), duplicateRemove); err != nil {
			t.Fatalf("Validate(duplicate removes) = %v, want nil", err)
		}
		out, err := Apply([]byte(testDocument), duplicateRemove)
		if err != nil {
			t.Fatalf("Apply(duplicate removes) = %v, want nil", err)
		}
		if strings.Contains(string(out), "A Thing") {
			t.Fatalf("value survived removal:\n%s", out)
		}
	})

	t.Run("duplicate sets are accepted and applied once", func(t *testing.T) {
		single, err := Apply([]byte(testDocument), duplicateSet[:1])
		if err != nil {
			t.Fatalf("Apply(single) = %v", err)
		}
		duplicated, err := Apply([]byte(testDocument), duplicateSet)
		if err != nil {
			t.Fatalf("Apply(duplicate sets) = %v, want nil", err)
		}
		byteIdentical(t, single, duplicated, "duplicate-set application")
		if err := Validate([]byte(testDocument), duplicateSet); err != nil {
			t.Fatalf("Validate(duplicate sets) = %v, want nil", err)
		}
	})

	t.Run("ApplyToNode agrees with Apply", func(t *testing.T) {
		root := &yaml.Node{}
		if err := yaml.Unmarshal([]byte(testDocument), root); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if err := ApplyToNode(root, duplicateRemove); err != nil {
			t.Fatalf("ApplyToNode(duplicate removes) = %v, want nil", err)
		}
	})

	t.Run("order still does not matter after deduplication", func(t *testing.T) {
		reordered := []Operation{duplicateSet[1], duplicateSet[0]}
		first, err := Apply([]byte(testDocument), duplicateSet)
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		second, err := Apply([]byte(testDocument), reordered)
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		byteIdentical(t, first, second, "duplicate-set ordering")
	})
}

// TestParseRejectsDuplicateMetadataAndTrailingDocuments closes the other half
// of the same gap: decoding YAML into a Node does not detect a duplicate
// mapping key, so a duplicate pointer, mode or version could silently take the
// last value.
func TestParseRejectsDuplicateMetadataAndTrailingDocuments(t *testing.T) {
	const operation = "  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: set\n    value: 1\n"

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "duplicate version",
			input:   "version: 1\nversion: 1\noperations: []\n",
			wantErr: `duplicate field "version"`,
		},
		{
			name:    "duplicate operations",
			input:   "version: 1\noperations: []\noperations: []\n",
			wantErr: `duplicate field "operations"`,
		},
		{
			name:    "duplicate source",
			input:   "version: 1\nsource: a\nsource: b\noperations: []\n",
			wantErr: `duplicate field "source"`,
		},
		{
			name:    "duplicate pointer",
			input:   "version: 1\noperations:\n  - pointer: /a\n    pointer: /b\n    channel: extension\n    key: x-one\n    mode: set\n    value: 1\n",
			wantErr: `duplicate operation field "pointer"`,
		},
		{
			name:    "duplicate mode",
			input:   "version: 1\noperations:\n  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: remove\n    mode: set\n    value: 1\n",
			wantErr: `duplicate operation field "mode"`,
		},
		{
			name:    "duplicate value",
			input:   "version: 1\noperations:\n  - pointer: /a\n    channel: extension\n    key: x-one\n    mode: set\n    value: 1\n    value: 2\n",
			wantErr: `duplicate operation field "value"`,
		},
		{
			name:    "trailing document",
			input:   "version: 1\noperations:\n" + operation + "---\nversion: 1\noperations: []\n",
			wantErr: "unexpected trailing YAML document",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want %q", tt.input, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Parse error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}

	t.Run("a single trailing comment is still one document", func(t *testing.T) {
		if _, err := Parse([]byte("version: 1\noperations: []\n# trailing comment\n")); err != nil {
			t.Fatalf("Parse = %v, want nil", err)
		}
	})
}

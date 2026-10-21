package enumquery

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"
)

type queryInput struct{ values url.Values }

func (q queryInput) Body() io.Reader         { return strings.NewReader("") }
func (q queryInput) Query() url.Values       { return q.values }
func (q queryInput) PathParam(string) string { return "" }

func TestGeneratedEnumQuery(t *testing.T) {
	tests := []struct {
		name, value string
		want        ExampleFormat
		invalid     bool
	}{
		{name: "omitted"},
		{name: "empty", value: ""},
		{name: "unspecified symbol", value: "EXAMPLE_FORMAT_UNSPECIFIED"},
		{name: "json symbol", value: "EXAMPLE_FORMAT_JSON", want: 1},
		{name: "text symbol", value: "EXAMPLE_FORMAT_TEXT", want: 2},
		{name: "alias", value: "EXAMPLE_FORMAT_PLAIN", want: 2},
		{name: "numeric zero", value: "0"},
		{name: "numeric one", value: "1", want: 1},
		{name: "numeric two", value: "2", want: 2},
		{name: "unknown numeric left to application", value: "99", want: 99},
		{name: "negative unknown", value: "-1", want: -1},
		{name: "maximum int32", value: "2147483647", want: 2147483647},
		{name: "minimum int32", value: "-2147483648", want: -2147483648},
		{name: "decimal leading zero", value: "02", want: 2},
		{name: "decimal plus", value: "+2", want: 2},
		{name: "unknown symbol", value: "EXAMPLE_FORMAT_OTHER", invalid: true},
		{name: "case sensitive", value: "example_format_text", invalid: true},
		{name: "noninteger", value: "1.5", invalid: true},
		{name: "hex", value: "0x2", invalid: true},
		{name: "whitespace", value: " 2", invalid: true},
		{name: "overflow", value: "2147483648", invalid: true},
		{name: "underflow", value: "-2147483649", invalid: true},
		{name: "wraparound", value: "4294967298", invalid: true},
		{name: "int64 overflow", value: "9223372036854775808", invalid: true},
		{name: "untrusted text", value: "untrusted\nvalue", invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := url.Values{}
			if tt.name != "omitted" {
				values.Set("format", tt.value)
			}
			out := new(Request)
			err := apigw_desc_ExampleServiceServer.Methods[0].Decoder(context.Background(), queryInput{values}, out)
			if tt.invalid {
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("expected InvalidArgument, got %v (decoded %v)", err, out.GetFormat())
				}
				if strings.Contains(status.Convert(err).Message(), tt.value) {
					t.Fatalf("error reflects untrusted input: %v", err)
				}
				return
			}
			if err != nil || out.GetFormat() != tt.want {
				t.Fatalf("got %v, %v; want %v", out.GetFormat(), err, tt.want)
			}
		})
	}
}

// Every published query enum spelling must be accepted by the generated
// decoder, including imported enum types, nested enum declarations and aliases.
func TestGeneratedOpenAPIEnumQuery(t *testing.T) {
	var doc struct {
		Paths map[string]struct {
			Get struct {
				Parameters []struct {
					Name   string `yaml:"name"`
					Schema struct {
						Type string   `yaml:"type"`
						Enum []string `yaml:"enum"`
					} `yaml:"schema"`
				} `yaml:"parameters"`
			} `yaml:"get"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal([]byte(APIGW_OAS31_ExampleServiceServer), &doc); err != nil {
		t.Fatal(err)
	}
	parameters := doc.Paths["/examples"].Get.Parameters
	if len(parameters) != 3 {
		t.Fatalf("expected three enum parameters, got %v", parameters)
	}
	for _, p := range parameters {
		if p.Schema.Type != "string" || len(p.Schema.Enum) == 0 {
			t.Fatalf("expected symbolic string enum for %s: %v", p.Name, p.Schema)
		}
		for _, value := range p.Schema.Enum {
			t.Run(p.Name+"/"+value, func(t *testing.T) {
				out := new(Request)
				err := apigw_desc_ExampleServiceServer.Methods[0].Decoder(context.Background(), queryInput{url.Values{p.Name: {value}}}, out)
				if err != nil {
					t.Fatalf("advertised enum rejected: %v", err)
				}
				field := out.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(p.Name))
				want := field.Enum().Values().ByName(protoreflect.Name(value))
				if want == nil || out.ProtoReflect().Get(field).Enum() != want.Number() {
					t.Fatalf("wrong typed enum decoded: %v", out)
				}
			})
		}
	}
}

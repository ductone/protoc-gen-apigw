package apigw

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/testing/protopack"
)

func TestBoolFieldTemplateSkipsAbsentQueryParameter(t *testing.T) {
	output, err := templateExecToString("field_bool.tmpl", &boolFieldContext{
		FieldName:  "latestOnly",
		Getter:     `vn0 := input.Query().Get("latest_only")`,
		InputName:  "vn0",
		OutputName: "vn1",
		ParamName:  "latest_only",
		IsQuery:    true,
		Tag:        protopack.Number(3),
	})
	if err != nil {
		t.Fatalf("render bool template: %v", err)
	}

	if !strings.Contains(output, `vn1 := protopack.Message{}`) {
		t.Fatalf("rendered converter does not initialize an absent query parameter: \n%s", output)
	}
	if !strings.Contains(output, `if _, ok := input.Query()["latest_only"]; ok {`) {
		t.Fatalf("rendered converter does not check whether the query parameter is present:\n%s", output)
	}
	if strings.Index(output, `if _, ok := input.Query()["latest_only"]; ok {`) > strings.Index(output, "strconv.ParseBool") {
		t.Fatalf("rendered converter parses before checking whether the query parameter is present:\n%s", output)
	}
}

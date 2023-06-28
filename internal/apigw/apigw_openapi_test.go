package apigw

import (
	"testing"
)

func Test_toSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "foo",
			want:  "foo",
		},
		{
			input: "foo.bar",
			want:  "foo_bar",
		},
		{
			input: "foo.bar.baz",
			want:  "foo_bar_baz",
		},
		{
			input: "foo_bar.baz",
			want:  "foo_bar_baz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toSnakeCase(tt.input); got != tt.want {
				t.Errorf("toSnakeCase() = %v, want %v", got, tt.want)
			}
		})
	}
}

package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRoute(t *testing.T) {
	tkn, err := ParseRoute("/foo/bar")
	require.NoError(t, err)
	require.Equal(t, tkn, []RouteToken{
		{
			Value: "foo",
		},
		{
			Value: "bar",
		},
	})
	tkn, err = ParseRoute("/foo/{thing}")
	require.NoError(t, err)
	require.Equal(t, tkn, []RouteToken{
		{
			Value: "foo",
		},
		{
			IsParam:   true,
			ParamName: "thing",
		},
	})
}

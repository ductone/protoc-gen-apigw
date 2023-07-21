package v1

import "strings"

func EnumValuesAsString(e map[string]int32) string {
	sb := strings.Builder{}
	first := true
	for k := range e {
		if first {
			first = false
		} else {
			_, _ = sb.WriteString(", ")
		}
		_, _ = sb.WriteString(k)
	}
	return sb.String()
}

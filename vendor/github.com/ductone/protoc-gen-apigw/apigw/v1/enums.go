package v1

import "strings"

func EnumValuesAsString(e map[string]int32) string {
	sb := strings.Builder{}
	first := true
	for k, _ := range e {
		if first {
			first = false
		} else {
			sb.WriteString(", ")
		}
		sb.WriteString(k)
	}
	return sb.String()
}

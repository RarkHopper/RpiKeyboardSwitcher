package complete

import (
	"fmt"
	"io"
	"sort"
)

func PrintValues(stdout io.Writer, values []string) {
	sort.Strings(values)
	for _, value := range values {
		_, _ = fmt.Fprintln(stdout, value)
	}
}

func MapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	return keys
}

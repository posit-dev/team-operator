package v1beta1

import (
	"fmt"
	"reflect"
	"sort"
)

// sortedMapKeys returns the keys of a map value sorted by their string form.
// Go randomizes map iteration order, so config renderers must iterate maps in
// a fixed order to produce byte-identical output (and stable hash annotations)
// across reconciles.
func sortedMapKeys(m reflect.Value) []reflect.Value {
	keys := m.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%v", keys[i]) < fmt.Sprintf("%v", keys[j])
	})
	return keys
}

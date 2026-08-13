// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ccschema

// PropertyJsonPointers is a list of PropertyJsonPointer.
type PropertyJsonPointers []PropertyJsonPointer

// ContainsPath returns true if an element matches the path.
func (ptrs PropertyJsonPointers) ContainsPath(path []string) bool {
	for _, ptr := range ptrs {
		if ptr.EqualsPath(path) {
			return true
		}
	}

	return false
}

// ContainsPathString returns true if an element matches the JSON Pointer path.
func (ptrs PropertyJsonPointers) ContainsPathString(path string) bool {
	for _, ptr := range ptrs {
		if ptr.String() == path {
			return true
		}
	}

	return false
}

/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import "github.com/StudioSol/set"

// Set type Reference c++ set interface to implemented stl set.
// With generics, it may be more generic.

func Difference(left, right *set.LinkedHashSetString) *set.LinkedHashSetString {
	diff := set.NewLinkedHashSetString()
	for e := range left.Iter() {
		if !right.InArray(e) {
			diff.Add(e)
		}
	}
	return diff
}

func ToSet[T interface{}](v map[string]T) *set.LinkedHashSetString {
	r := set.NewLinkedHashSetString()
	for k := range v {
		r.Add(k)
	}
	return r
}

func EqSet(left, right *set.LinkedHashSetString) bool {
	if left.Length() != right.Length() {
		return false
	}
	for e := range left.Iter() {
		if !right.InArray(e) {
			return false
		}
	}
	return true
}

func MapKeyDifference[T interface{}](left, right map[string]T) *set.LinkedHashSetString {
	lSet := ToSet(left)
	rSet := ToSet(right)
	return Difference(lSet, rSet)
}

func Union(left, right *set.LinkedHashSetString) *set.LinkedHashSetString {
	deleteSet := Difference(left, right)
	return Difference(left, deleteSet)
}

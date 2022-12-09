/*
Copyright ApeCloud Inc.

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

package configuration

// Set type Reference c++ set interface to implemented stl set.
// With generics, it may be more generic.
type Set map[string]EmptyStruct

type EmptyStruct struct{}

func (s *Set) Insert(v string) bool {
	prevLen := len(*s)
	(*s)[v] = EmptyStruct{}
	return prevLen != len(*s)
}

func (s *Set) InsertArray(arr []string) {
	for _, v := range arr {
		s.Insert(v)
	}
}

func NewSet() *Set {
	s := make(Set)
	return &s
}

func NewSetFromList(v []string) *Set {
	s := NewSet()

	for _, item := range v {
		s.Insert(item)
	}

	return s
}

func NewSetFromMap[T interface{}](v map[string]T) *Set {
	s := NewSet()

	for key := range v {
		s.Insert(key)
	}

	return s
}

func Difference(left, right *Set) *Set {
	return left.Difference(right)
}

func MapKeyDifference[T interface{}](left, right map[string]T) *Set {
	lSet := NewSetFromMap(left)
	rSet := NewSetFromMap(right)
	return Difference(lSet, rSet)
}

func Union(left, right *Set) *Set {
	deleteSet := Difference(left, right)
	return Difference(left, deleteSet)
}

func (s *Set) Difference(other *Set) *Set {
	diff := NewSet()

	for e := range *s {
		if !other.Contains(e) {
			diff.Insert(e)
		}
	}

	return diff
}

type ApplyFunc func(key string)

func (s *Set) ForEach(fn ApplyFunc) {
	if s.Size() == 0 {
		return
	}
	for key := range *s {
		fn(key)
	}
}

func (s *Set) Contains(v string) bool {
	_, ok := (*s)[v]
	return ok
}

func (s *Set) Size() int {
	return len(*s)
}

func (s *Set) Empty() bool {
	return s.Size() == 0
}

func (s *Set) Remove(v string) bool {
	ok := s.Contains(v)
	if ok {
		delete(*s, v)
		return true
	}

	return false
}

func (s *Set) ToList() []string {
	if s.Empty() {
		return nil
	}

	tmp := make([]string, 0, s.Size())
	s.ForEach(func(key string) {
		tmp = append(tmp, key)
	})
	return tmp
}

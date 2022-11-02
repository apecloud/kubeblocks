/*
Copyright 2022.

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

import "github.com/spf13/viper"

// Reference c++ set interface to implemented stl_set
// With generics, it may be more generic
type Set map[string]struct{}

func (s *Set) Insert(v string) bool {
	prevLen := len(*s)
	(*s)[v] = struct{}{}
	return prevLen != len(*s)
}

func NewSet() *Set {
	s := make(Set)
	return &s
}

func NewSetFromMap(v map[string]*viper.Viper) *Set {
	s := NewSet()

	for key := range v {
		s.Insert(key)
	}

	return s
}

func Difference(left, right *Set) *Set {
	return left.Difference(right)
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

func (s *Set) Contains(v string) bool {
	_, ok := (*s)[v]
	return ok
}

func (s *Set) Size() int {
	return len(*s)
}

func (s *Set) Remove(v string) bool {
	ok := s.Contains(v)
	if ok {
		delete(*s, v)
		return true
	}

	return false
}

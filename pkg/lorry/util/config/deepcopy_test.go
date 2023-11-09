/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package config

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stringStruct struct {
	Key   string
	Value string
}

type sliceStruct struct {
	Key string
	A   []int
}

type mapStruct struct {
	Key string
	M   map[string]string
}

func Print(a any) {
	v := reflect.ValueOf(a)
	fmt.Print(v.Kind())
	b := v.Interface()
	c, ok := b.(*stringStruct)
	if ok {
		c.Key = "key2"
		fmt.Printf("Key: %v\n", v)
		fmt.Printf("Key: %v\n", c)
	}

}

func TestDeepCopy(t *testing.T) {
	t.Run("test struct with String", func(t *testing.T) {
		s := &stringStruct{
			Key:   "key1",
			Value: "values",
		}
		d := &stringStruct{}
		DeepCopy(s, d)
		t.Logf("s: %v\n", s)
		t.Logf("d: %v\n", d)
		assert.Equal(t, s, d)

		a := *s
		Print(a)
		Print(s)
	})

	t.Run("test struct with alice", func(t *testing.T) {
		s := sliceStruct{
			Key: "slice",
			A:   []int{1, 3},
		}
		func(s any) {
			sv := reflect.ValueOf(s)
			//p1 := unsafe.Pointer(sv.UnsafeAddr())
			//s1 := (*sliceStruct)(p1)
			fmt.Printf("sliceStruct s: %v \n", s)
			s1 := s.(sliceStruct)
			s1.Key = "slice1"
			s1.A[1] = 2
			fmt.Printf("sliceStruct s: %v \n", s)
			fmt.Printf("sliceStruct s1: %v \n", s1)

			s2 := sv.Interface().(sliceStruct)
			s2.A[1] = 4
			s2.Key = "slice2"
			fmt.Printf("sliceStruct s: %v \n", s)
			fmt.Printf("sliceStruct s2: %v \n", s2)
		}(s)
		fmt.Printf("sliceStruct s: %v \n", s)

		d := &sliceStruct{}
		DeepCopy(&s, d)
		fmt.Printf("sliceStruct d: %v \n", d)
		assert.Equal(t, *d, s)
		d.A[0] = 2
		fmt.Printf("sliceStruct s: %v \n", s)
		fmt.Printf("sliceStruct d: %v \n", d)
		assert.NotEqual(t, *d, s)
	})

	t.Run("test struct with map", func(t *testing.T) {
		s := mapStruct{
			Key: "map1",
			M:   map[string]string{"key": "value"},
		}
		fmt.Printf("s: %v\n", s)
		d := &mapStruct{}
		DeepCopy(&s, d)
		fmt.Printf("d: %v\n", d)
		assert.Equal(t, s, *d)

		d.M["key2"] = "value2"
		fmt.Printf("s: %v\n", s)
		fmt.Printf("d: %v\n", d)
		assert.NotEqual(t, s, *d)
	})
}

/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package lru

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- New ---

func TestNew(t *testing.T) {
	c := New(10)
	require.NotNil(t, c)
	assert.Equal(t, 10, c.capacity)
	assert.NotNil(t, c.list)
	assert.NotNil(t, c.items)
}

func TestNew_ZeroCapacity(t *testing.T) {
	c := New(0)
	require.NotNil(t, c)
	assert.Equal(t, 0, c.capacity)
}

// --- Get ---

func TestGet_Miss(t *testing.T) {
	c := New(5)
	val, ok := c.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestGet_Hit(t *testing.T) {
	c := New(5)
	c.Put("key1", "value1")
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestGet_MovesToFront(t *testing.T) {
	c := New(3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	// Access "a" to move it to front
	_, _ = c.Get("a")

	// Now insert "d" — should evict "b" (the least recently used), not "a"
	c.Put("d", 4)

	_, ok := c.Get("b")
	assert.False(t, ok, "b should have been evicted")

	val, ok := c.Get("a")
	assert.True(t, ok, "a should still be present after Get moved it to front")
	assert.Equal(t, 1, val)
}

// --- Put ---

func TestPut_NewEntry(t *testing.T) {
	c := New(5)
	c.Put("key1", "val1")
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "val1", val)
}

func TestPut_UpdateExisting(t *testing.T) {
	c := New(5)
	c.Put("key1", "val1")
	c.Put("key1", "val2")
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "val2", val)
}

func TestPut_EvictsLRU(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // should evict "a"

	_, ok := c.Get("a")
	assert.False(t, ok, "a should have been evicted")

	val, ok := c.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, val)

	val, ok = c.Get("c")
	assert.True(t, ok)
	assert.Equal(t, 3, val)
}

func TestPut_UpdateMovesToFront(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 10) // update "a", moves to front

	c.Put("c", 3) // should evict "b" (now LRU), not "a"

	_, ok := c.Get("b")
	assert.False(t, ok, "b should have been evicted")

	val, ok := c.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 10, val)
}

func TestPut_NilValue(t *testing.T) {
	c := New(5)
	c.Put("key", nil)
	val, ok := c.Get("key")
	assert.True(t, ok)
	assert.Nil(t, val)
}

func TestPut_DifferentTypes(t *testing.T) {
	c := New(5)
	c.Put("int", 42)
	c.Put("string", "hello")
	c.Put("struct", struct{ Name string }{"test"})

	v1, ok := c.Get("int")
	assert.True(t, ok)
	assert.Equal(t, 42, v1)

	v2, ok := c.Get("string")
	assert.True(t, ok)
	assert.Equal(t, "hello", v2)

	v3, ok := c.Get("struct")
	assert.True(t, ok)
	assert.Equal(t, struct{ Name string }{"test"}, v3)
}

// --- Eviction order ---

func TestEvictionOrder_FIFO(t *testing.T) {
	c := New(3)
	c.Put("1", "a")
	c.Put("2", "b")
	c.Put("3", "c")
	c.Put("4", "d") // evicts "1"
	c.Put("5", "e") // evicts "2"

	_, ok := c.Get("1")
	assert.False(t, ok)
	_, ok = c.Get("2")
	assert.False(t, ok)

	_, ok = c.Get("3")
	assert.True(t, ok)
	_, ok = c.Get("4")
	assert.True(t, ok)
	_, ok = c.Get("5")
	assert.True(t, ok)
}

func TestEvictionOrder_GetPreventsEviction(t *testing.T) {
	c := New(3)
	c.Put("1", "a")
	c.Put("2", "b")
	c.Put("3", "c")

	// Access "1" so it becomes most recently used
	c.Get("1")

	c.Put("4", "d") // should evict "2" (LRU now)

	_, ok := c.Get("1")
	assert.True(t, ok, "1 should not be evicted after Get")
	_, ok = c.Get("2")
	assert.False(t, ok, "2 should be evicted as LRU")
}

// --- Capacity edge cases ---

func TestCapacityOne(t *testing.T) {
	c := New(1)
	c.Put("a", 1)
	c.Put("b", 2) // evicts "a"

	_, ok := c.Get("a")
	assert.False(t, ok)

	val, ok := c.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, val)
}

func TestCapacityOne_UpdateNoEviction(t *testing.T) {
	c := New(1)
	c.Put("a", 1)
	c.Put("a", 2) // update, no eviction

	val, ok := c.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 2, val)
}

// --- Concurrency ---

func TestConcurrentAccess(t *testing.T) {
	c := New(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Put(fmt.Sprintf("key-%d", i), i)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Get(fmt.Sprintf("key-%d", i))
		}(i)
	}

	wg.Wait()
	// No panic or race = pass
}

func TestConcurrentPutSameKey(t *testing.T) {
	c := New(10)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Put("shared", i)
		}(i)
	}
	wg.Wait()

	val, ok := c.Get("shared")
	assert.True(t, ok)
	assert.NotNil(t, val)
}

// --- Fill exactly to capacity ---

func TestFillExactly(t *testing.T) {
	c := New(5)
	for i := 0; i < 5; i++ {
		c.Put(fmt.Sprintf("k%d", i), i)
	}
	for i := 0; i < 5; i++ {
		val, ok := c.Get(fmt.Sprintf("k%d", i))
		assert.True(t, ok)
		assert.Equal(t, i, val)
	}
}

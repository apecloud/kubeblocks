/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"container/list"
	"sync"
)

type Cache struct {
	capacity int
	m        sync.RWMutex
	list     *list.List
	items    map[string]*list.Element
}

type cacheItem struct {
	key   string
	value any
}

func New(capacity int) *Cache {
	return &Cache{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	if elem, ok := c.items[key]; ok {
		c.list.MoveToFront(elem)
		return elem.Value.(*cacheItem).value, true
	}
	return nil, false
}

func (c *Cache) Put(key string, value any) {
	c.m.Lock()
	defer c.m.Unlock()

	if elem, ok := c.items[key]; ok {
		c.list.MoveToFront(elem)
		elem.Value.(*cacheItem).value = value
		return
	}

	if c.list.Len() == c.capacity {
		last := c.list.Back()
		c.list.Remove(last)
		delete(c.items, last.Value.(*cacheItem).key)
	}

	elem := c.list.PushFront(&cacheItem{key, value})
	c.items[key] = elem
}

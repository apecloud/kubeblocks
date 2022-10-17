/*
Copyright 2022 The KubeBlocks Authors

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

package controllerutil

import (
	"embed"
	"sync"
)

// global cache, assure goroutines safe, eventual consistency
var (
	cacheCtx = sync.Map{}
	//go:embed cue/*
	CueTemplates embed.FS
)

// GetCacheBytesValue Get bytes from cache
func GetCacheBytesValue(key string, valueCreator func() ([]byte, error)) ([]byte, error) {
	vIf, ok := cacheCtx.Load(key)
	if ok {
		return vIf.([]byte), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	cacheCtx.Store(key, v)
	return v, err
}

// GetCacheCUETplValue Get CUETpl from cache
func GetCacheCUETplValue(key string, valueCreator func() (*CUETpl, error)) (*CUETpl, error) {
	vIf, ok := cacheCtx.Load(key)
	if ok {
		return vIf.(*CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	cacheCtx.Store(key, v)
	return v, err
}

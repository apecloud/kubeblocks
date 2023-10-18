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

package tasks

import (
	"fmt"
	"io"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/cache"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/ending"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/module"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/pipeline"

	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type PipelineWrapper struct {
	pipeline.Pipeline
}

func NewPipelineRunner(name string, modules []module.Module, runtime connector.Runtime) *PipelineWrapper {
	return &PipelineWrapper{
		Pipeline: pipeline.Pipeline{
			Name:          name,
			Modules:       modules,
			Runtime:       runtime,
			PipelineCache: cache.NewCache(),
			SpecHosts:     len(runtime.GetAllHosts()),
		},
	}
}

func (w *PipelineWrapper) Do(output io.Writer) error {
	defer func() {
		w.PipelineCache.Clean()
		w.releaseHostsConnector()
	}()

	for i := range w.Modules {
		m := w.Modules[i]
		if m.IsSkip() {
			continue
		}
		if res := w.safeRunModule(m); res.IsFailed() {
			return cfgcore.WrapError(res.CombineResult, "failed to execute module: %s", getModuleName(m))
		}
	}
	fmt.Fprintf(output, "succeed to execute all modules in the pipeline[%s]", w.Name)
	return nil
}

func (w *PipelineWrapper) safeRunModule(m module.Module) *ending.ModuleResult {
	newCache := func() *cache.Cache {
		if moduleCache, ok := w.ModuleCachePool.Get().(*cache.Cache); ok {
			return moduleCache
		}
		return cache.NewCache()
	}
	releaseCache := func(cache *cache.Cache) {
		cache.Clean()
		w.ModuleCachePool.Put(cache)
	}

	moduleCache := newCache()
	defer releaseCache(moduleCache)
	m.Default(w.Runtime, w.PipelineCache, moduleCache)
	m.AutoAssert()
	m.Init()
	return w.RunModule(m)
}

func (w *PipelineWrapper) releaseHostsConnector() {
	for _, host := range w.Runtime.GetAllHosts() {
		if connector := w.Runtime.GetConnector(); connector != nil {
			connector.Close(host)
		}
	}
}

func getModuleName(m module.Module) string {
	if b, ok := m.(*module.BaseModule); ok {
		return b.Name
	}
	return ""
}

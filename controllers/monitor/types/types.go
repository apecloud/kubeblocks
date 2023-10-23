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

package types

import (
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type OTeldParams struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type OteldConfig struct {
	OteldInstanceMap map[v1alpha1.Mode]*OteldInstance
	Exporters        *Exporters
	ConfigGenerator  *OteldConfigGenerater
}

type OteldCfgRef struct {
	Config *OteldConfig
}

// ReconcileCtx wrapper for reconcile procedure context parameters
type ReconcileCtx struct {
	intctrlutil.RequestCtx

	// Ctx       context.Context
	// Req       ctrl.Request
	// Log       logr.Logger
	Config    *Config
	Namespace string

	OteldCfgRef *OteldCfgRef
}

func (c *ReconcileCtx) SetConfigGenerator(cg *OteldConfigGenerater) {
	if c.OteldCfgRef == nil {
		c.OteldCfgRef = &OteldCfgRef{}
	}
	if c.OteldCfgRef.Config == nil {
		c.OteldCfgRef.Config = &OteldConfig{}
	}
	c.OteldCfgRef.Config.ConfigGenerator = cg
}

func (c *ReconcileCtx) GetConfigGenerator() *OteldConfigGenerater {
	if c.OteldCfgRef == nil ||
		c.OteldCfgRef.Config == nil ||
		c.OteldCfgRef.Config.OteldInstanceMap == nil {
		return nil
	}
	return c.OteldCfgRef.Config.ConfigGenerator
}

func (c *ReconcileCtx) SetOteldInstanceMap(oteldInstanceMap map[v1alpha1.Mode]*OteldInstance) {
	if c.OteldCfgRef == nil {
		c.OteldCfgRef = &OteldCfgRef{}
	}
	if c.OteldCfgRef.Config == nil {
		c.OteldCfgRef.Config = &OteldConfig{}
	}
	c.OteldCfgRef.Config.OteldInstanceMap = oteldInstanceMap
}

func (c *ReconcileCtx) GetOteldConfig() (map[v1alpha1.Mode]*OteldInstance, error) {
	if c.OteldCfgRef == nil ||
		c.OteldCfgRef.Config == nil ||
		c.OteldCfgRef.Config.OteldInstanceMap == nil {
		return nil, cfgcore.MakeError("not found oteld config")
	}
	return c.OteldCfgRef.Config.OteldInstanceMap, nil
}

func (c *ReconcileCtx) SetExporters(exporters *Exporters) {
	if c.OteldCfgRef == nil {
		c.OteldCfgRef = &OteldCfgRef{}
	}
	if c.OteldCfgRef.Config == nil {
		c.OteldCfgRef.Config = &OteldConfig{}
	}
	c.OteldCfgRef.Config.Exporters = exporters
}

func (c *ReconcileCtx) GetExporters() *Exporters {
	if c.OteldCfgRef == nil ||
		c.OteldCfgRef.Config == nil ||
		c.OteldCfgRef.Config.Exporters == nil {
		return nil
	}
	return c.OteldCfgRef.Config.Exporters
}

func (c *ReconcileCtx) VerifyOteldInstance(metricsExporterList *v1alpha1.MetricsExporterSinkList, logsExporterList *v1alpha1.LogsExporterSinkList) error {
	metricsMap := make(map[string]bool)
	for _, mExporter := range metricsExporterList.Items {
		metricsMap[string(mExporter.Spec.Type)] = true
	}
	logMap := make(map[string]bool)
	for _, lExporter := range logsExporterList.Items {
		logMap[string(lExporter.Spec.Type)] = true
	}

	for _, instance := range c.OteldCfgRef.Config.OteldInstanceMap {
		if instance.MetricsPipline != nil {
			for _, pipline := range instance.MetricsPipline {
				for key := range pipline.ExporterMap {
					if _, ok := metricsMap[key]; !ok {
						return cfgcore.MakeError("not found exporter %s", key)
					}
				}
			}
		}
		if instance.LogsPipline != nil {
			for _, pipline := range instance.LogsPipline {
				for key := range pipline.ExporterMap {
					if _, ok := logMap[key]; !ok {
						return cfgcore.MakeError("not found exporter %s", key)
					}
				}
			}
		}
	}
	return nil
}

func (c *ReconcileCtx) GetOteldInstance(mode v1alpha1.Mode) *OteldInstance {
	if c.OteldCfgRef == nil || c.OteldCfgRef.Config == nil || c.OteldCfgRef.Config.OteldInstanceMap == nil {
		return nil
	}
	return c.OteldCfgRef.Config.OteldInstanceMap[mode]
}

type ReconcileTask interface {
	Do(reqCtx ReconcileCtx) error
}

type ReconcileFunc func(reqCtx ReconcileCtx) error

func (f ReconcileFunc) Do(reqCtx ReconcileCtx) error {
	return f(reqCtx)
}

type baseTask struct {
	ReconcileFunc
}

var errNilFunc = cfgcore.MakeError("nil reconcile func")

func NewReconcileTask(name string, task ReconcileFunc) ReconcileTask {
	if task == nil {
		// not walk here
		panic(errNilFunc)
	}
	newTask := func(reqCtx ReconcileCtx) error {
		reqCtx = ReconcileCtx{
			RequestCtx: intctrlutil.RequestCtx{
				Ctx: reqCtx.Ctx,
				Req: reqCtx.Req,
				Log: reqCtx.Log.WithValues("subTask", name),
			},
			Config:      reqCtx.Config,
			Namespace:   reqCtx.Namespace,
			OteldCfgRef: reqCtx.OteldCfgRef,
		}
		return task(reqCtx)
	}
	return baseTask{ReconcileFunc: newTask}
}

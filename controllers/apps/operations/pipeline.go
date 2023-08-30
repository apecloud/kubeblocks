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

package operations

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type reconfigureContext struct {
	// reconfiguring request
	config appsv1alpha1.Configuration

	cli      client.Client
	reqCtx   intctrlutil.RequestCtx
	resource *OpsResource

	clusterName   string
	componentName string
}

type pipeline struct {
	err      error
	isFailed bool

	updatedParameters []cfgcore.ParamPairs
	mergedConfig      map[string]string
	configPatch       *cfgcore.ConfigPatchInfo
	isFileUpdated     bool

	configMap        *corev1.ConfigMap
	clusterVer       *appsv1alpha1.ClusterVersion
	clusterDef       *appsv1alpha1.ClusterDefinition
	configConstraint *appsv1alpha1.ConfigConstraint
	configSpec       *appsv1alpha1.ComponentConfigSpec

	reconfigureContext
}

func (p *pipeline) wrapper(fn func() error) (ret *pipeline) {
	ret = p
	if ret.err != nil {
		return
	}
	ret.err = fn()
	return
}

func (p pipeline) foundConfigSpec(name string, configSpecs []appsv1alpha1.ComponentConfigSpec) *appsv1alpha1.ComponentConfigSpec {
	if len(name) == 0 && len(configSpecs) == 1 {
		return &configSpecs[0]
	}
	for _, configSpec := range configSpecs {
		if configSpec.Name == name {
			return &configSpec
		}
	}
	return nil
}

func (p *pipeline) Validate() *pipeline {
	validateFn := func() (err error) {
		var components []appsv1alpha1.ClusterComponentVersion
		var configSpecs []appsv1alpha1.ComponentConfigSpec

		if p.clusterVer != nil {
			components = p.clusterVer.Spec.ComponentVersions
		}
		configSpecs, err = cfgcore.GetConfigTemplatesFromComponent(
			p.resource.Cluster.Spec.ComponentSpecs,
			p.clusterDef.Spec.ComponentDefs,
			components,
			p.componentName)
		if err != nil {
			p.isFailed = true
			return
		}

		configSpec := p.foundConfigSpec(p.config.Name, configSpecs)
		if configSpec != nil {
			p.configSpec = configSpec
			return
		}
		err = cfgcore.MakeError(
			"failed to reconfigure, not existed config[%s], all configs: %v",
			p.config.Name, getConfigSpecName(configSpecs))
		p.isFailed = true
		return
	}

	return p.wrapper(validateFn)
}

func (p *pipeline) ClusterDefinition() *pipeline {
	cdKey := client.ObjectKey{
		Name: p.resource.Cluster.Spec.ClusterDefRef,
	}

	return p.wrapper(func() error {
		p.clusterDef = &appsv1alpha1.ClusterDefinition{}
		return p.cli.Get(p.reqCtx.Ctx, cdKey, p.clusterDef)
	})
}

func (p *pipeline) ClusterVersion() *pipeline {
	cvKey := client.ObjectKey{
		Name: p.resource.Cluster.Spec.ClusterVersionRef,
	}

	return p.wrapper(func() error {
		if cvKey.Name == "" {
			return nil
		}
		p.clusterVer = &appsv1alpha1.ClusterVersion{}
		return p.cli.Get(p.reqCtx.Ctx, cvKey, p.clusterVer)
	})
}

func (p *pipeline) ConfigMap() *pipeline {
	cmKey := client.ObjectKey{
		Name:      cfgcore.GetComponentCfgName(p.clusterName, p.componentName, p.configSpec.Name),
		Namespace: p.resource.Cluster.Namespace,
	}

	return p.wrapper(func() error {
		p.configMap = &corev1.ConfigMap{}
		return p.cli.Get(p.reqCtx.Ctx, cmKey, p.configMap)
	})
}

func (p *pipeline) ConfigConstraints() *pipeline {
	validateFn := func() (err error) {
		if !hasFileUpdate(p.config) {
			p.isFailed = true
			err = cfgcore.MakeError(
				"current configSpec not support reconfigure, configSpec: %v",
				p.configSpec.Name)
		}
		return
	}

	ccKey := client.ObjectKey{
		Name: p.configSpec.ConfigConstraintRef,
	}
	fetchCCFn := func() error {
		p.configConstraint = &appsv1alpha1.ConfigConstraint{}
		return p.cli.Get(p.reqCtx.Ctx, ccKey, p.configConstraint)
	}

	return p.wrapper(func() error {
		if p.configSpec.ConfigConstraintRef == "" {
			return validateFn()
		} else {
			return fetchCCFn()
		}
	})
}

func (p *pipeline) doMerge() error {
	var err error
	var newCfg map[string]string

	cm := p.configMap
	cc := p.configConstraint
	config := p.config

	updatedFiles := make(map[string]string, len(config.Keys))
	updatedParams := make([]cfgcore.ParamPairs, 0, len(config.Keys))
	for _, key := range config.Keys {
		if key.FileContent != "" {
			updatedFiles[key.Key] = key.FileContent
		}
		if len(key.Parameters) > 0 {
			updatedParams = append(updatedParams, cfgcore.ParamPairs{
				Key:           key.Key,
				UpdatedParams: fromKeyValuePair(key.Parameters),
			})
		}
	}

	if newCfg, err = mergeUpdatedParams(cm.Data, updatedFiles, updatedParams, cc, *p.configSpec); err != nil {
		p.isFailed = true
		return err
	}

	p.mergedConfig = newCfg

	// for full update
	if cc == nil {
		p.isFileUpdated = true
		return nil
	}

	// for patch update
	configPatch, restart, err := cfgcore.CreateConfigPatch(cm.Data,
		newCfg,
		cc.Spec.FormatterConfig.Format,
		p.configSpec.Keys,
		len(updatedFiles) != 0)
	if err != nil {
		return err
	}
	p.isFileUpdated = restart
	p.configPatch = configPatch
	p.updatedParameters = updatedParams
	return nil
}

func (p *pipeline) Merge() *pipeline {
	return p.wrapper(p.doMerge)
}

func (p *pipeline) UpdateOpsLabel() *pipeline {
	updateFn := func() error {
		if len(p.updatedParameters) == 0 ||
			p.configConstraint == nil ||
			p.configConstraint.Spec.FormatterConfig == nil {
			return nil
		}

		request := p.resource.OpsRequest
		deepObject := request.DeepCopy()
		formatter := p.configConstraint.Spec.FormatterConfig
		updateOpsLabelWithReconfigure(request, p.updatedParameters, p.configMap.Data, formatter)
		return p.cli.Patch(p.reqCtx.Ctx, request, client.MergeFrom(deepObject))
	}

	return p.wrapper(updateFn)
}

func (p *pipeline) Sync() *pipeline {
	return p.wrapper(func() error {
		var cc *appsv1alpha1.ConfigConstraintSpec
		var configSpec = *p.configSpec

		if p.configConstraint != nil {
			cc = &p.configConstraint.Spec
		}
		return syncConfigmap(p.configMap,
			p.mergedConfig,
			p.cli,
			p.reqCtx.Ctx,
			p.resource.OpsRequest.Name,
			configSpec,
			cc,
			p.config.Policy)
	})
}

func (p *pipeline) Complete() reconfiguringResult {
	if p.err != nil {
		return makeReconfiguringResult(p.err, withFailed(p.isFailed))
	}

	return makeReconfiguringResult(nil,
		withReturned(p.mergedConfig, p.configPatch),
		withNoFormatFilesUpdated(p.isFileUpdated),
	)
}

func newPipeline(context reconfigureContext) *pipeline {
	return &pipeline{reconfigureContext: context}
}

func hasFileUpdate(config appsv1alpha1.Configuration) bool {
	for _, key := range config.Keys {
		if key.FileContent != "" {
			return true
		}
	}
	return false
}

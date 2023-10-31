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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type reconfigureContext struct {
	// reconfiguring request
	config appsv1alpha1.ConfigurationItem

	cli      client.Client
	reqCtx   intctrlutil.RequestCtx
	resource *OpsResource

	clusterName   string
	componentName string
}

type pipeline struct {
	isFailed bool

	updatedParameters []cfgcore.ParamPairs
	mergedConfig      map[string]string
	configPatch       *cfgcore.ConfigPatchInfo
	isFileUpdated     bool

	updatedObject    *appsv1alpha1.Configuration
	configConstraint *appsv1alpha1.ConfigConstraint
	configSpec       *appsv1alpha1.ComponentConfigSpec

	reconfigureContext
	intctrlutil.ResourceFetcher[pipeline]
}

func newPipeline(ctx reconfigureContext) *pipeline {
	pipeline := &pipeline{reconfigureContext: ctx}
	pipeline.Init(&intctrlutil.ResourceCtx{
		Client:        ctx.cli,
		Context:       ctx.reqCtx.Ctx,
		Namespace:     ctx.resource.OpsRequest.Namespace,
		ClusterName:   ctx.clusterName,
		ComponentName: ctx.componentName,
	}, pipeline)
	pipeline.ClusterObj = ctx.resource.Cluster
	return pipeline
}

func (p *pipeline) Validate() *pipeline {
	validateFn := func() error {
		if p.ConfigurationObj == nil {
			return cfgcore.MakeError("failed to found configuration of component[%s] in the cluster[%s]",
				p.reconfigureContext.componentName,
				p.reconfigureContext.clusterName,
			)
		}

		item := p.ConfigurationObj.Spec.GetConfigurationItem(p.config.Name)
		if item == nil || item.ConfigSpec == nil {
			p.isFailed = true
			return cfgcore.MakeError("failed to reconfigure, not existed config[%s]", p.config.Name)
		}

		p.configSpec = item.ConfigSpec
		return nil
	}

	return p.Wrap(validateFn)
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

	fetchCCFn := func() error {
		ccKey := client.ObjectKey{
			Name: p.configSpec.ConfigConstraintRef,
		}
		p.configConstraint = &appsv1alpha1.ConfigConstraint{}
		return p.cli.Get(p.reqCtx.Ctx, ccKey, p.configConstraint)
	}

	return p.Wrap(func() error {
		if p.configSpec.ConfigConstraintRef == "" {
			return validateFn()
		} else {
			return fetchCCFn()
		}
	})
}

func (p *pipeline) doMergeImpl(parameters appsv1alpha1.ConfigurationItem) error {
	newConfigObj := p.ConfigurationObj.DeepCopy()

	item := newConfigObj.Spec.GetConfigurationItem(p.config.Name)
	if item == nil {
		return cfgcore.MakeError("not found config item: %s", parameters.Name)
	}

	configSpec := p.configSpec
	if item.ConfigFileParams == nil {
		item.ConfigFileParams = make(map[string]appsv1alpha1.ConfigParams)
	}
	filter := validate.WithKeySelector(configSpec.Keys)
	for _, key := range parameters.Keys {
		// patch parameters
		if configSpec.ConfigConstraintRef != "" && filter(key.Key) {
			if key.FileContent != "" {
				return cfgcore.MakeError("not allowed to update file content: %s", key.Key)
			}
			updateParameters(item, key.Key, key.Parameters)
			p.updatedParameters = append(p.updatedParameters, cfgcore.ParamPairs{
				Key:           key.Key,
				UpdatedParams: fromKeyValuePair(key.Parameters),
			})
			continue
		}
		// update file content
		if len(key.Parameters) != 0 {
			return cfgcore.MakeError("not allowed to patch parameters: %s", key.Key)
		}
		updateFileContent(item, key.Key, key.FileContent)
		p.isFileUpdated = true
	}
	p.updatedObject = newConfigObj
	return p.createUpdatePatch(item, configSpec)
}

func (p *pipeline) createUpdatePatch(item *appsv1alpha1.ConfigurationItemDetail, configSpec *appsv1alpha1.ComponentConfigSpec) error {
	if p.configConstraint == nil {
		return nil
	}

	updatedData, err := configuration.DoMerge(p.ConfigMapObj.Data, item.ConfigFileParams, p.configConstraint, *configSpec)
	if err != nil {
		p.isFailed = true
		return err
	}
	p.configPatch, _, err = cfgcore.CreateConfigPatch(p.ConfigMapObj.Data,
		updatedData,
		p.configConstraint.Spec.FormatterConfig.Format,
		p.configSpec.Keys,
		false)
	return err
}

func (p *pipeline) doMerge() error {
	if p.ConfigurationObj == nil {
		return cfgcore.MakeError("not found config: %s",
			cfgcore.GenerateComponentConfigurationName(p.clusterName, p.componentName))
	}
	return p.doMergeImpl(p.config)
}

func (p *pipeline) Merge() *pipeline {
	return p.Wrap(p.doMerge)
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
		updateOpsLabelWithReconfigure(request, p.updatedParameters, p.ConfigMapObj.Data, formatter)
		return p.cli.Patch(p.reqCtx.Ctx, request, client.MergeFrom(deepObject))
	}

	return p.Wrap(updateFn)
}

func (p *pipeline) Sync() *pipeline {
	return p.Wrap(func() error {
		return p.Client.Patch(p.reqCtx.Ctx, p.updatedObject, client.MergeFrom(p.ConfigurationObj))
	})
}

func (p *pipeline) Complete() reconfiguringResult {
	if p.Err != nil {
		return makeReconfiguringResult(p.Err, withFailed(p.isFailed))
	}

	return makeReconfiguringResult(nil,
		withReturned(p.mergedConfig, p.configPatch),
		withNoFormatFilesUpdated(p.isFileUpdated),
	)
}

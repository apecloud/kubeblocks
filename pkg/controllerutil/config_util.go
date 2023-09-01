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

package controllerutil

import (
	"context"

	core2 "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"

	"github.com/StudioSol/set"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ConfigEventContext struct {
	Client  client.Client
	ReqCtx  RequestCtx
	Cluster *v1alpha1.Cluster

	ClusterComponent *v1alpha1.ClusterComponentSpec
	Component        *v1alpha1.ClusterComponentDefinition
	ComponentUnits   []appsv1.StatefulSet
	DeploymentUnits  []appsv1.Deployment

	ConfigSpecName   string
	ConfigPatch      *core2.ConfigPatchInfo
	ConfigMap        *corev1.ConfigMap
	ConfigConstraint *v1alpha1.ConfigConstraintSpec

	PolicyStatus core2.PolicyExecStatus
}

type ConfigEventHandler interface {
	Handle(eventContext ConfigEventContext, lastOpsRequest string, phase v1alpha1.OpsPhase, err error) error
}

var ConfigEventHandlerMap = make(map[string]ConfigEventHandler)

// MergeAndValidateConfigs merges and validates configuration files
func MergeAndValidateConfigs(configConstraint v1alpha1.ConfigConstraintSpec, baseConfigs map[string]string, cmKey []string, updatedParams []core2.ParamPairs) (map[string]string, error) {
	var (
		err error
		fc  = configConstraint.FormatterConfig

		newCfg         map[string]string
		configOperator core2.ConfigOperator
		updatedKeys    = util.NewSet()
	)

	cmKeySet := core2.FromCMKeysSelector(cmKey)
	configLoaderOption := core2.CfgOption{
		Type:           core2.CfgCmType,
		Log:            log.FromContext(context.TODO()),
		CfgType:        fc.Format,
		ConfigResource: core2.FromConfigData(baseConfigs, cmKeySet),
	}
	if configOperator, err = core2.NewConfigLoader(configLoaderOption); err != nil {
		return nil, err
	}

	// merge param to config file
	for _, params := range updatedParams {
		if err := configOperator.MergeFrom(params.UpdatedParams, core2.NewCfgOptions(params.Key, core2.WithFormatterConfig(fc))); err != nil {
			return nil, err
		}
		updatedKeys.Add(params.Key)
	}

	if newCfg, err = configOperator.ToCfgContent(); err != nil {
		return nil, core2.WrapError(err, "failed to generate config file")
	}

	// The ToCfgContent interface returns the file contents of all keys, the configuration file is encoded and decoded into keys,
	// the content may be different with the original file, such as comments, blank lines, etc,
	// in order to minimize the impact on the original file, only update the changed part.
	updatedCfg := fromUpdatedConfig(newCfg, updatedKeys)
	if err = validate.NewConfigValidator(&configConstraint, validate.WithKeySelector(cmKey)).Validate(updatedCfg); err != nil {
		return nil, core2.WrapError(err, "failed to validate updated config")
	}
	return core2.MergeUpdatedConfig(baseConfigs, updatedCfg), nil
}

// fromUpdatedConfig filters out changed file contents.
func fromUpdatedConfig(m map[string]string, sets *set.LinkedHashSetString) map[string]string {
	if sets.Length() == 0 {
		return map[string]string{}
	}

	r := make(map[string]string, sets.Length())
	for key, v := range m {
		if sets.InArray(key) {
			r[key] = v
		}
	}
	return r
}

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

package parameters

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

type paramsOps func(params *reconfigureContext)

func withClusterComponent(replicas int) paramsOps {
	return func(params *reconfigureContext) {
		params.ClusterComponent = &appsv1.ClusterComponentSpec{
			Name:     "test",
			Replicas: func() int32 { rep := int32(replicas); return rep }(),
		}
	}
}

func withClusterComponentNConfigs(replicas int, configs []appsv1.ClusterComponentConfig) paramsOps {
	return func(params *reconfigureContext) {
		params.ClusterComponent = &appsv1.ClusterComponentSpec{
			Name:     "test",
			Replicas: func() int32 { rep := int32(replicas); return rep }(),
			Configs:  configs,
		}
	}
}

func withWorkload() paramsOps {
	return func(params *reconfigureContext) {
		params.its = &workloads.InstanceSet{}
	}
}

func withConfigSpec(configSpecName string, data map[string]string) paramsOps {
	return func(params *reconfigureContext) {
		params.ConfigTemplate.Name = configSpecName
		params.VersionHash = computeTargetVersionHash(params.RequestCtx, data)
	}
}

func withConfigDescription(formatter *parametersv1alpha1.FileFormatConfig) paramsOps {
	return func(params *reconfigureContext) {
		params.ConfigDescription = &parametersv1alpha1.ComponentConfigDescription{
			Name:             "for-test",
			FileFormatConfig: formatter,
		}
	}
}

func withUpdatedParameters(patch *core.ConfigPatchInfo) paramsOps {
	return func(params *reconfigureContext) {
		params.Patch = patch
	}
}

func withParamDef(pd *parametersv1alpha1.ParametersDefinitionSpec) paramsOps {
	return func(params *reconfigureContext) {
		params.ParametersDef = pd
	}
}

func newMockReconfigureParams(testName string, cli client.Client, paramOps ...paramsOps) reconfigureContext {
	params := reconfigureContext{
		Client: cli,
		RequestCtx: intctrlutil.RequestCtx{
			Ctx:      ctx,
			Log:      log.FromContext(ctx).WithValues("policy_test", testName),
			Recorder: record.NewFakeRecorder(100),
		},
		SynthesizedComponent: &component.SynthesizedComponent{
			Namespace:   testCtx.DefaultNamespace,
			ClusterName: "test",
			Name:        "test",
			PodSpec: &corev1.PodSpec{
				Containers: []corev1.Container{},
				Volumes: []corev1.Volume{{
					Name: "for_test",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/tmp",
						},
					}}},
			},
			MinReadySeconds: 5,
			Roles: []appsv1.ReplicaRole{
				{
					Name:                 "leader",
					ParticipatesInQuorum: true,
					UpdatePriority:       5,
				},
				{
					Name:                 "follower",
					ParticipatesInQuorum: true,
					UpdatePriority:       4,
				},
			},
		},
		Cluster: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "test",
			}},
		ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{},
	}
	for _, customFn := range paramOps {
		customFn(&params)
	}

	if params.ClusterComponent != nil {
		params.Cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{
			*params.ClusterComponent,
		}
	}
	return params
}

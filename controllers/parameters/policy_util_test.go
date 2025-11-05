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
	"fmt"
	"testing"

	"github.com/sethvargo/go-password/password"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	defaultNamespace = "default"
	itsSchemaKind    = workloads.GroupVersion.WithKind(workloads.InstanceSetKind)
)

func newMockInstanceSet(replicas int, name string, labels map[string]string) workloads.InstanceSet {
	uid, _ := password.Generate(12, 12, 0, true, false)
	return workloads.InstanceSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       workloads.InstanceSetKind,
			APIVersion: workloads.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
			UID:       types.UID(uid),
		},
		Spec: workloads.InstanceSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: func() *int32 { i := int32(replicas); return &i }(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
					Volumes: []corev1.Volume{{
						Name: "for_test",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/tmp",
							},
						}}},
				},
			},
		},
	}
}

func newMockRunningComponent() *appsv1.Component {
	return &appsv1.Component{
		Status: appsv1.ComponentStatus{
			Phase: appsv1.RunningComponentPhase,
		},
	}
}

type ParamsOps func(params *reconfigureContext)

func withMockInstanceSet(replicas int, labels map[string]string) ParamsOps {
	return func(params *reconfigureContext) {
		rand, _ := password.Generate(12, 8, 0, true, false)
		itsName := "test_" + rand
		params.InstanceSetUnits = []workloads.InstanceSet{
			newMockInstanceSet(replicas, itsName, labels),
		}
	}
}

func withClusterComponent(replicas int) ParamsOps {
	return func(params *reconfigureContext) {
		params.ClusterComponent = &appsv1.ClusterComponentSpec{
			Name:     "test",
			Replicas: func() int32 { rep := int32(replicas); return rep }(),
		}
	}
}

func withGRPCClient(clientFactory createReconfigureClient) ParamsOps {
	return func(params *reconfigureContext) {
		params.ReconfigureClientFactory = clientFactory
	}
}

func withConfigSpec(configSpecName string, data map[string]string) ParamsOps {
	return func(params *reconfigureContext) {
		params.ConfigMap = &corev1.ConfigMap{
			Data: data,
		}
		params.ConfigTemplate.Name = configSpecName
	}
}

func withConfigDescription(formatter *parametersv1alpha1.FileFormatConfig) ParamsOps {
	return func(params *reconfigureContext) {
		params.ConfigDescription = &parametersv1alpha1.ComponentConfigDescription{
			Name:             "for-test",
			FileFormatConfig: formatter,
		}
	}
}

func withUpdatedParameters(patch *core.ConfigPatchInfo) ParamsOps {
	return func(params *reconfigureContext) {
		params.Patch = patch
	}
}

func withParamDef(pd *parametersv1alpha1.ParametersDefinitionSpec) ParamsOps {
	return func(params *reconfigureContext) {
		params.ParametersDef = pd
	}
}

func newMockReconfigureParams(testName string, cli client.Client, paramOps ...ParamsOps) reconfigureContext {
	params := reconfigureContext{
		Client: cli,
		RequestCtx: intctrlutil.RequestCtx{
			Ctx:      ctx,
			Log:      log.FromContext(ctx).WithValues("policy_test", testName),
			Recorder: record.NewFakeRecorder(100),
		},
		SynthesizedComponent: &component.SynthesizedComponent{
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
				Name: "test",
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

func newMockPodsWithInstanceSet(its *workloads.InstanceSet, replicas int, options ...PodOptions) []corev1.Pod {
	pods := make([]corev1.Pod, replicas)
	for i := 0; i < replicas; i++ {
		pods[i] = newMockPod(its.Name+"-"+fmt.Sprint(i), &its.Spec.Template.Spec)
		pods[i].OwnerReferences = []metav1.OwnerReference{newControllerRef(its, itsSchemaKind)}
		pods[i].Status.PodIP = "1.1.1.1"
	}
	for _, customFn := range options {
		for i := range pods {
			pod := &pods[i]
			customFn(pod, i)
		}
	}
	return pods
}

func withReadyPod(rMin, rMax int) PodOptions {
	return func(pod *corev1.Pod, index int) {
		if index < rMin || index >= rMax {
			return
		}

		if pod.Status.Conditions == nil {
			pod.Status.Conditions = make([]corev1.PodCondition, 0)
		}

		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		})

		pod.Status.Phase = corev1.PodRunning
	}
}

func fromPodObjectList(pods []corev1.Pod) []runtime.Object {
	objs := make([]runtime.Object, len(pods))
	for i := 0; i < len(pods); i++ {
		objs[i] = &pods[i]
	}
	return objs
}

func newControllerRef(owner client.Object, gvk schema.GroupVersionKind) metav1.OwnerReference {
	bRefFn := func(b bool) *bool { return &b }
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         bRefFn(true),
		BlockOwnerDeletion: bRefFn(false),
	}
}

type PodOptions func(pod *corev1.Pod, index int)

func newMockPod(podName string, podSpec *corev1.PodSpec) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: defaultNamespace,
		},
	}
	pod.Spec = *podSpec.DeepCopy()
	return pod
}

func Test_resolveReloadActionPolicy(t *testing.T) {
	type args struct {
		jsonPatch string
		format    *parametersv1alpha1.FileFormatConfig
		pd        *parametersv1alpha1.ParametersDefinitionSpec
	}
	tests := []struct {
		name    string
		args    args
		want    parametersv1alpha1.ReloadPolicy
		wantErr bool
	}{{
		name: "restart policy",
		args: args{
			jsonPatch: `{"static1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				StaticParameters: []string{
					"static1",
					"static2",
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
					},
				},
			},
		},
		want: parametersv1alpha1.RestartPolicy,
	}, {
		name: "restart and reload policy",
		args: args{
			jsonPatch: `{"static1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
					},
				},
				MergeReloadAndRestart: pointer.Bool(false),
			},
		},
		want: parametersv1alpha1.DynamicReloadAndRestartPolicy,
	}, {
		name: "hot update policy",
		args: args{
			jsonPatch: `{"dynamic1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					AutoTrigger: &parametersv1alpha1.AutoTrigger{},
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
			},
		},
		want: parametersv1alpha1.AsyncDynamicReloadPolicy,
	}, {
		name: "sync reload policy",
		args: args{
			jsonPatch: `{"dynamic1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
						Sync:    pointer.Bool(true),
					},
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
			},
		},
		want: parametersv1alpha1.SyncDynamicReloadPolicy,
	}, {
		name: "async reload policy",
		args: args{
			jsonPatch: `{"dynamic1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
						Sync:    pointer.Bool(false),
					},
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
			},
		},
		want: parametersv1alpha1.AsyncDynamicReloadPolicy,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveReloadActionPolicy(tt.args.jsonPatch, tt.args.format, tt.args.pd)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveReloadActionPolicy(%v, %v, %v)", tt.args.jsonPatch, tt.args.format, tt.args.pd)
			}
			assert.Equalf(t, tt.want, got, "resolveReloadActionPolicy(%v, %v, %v)", tt.args.jsonPatch, tt.args.format, tt.args.pd)
		})
	}
}

/*
Copyright ApeCloud, Inc.

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

package configuration

import (
	"fmt"
	"time"

	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var (
	defaultNamespace = "default"
	stsSchemaKind    = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
)

func newMockStatefulSet(replicas int, name string, labels map[string]string) appsv1.StatefulSet {
	uid, _ := password.Generate(12, 12, 0, true, false)
	serviceName, _ := password.Generate(12, 0, 0, true, false)
	return appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
			UID:       types.UID(uid),
		},
		Spec: appsv1.StatefulSetSpec{
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
			ServiceName: serviceName,
		},
	}
}

type ParamsOps func(params *reconfigureParams)

func withMockStatefulSet(replicas int, labels map[string]string) ParamsOps {
	return func(params *reconfigureParams) {
		rand, _ := password.Generate(12, 8, 0, true, false)
		stsName := "test_" + rand
		params.ComponentUnits = []appsv1.StatefulSet{
			newMockStatefulSet(replicas, stsName, labels),
		}
	}
}

func withClusterComponent(replicas int) ParamsOps {
	return func(params *reconfigureParams) {
		params.ClusterComponent = &appsv1alpha1.ClusterComponentSpec{
			Replicas: func() int32 { rep := int32(replicas); return rep }(),
		}
	}
}

func withGRPCClient(clientFactory createReconfigureClient) ParamsOps {
	return func(params *reconfigureParams) {
		params.ReconfigureClientFactory = clientFactory
	}
}

func withConfigSpec(configSpecName string, data map[string]string) ParamsOps {
	return func(params *reconfigureParams) {
		params.ConfigMap = &corev1.ConfigMap{
			Data: data,
		}
		params.ConfigSpecName = configSpecName
	}
}

func withConfigConstraintSpec(formatter *appsv1alpha1.FormatterConfig) ParamsOps {
	return func(params *reconfigureParams) {
		params.ConfigConstraint = &appsv1alpha1.ConfigConstraintSpec{
			FormatterConfig: formatter,
		}
	}
}

func withConfigPatch(patch map[string]string) ParamsOps {
	mockEmptyData := func(m map[string]string) map[string]string {
		r := make(map[string]string, len(patch))
		for key := range m {
			r[key] = ""
		}
		return r
	}
	transKeyPair := func(pts map[string]string) map[string]interface{} {
		m := make(map[string]interface{}, len(pts))
		for key, value := range pts {
			m[key] = value
		}
		return m
	}
	return func(params *reconfigureParams) {
		cc := params.ConfigConstraint
		newConfigData, _ := cfgcore.MergeAndValidateConfigs(*cc, map[string]string{"for_test": ""}, nil, []cfgcore.ParamPairs{{
			Key:           "for_test",
			UpdatedParams: transKeyPair(patch),
		}})
		configPatch, _, _ := cfgcore.CreateConfigPatch(mockEmptyData(newConfigData), newConfigData, cc.FormatterConfig.Format, nil, false)
		params.ConfigPatch = configPatch
	}
}

func withCDComponent(compType appsv1alpha1.WorkloadType, tpls []appsv1alpha1.ComponentConfigSpec) ParamsOps {
	return func(params *reconfigureParams) {
		params.Component = &appsv1alpha1.ClusterComponentDefinition{
			ConfigSpecs:  tpls,
			WorkloadType: compType,
			Name:         string(compType),
		}
		if compType == appsv1alpha1.Consensus {
			params.Component.ConsensusSpec = &appsv1alpha1.ConsensusSetSpec{
				Leader: appsv1alpha1.ConsensusMember{
					Name: "leader",
				},
				Followers: []appsv1alpha1.ConsensusMember{
					{
						Name: "follower",
					},
				},
			}
		}
	}
}

func newMockReconfigureParams(testName string, cli client.Client, paramOps ...ParamsOps) reconfigureParams {
	params := reconfigureParams{
		Restart: true,
		Client:  cli,
		Ctx: intctrlutil.RequestCtx{
			Ctx: ctx,
			Log: log.FromContext(ctx).WithValues("policy_test", testName),
		},
	}
	for _, customFn := range paramOps {
		customFn(&params)
	}
	return params
}

func newMockPodsWithStatefulSet(sts *appsv1.StatefulSet, replicas int, options ...PodOptions) []corev1.Pod {
	pods := make([]corev1.Pod, replicas)
	for i := 0; i < replicas; i++ {
		pods[i] = newMockPod(sts.Name+"-"+fmt.Sprint(i), &sts.Spec.Template.Spec)
		pods[i].OwnerReferences = []metav1.OwnerReference{newControllerRef(sts, stsSchemaKind)}
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

func withAvailablePod(rMin, rMax int) PodOptions {
	return func(pod *corev1.Pod, index int) {
		if index < rMin || index >= rMax {
			return
		}

		if pod.Status.Conditions == nil {
			pod.Status.Conditions = make([]corev1.PodCondition, 0)
		}

		h, _ := time.ParseDuration("-1h")
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               corev1.PodReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Now().Add(h)),
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

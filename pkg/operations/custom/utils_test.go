/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package custom

import (
	"context"
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

type jobUIDClient struct {
	client.Client
	jobCreates    int
	dryRunCreates int
	lastDryRunJob *batchv1.Job
}

type managedJobOnlyReader struct {
	client.Reader
	jobGets        int
	unexpectedRead int
}

type managedJobServiceAccountErrorReader struct {
	client.Reader
	err error
}

func (r *managedJobOnlyReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*batchv1.Job); !ok {
		r.unexpectedRead++
		return errors.New("dynamic managed Job input read is forbidden after the exact Job exists")
	}
	r.jobGets++
	return r.Reader.Get(ctx, key, obj, opts...)
}

func (r *managedJobOnlyReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	r.unexpectedRead++
	return errors.New("dynamic managed Job input list is forbidden after the exact Job exists")
}

func (r *managedJobServiceAccountErrorReader) Get(ctx context.Context, key client.ObjectKey,
	obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.ServiceAccount); ok {
		return r.err
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func (c *jobUIDClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if job, ok := obj.(*batchv1.Job); ok {
		if len(opts) > 0 {
			c.dryRunCreates++
			c.lastDryRunJob = job.DeepCopy()
		} else {
			c.jobCreates++
		}
		if job.UID == "" {
			job.UID = types.UID("managed-job-uid")
		}
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestCustom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom Ops Action Suite")
}

var _ = Describe("custom ops helpers", func() {
	const (
		namespace = "default"
		cluster   = "cluster"
		compName  = "mysql"
	)

	var (
		scheme *runtime.Scheme
		reqCtx intctrlutil.RequestCtx
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).Should(Succeed())
		Expect(batchv1.AddToScheme(scheme)).Should(Succeed())
		Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
		Expect(opsv1alpha1.AddToScheme(scheme)).Should(Succeed())
		Expect(workloads.AddToScheme(scheme)).Should(Succeed())
		reqCtx = intctrlutil.RequestCtx{Ctx: context.Background()}
	})

	newFakeClient := func(objs ...client.Object) client.Client {
		return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	}

	envByName := func(envs []corev1.EnvVar) map[string]corev1.EnvVar {
		result := map[string]corev1.EnvVar{}
		for i := range envs {
			result[envs[i].Name] = envs[i]
		}
		return result
	}

	newOpsRequest := func() *opsv1alpha1.OpsRequest {
		return &opsv1alpha1.OpsRequest{
			TypeMeta: metav1.TypeMeta{
				APIVersion: opsv1alpha1.GroupVersion.String(),
				Kind:       "OpsRequest",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-ops",
				Namespace: namespace,
				UID:       types.UID("1234567890abcdef"),
			},
			Spec: opsv1alpha1.OpsRequestSpec{
				SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
					CustomOps: &opsv1alpha1.CustomOps{},
				},
			},
		}
	}

	newCluster := func() *appsv1.Cluster {
		return &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: cluster, Namespace: namespace, UID: types.UID("cluster-uid")},
			Spec: appsv1.ClusterSpec{
				ComponentSpecs: []appsv1.ClusterComponentSpec{{
					Name:         compName,
					ComponentDef: "cmpd",
					Replicas:     2,
				}},
			},
		}
	}

	managedSourceOwnerObjects := func(clusterObj *appsv1.Cluster, shardingName string, pod *corev1.Pod) []client.Object {
		shortComponentName := pod.Labels[constant.KBAppComponentLabelKey]
		component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, shortComponentName),
			Namespace: clusterObj.Namespace,
			UID:       types.UID("component-" + pod.Name + "-uid"),
			Labels: map[string]string{
				constant.AppInstanceLabelKey:       clusterObj.Name,
				constant.KBAppComponentLabelKey:    shortComponentName,
				constant.KBAppShardingNameLabelKey: shardingName,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(clusterObj, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
			},
		}}
		instanceSet := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
			UID:       types.UID("instanceset-" + pod.Name + "-uid"),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(component, appsv1.GroupVersion.WithKind(appsv1.ComponentKind)),
			},
		}}
		pod.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(instanceSet, workloads.GroupVersion.WithKind(workloads.InstanceSetKind)),
		}
		return []client.Object{component, instanceSet, pod}
	}

	newComponentSpec := func() *appsv1.ClusterComponentSpec {
		return &appsv1.ClusterComponentSpec{
			Name:           compName,
			ComponentDef:   "cmpd",
			ServiceVersion: "8.0.30",
			Replicas:       2,
		}
	}

	newTargetPod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "node-1",
				Containers: []corev1.Container{{
					Name: "db",
					Env: []corev1.EnvVar{
						{Name: "DIRECT", Value: "direct"},
						{
							Name: "POD_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
							},
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							Prefix: "CFG_",
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "settings"},
							},
						},
						{
							Prefix: "SEC_",
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "creds"},
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
					},
				}},
				Volumes: []corev1.Volume{{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		}
	}

	It("matches component infos by exact, prefix, and regex definitions", func() {
		Expect(getComponentInfo(nil, "mysql")).Should(BeNil())

		opsDef := &opsv1alpha1.OpsDefinition{
			Spec: opsv1alpha1.OpsDefinitionSpec{
				ComponentInfos: []opsv1alpha1.ComponentInfo{
					{ComponentDefinitionName: "postgres"},
					{ComponentDefinitionName: "mysql-8.0"},
					{ComponentDefinitionName: "^redis-[0-9]+$"},
				},
			},
		}
		Expect(getComponentInfo(opsDef, "mysql-8.0.30")).ShouldNot(BeNil())
		Expect(getComponentInfo(opsDef, "redis-7")).ShouldNot(BeNil())
		Expect(getComponentInfo(opsDef, "mongodb")).Should(BeNil())
	})

	It("builds env vars from pod env, envFrom, field paths, and optional missing refs", func() {
		targetPod := newTargetPod("target-0")
		cli := newFakeClient(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: namespace},
				Data:       map[string]string{"port": "3306"},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: namespace},
				Data:       map[string][]byte{"password": []byte("secret")},
			},
		)

		envs, err := buildEnvVars(reqCtx, cli, targetPod, []opsv1alpha1.OpsEnvVar{
			{
				Name: "COPIED_DIRECT",
				ValueFrom: &opsv1alpha1.OpsVarSource{
					EnvVarRef: &opsv1alpha1.EnvVarRef{TargetContainerName: "db", EnvName: "DIRECT"},
				},
			},
			{
				Name: "COPIED_FIELD_REF",
				ValueFrom: &opsv1alpha1.OpsVarSource{
					EnvVarRef: &opsv1alpha1.EnvVarRef{TargetContainerName: "db", EnvName: "POD_NAME"},
				},
			},
			{
				Name: "FROM_CONFIGMAP",
				ValueFrom: &opsv1alpha1.OpsVarSource{
					EnvVarRef: &opsv1alpha1.EnvVarRef{TargetContainerName: "db", EnvName: "CFG_port"},
				},
			},
			{
				Name: "FROM_SECRET",
				ValueFrom: &opsv1alpha1.OpsVarSource{
					EnvVarRef: &opsv1alpha1.EnvVarRef{TargetContainerName: "db", EnvName: "SEC_password"},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &opsv1alpha1.OpsVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: ".metadata.namespace"},
				},
			},
			{
				Name:     "OPTIONAL_MISSING",
				Optional: ptr.To(true),
				ValueFrom: &opsv1alpha1.OpsVarSource{
					EnvVarRef: &opsv1alpha1.EnvVarRef{TargetContainerName: "db", EnvName: "missing"},
				},
			},
		})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(envs).Should(HaveLen(5))
		envMap := envByName(envs)
		Expect(envMap["COPIED_DIRECT"].Value).Should(Equal("direct"))
		Expect(envMap["COPIED_FIELD_REF"].Value).Should(Equal("target-0"))
		Expect(envMap["FROM_CONFIGMAP"].Value).Should(Equal("3306"))
		Expect(envMap["FROM_SECRET"].ValueFrom.SecretKeyRef.Name).Should(Equal("creds"))
		Expect(envMap["FROM_SECRET"].ValueFrom.SecretKeyRef.Key).Should(Equal("password"))
		Expect(envMap["POD_NAMESPACE"].Value).Should(Equal(namespace))
		Expect(envMap).ShouldNot(HaveKey("OPTIONAL_MISSING"))
	})

	It("returns fatal errors for missing required env refs", func() {
		targetPod := newTargetPod("target-0")
		_, err := buildEnvVars(reqCtx, newFakeClient(), targetPod, []opsv1alpha1.OpsEnvVar{{
			Name: "MISSING",
			ValueFrom: &opsv1alpha1.OpsVarSource{
				EnvVarRef: &opsv1alpha1.EnvVarRef{TargetContainerName: "db", EnvName: "missing"},
			},
		}})
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
	})

	It("handles target extractor and field path edge cases", func() {
		opsDef := &opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
			PodInfoExtractors: []opsv1alpha1.PodInfoExtractor{{Name: "target"}},
		}}
		Expect(getTargetPodInfoExtractor(opsDef, "missing")).Should(BeNil())
		extractor, pod, err := getTargetTemplateAndPod(reqCtx.Ctx, newFakeClient(), opsDef, "target", "", namespace)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(extractor).Should(BeNil())
		Expect(pod).Should(BeNil())

		_, err = buildVarWithFieldPath(newTargetPod("target-0"), "[")
		Expect(err).Should(HaveOccurred())
		_, err = buildVarWithFieldPath(newTargetPod("target-0"), ".missing.field")
		Expect(err).Should(HaveOccurred())

		clusterWithToleration := newCluster()
		clusterWithToleration.Spec.SchedulingPolicy = &appsv1.SchedulingPolicy{
			Tolerations: []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}},
		}
		Expect(getTolerations(clusterWithToleration, newComponentSpec())).Should(ContainElement(corev1.Toleration{
			Key:      "dedicated",
			Operator: corev1.TolerationOpExists,
		}))
	})

	It("builds action pod env with built-ins and parameter sources", func() {
		configMapKeyRef := &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "param-cm"},
			Key:                  "statement",
		}
		secretKeyRef := &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "param-secret"},
			Key:                  "password",
		}
		envs, err := buildActionPodEnv(
			reqCtx,
			newFakeClient(),
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			newOpsRequest(),
			newComponentSpec(),
			&opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "logical"},
				Parameters: []opsv1alpha1.Parameter{
					{Name: "SQL", Value: "select 1"},
					{Name: "FROM_CM", ValueFrom: &opsv1alpha1.ParameterSource{ConfigMapKeyRef: configMapKeyRef}},
					{Name: "FROM_SECRET", ValueFrom: &opsv1alpha1.ParameterSource{SecretKeyRef: secretKeyRef}},
					{Name: "EMPTY_VALUE"},
				},
			},
			nil,
			newTargetPod("target-0"),
			false,
		)

		Expect(err).ShouldNot(HaveOccurred())
		envMap := envByName(envs)
		Expect(envMap[KBEnvOpsName].Value).Should(Equal("custom-ops"))
		Expect(envMap[KbEnvOpsNamespace].Value).Should(Equal(namespace))
		Expect(envMap[constant.KBEnvClusterName].Value).Should(Equal(cluster))
		Expect(envMap[constant.KBEnvCompName].Value).Should(Equal(compName))
		Expect(envMap[constant.KBEnvCompReplicas].Value).Should(Equal("2"))
		Expect(envMap["SQL"].Value).Should(Equal("select 1"))
		Expect(envMap["FROM_CM"].ValueFrom.ConfigMapKeyRef).Should(Equal(configMapKeyRef))
		Expect(envMap["FROM_SECRET"].ValueFrom.SecretKeyRef).Should(Equal(secretKeyRef))
		Expect(envMap["EMPTY_VALUE"].Value).Should(BeEmpty())
	})

	It("adds component definition service and account env vars when requested", func() {
		fullCompName := constant.GenerateClusterComponentName(cluster, compName)
		cli := newFakeClient(
			&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{Name: fullCompName, Namespace: namespace},
				Spec:       appsv1.ComponentSpec{CompDef: "cmpd"},
			},
			&appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "cmpd"},
				Spec: appsv1.ComponentDefinitionSpec{
					ServiceVersion: "8.0.30",
					Services: []appsv1.ComponentService{{
						Service: appsv1.Service{
							Name:        "client",
							ServiceName: "mysql",
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{{Name: "mysql-port", Port: 3306}},
							},
						},
					}},
				},
			},
		)
		envs := []corev1.EnvVar{}
		err := buildComponentEnvs(
			reqCtx,
			cli,
			newCluster(),
			&opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
				ComponentInfos: []opsv1alpha1.ComponentInfo{{
					ComponentDefinitionName: "cmpd",
					AccountName:             "root",
					ServiceName:             "client",
				}},
			}},
			&envs,
			newComponentSpec(),
			compName,
		)

		Expect(err).ShouldNot(HaveOccurred())
		envMap := envByName(envs)
		Expect(envMap[kbEnvCompServiceVersion].Value).Should(Equal("8.0.30"))
		Expect(envMap[kbEnvAccountUserName].Value).Should(Equal("root"))
		Expect(envMap[kbEnvAccountPassword].ValueFrom.SecretKeyRef.Name).Should(Equal("cluster-mysql-account-root"))
		Expect(envMap[kbEnvCompSVCName].Value).Should(Equal("cluster-mysql-mysql"))
		Expect(envMap[kbEnvCompSVCPortPrefix+"MYSQL_PORT"].Value).Should(Equal("3306"))
	})

	It("selects sharding target pods by role and availability policy", func() {
		clusterObj := newCluster()
		clusterObj.Spec.Shardings = []appsv1.ClusterSharding{{Name: "shard", Shards: 2}}
		labels := constant.GetClusterLabels(cluster)
		labels[constant.KBAppShardingNameLabelKey] = "shard"
		labels[constant.RoleLabelKey] = "leader"

		availablePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: namespace, Labels: labels},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db"}}},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			},
		}
		unavailablePod := availablePod.DeepCopy()
		unavailablePod.Name = "pod-a"
		unavailablePod.Status.Conditions[0].Status = corev1.ConditionFalse

		pods, err := getTargetPods(
			reqCtx.Ctx,
			newFakeClient(unavailablePod, availablePod),
			clusterObj,
			opsv1alpha1.PodSelector{Role: "leader", MultiPodSelectionPolicy: opsv1alpha1.Any},
			"shard",
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(pods).Should(HaveLen(1))
		Expect(pods[0].Name).Should(Equal("pod-b"))

		pods, err = getTargetPods(
			reqCtx.Ctx,
			newFakeClient(unavailablePod, availablePod),
			clusterObj,
			opsv1alpha1.PodSelector{Role: "leader", MultiPodSelectionPolicy: opsv1alpha1.All},
			"shard",
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(pods).Should(HaveLen(2))
	})

	It("builds workload pod specs with extracted volumes, envs, defaults, and image mappings", func() {
		opsRequest := newOpsRequest()
		targetPod := newTargetPod("target-0")
		cli := newFakeClient(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constant.GenerateDefaultServiceAccountName("cmpd"),
				Namespace: namespace,
			},
		})
		workloadAction := &WorkloadAction{
			OpsRequest: opsRequest,
			Cluster:    newCluster(),
			OpsDef:     &opsv1alpha1.OpsDefinition{},
			CustomCompOps: &opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName},
				Parameters:   []opsv1alpha1.Parameter{{Name: "SQL", Value: "select 1"}},
			},
			Comp: newComponentSpec(),
		}
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: cli,
			Action: &opsv1alpha1.OpsAction{
				Name: "work",
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type: opsv1alpha1.PodWorkload,
					PodSpec: corev1.PodSpec{
						Containers:     []corev1.Container{{Name: "runner", Image: "old"}},
						InitContainers: []corev1.Container{{Name: "init"}},
					},
				},
			},
			Images: map[string]string{"runner": "new"},
		}
		podSpec, err := workloadAction.buildPodSpec(
			actionCtx,
			&opsv1alpha1.PodInfoExtractor{VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}},
			targetPod,
		)

		Expect(err).ShouldNot(HaveOccurred())
		Expect(podSpec.RestartPolicy).Should(Equal(corev1.RestartPolicyNever))
		Expect(podSpec.NodeSelector).Should(HaveKeyWithValue(corev1.LabelHostname, "node-1"))
		Expect(podSpec.ServiceAccountName).Should(Equal(constant.GenerateDefaultServiceAccountName("cmpd")))
		Expect(podSpec.Containers[0].Image).Should(Equal("new"))
		Expect(podSpec.Containers[0].Env).Should(ContainElement(corev1.EnvVar{Name: "SQL", Value: "select 1"}))
		Expect(podSpec.Containers[0].VolumeMounts).Should(ContainElement(corev1.VolumeMount{Name: "data", MountPath: "/data"}))
		Expect(podSpec.Containers[0].VolumeMounts).Should(ContainElement(corev1.VolumeMount{Name: "ops-utils", MountPath: "/scripts"}))
		Expect(podSpec.InitContainers).Should(HaveLen(2))
		Expect(podSpec.Volumes).Should(ContainElement(corev1.Volume{Name: "ops-utils", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}))
	})

	It("builds exec pod specs with kubectl exec args and default container selection", func() {
		execAction := &ExecAction{
			OpsRequest:    newOpsRequest(),
			Cluster:       newCluster(),
			OpsDef:        &opsv1alpha1.OpsDefinition{},
			CustomCompOps: &opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			Comp:          newComponentSpec(),
		}
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(),
			Action: &opsv1alpha1.OpsAction{
				Name: "exec",
				Exec: &opsv1alpha1.OpsExecAction{
					Command: []string{"sh", "-c", "echo ok"},
				},
			},
		}

		podSpec, err := execAction.buildExecPodSpec(actionCtx, nil, newTargetPod("target-0"))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(podSpec.Containers).Should(HaveLen(1))
		Expect(podSpec.Containers[0].Command).Should(Equal([]string{"kubectl"}))
		Expect(podSpec.Containers[0].Args).Should(Equal([]string{
			"-n", namespace, "exec", "target-0", "-c", "db", "--", "sh", "-c", "echo ok",
		}))
		Expect(podSpec.RestartPolicy).Should(BeEmpty())
	})

	It("checks pod action task status and retry paths", func() {
		succeededPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "succeeded", Namespace: namespace},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
		}
		failedPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "failed", Namespace: namespace},
			Status:     corev1.PodStatus{Phase: corev1.PodFailed},
		}
		actionCtx := ActionContext{ReqCtx: reqCtx, Client: newFakeClient(succeededPod, failedPod)}

		completed, failed, err := actionCtx.checkPodTaskStatus(
			&opsv1alpha1.ActionTask{Namespace: namespace, ObjectKey: "Pod/succeeded"},
			1,
			func() error { return nil },
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(completed).Should(BeTrue())
		Expect(failed).Should(BeFalse())

		retryTask := &opsv1alpha1.ActionTask{Namespace: namespace, ObjectKey: "Pod/failed"}
		recreated := false
		completed, failed, err = actionCtx.checkPodTaskStatus(retryTask, 1, func() error {
			recreated = true
			return nil
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(completed).Should(BeFalse())
		Expect(failed).Should(BeFalse())
		Expect(retryTask.Retries).Should(Equal(int32(1)))
		Expect(recreated).Should(BeTrue())

		completed, failed, err = actionCtx.checkPodTaskStatus(
			&opsv1alpha1.ActionTask{Namespace: namespace, ObjectKey: "Pod/failed", Retries: 1},
			1,
			func() error { return nil },
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(completed).Should(BeTrue())
		Expect(failed).Should(BeTrue())
	})

	It("checks aggregate action status and syncs completed task statuses", func() {
		actionCtx := ActionContext{ReqCtx: reqCtx, Client: newFakeClient()}
		status, err := actionCtx.checkActionStatus(
			opsv1alpha1.ProgressStatusDetail{ActionTasks: []opsv1alpha1.ActionTask{
				{Namespace: namespace, ObjectKey: "Pod/one"},
				{Namespace: namespace, ObjectKey: "Pod/two"},
			}},
			func(_ ActionContext, task *opsv1alpha1.ActionTask, index int) (bool, bool, error) {
				return true, index == 1, nil
			},
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.IsCompleted).Should(BeTrue())
		Expect(status.ExistFailure).Should(BeTrue())
		Expect(status.ActionTasks[0].Status).Should(Equal(opsv1alpha1.SucceedActionTaskStatus))
		Expect(status.ActionTasks[1].Status).Should(Equal(opsv1alpha1.FailedActionTaskStatus))
	})

	It("checks job action task status from job conditions", func() {
		workloadAction := &WorkloadAction{OpsRequest: newOpsRequest()}
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{Name: "complete", Namespace: namespace},
					Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					}}},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{Name: "failed", Namespace: namespace},
					Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
						Type:   batchv1.JobFailed,
						Status: corev1.ConditionTrue,
					}}},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{Name: "running", Namespace: namespace},
					Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
						Type:   batchv1.JobFailed,
						Status: corev1.ConditionFalse,
					}}},
				},
			),
		}

		completed, failed, err := workloadAction.checkJobStatus(actionCtx, &opsv1alpha1.ActionTask{
			Namespace: namespace,
			ObjectKey: "Job/complete",
		}, 0)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(completed).Should(BeTrue())
		Expect(failed).Should(BeFalse())

		completed, failed, err = workloadAction.checkJobStatus(actionCtx, &opsv1alpha1.ActionTask{
			Namespace: namespace,
			ObjectKey: "Job/failed",
		}, 0)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(completed).Should(BeTrue())
		Expect(failed).Should(BeTrue())

		completed, failed, err = workloadAction.checkJobStatus(actionCtx, &opsv1alpha1.ActionTask{
			Namespace: namespace,
			ObjectKey: "Job/running",
		}, 0)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(completed).Should(BeFalse())
		Expect(failed).Should(BeFalse())
	})

	It("checks workload action status through pod and job workload switches", func() {
		podAction := NewWorkloadAction(
			newOpsRequest(),
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{ActionTasks: []opsv1alpha1.ActionTask{
				{Namespace: namespace, ObjectKey: "Pod/done", Status: opsv1alpha1.SucceedActionTaskStatus},
				{Namespace: namespace, ObjectKey: "Pod/failed", Status: opsv1alpha1.FailedActionTaskStatus},
			}},
		)
		status, err := podAction.CheckStatus(ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(),
			Action: &opsv1alpha1.OpsAction{
				Workload: &opsv1alpha1.OpsWorkloadAction{Type: opsv1alpha1.PodWorkload},
			},
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.IsCompleted).Should(BeTrue())
		Expect(status.ExistFailure).Should(BeTrue())
		Expect(status.ActionTasks[0].Status).Should(Equal(opsv1alpha1.SucceedActionTaskStatus))
		Expect(status.ActionTasks[1].Status).Should(Equal(opsv1alpha1.FailedActionTaskStatus))

		jobAction := NewWorkloadAction(
			newOpsRequest(),
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{ActionTasks: []opsv1alpha1.ActionTask{
				{Namespace: namespace, ObjectKey: "Job/done", Status: opsv1alpha1.SucceedActionTaskStatus},
			}},
		)
		status, err = jobAction.CheckStatus(ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(),
			Action: &opsv1alpha1.OpsAction{
				Workload: &opsv1alpha1.OpsWorkloadAction{Type: opsv1alpha1.JobWorkload},
			},
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.IsCompleted).Should(BeTrue())
		Expect(status.ExistFailure).Should(BeFalse())
	})

	It("extracts names from action object keys", func() {
		Expect(getNameFromObjectKey("Pod/action-pod")).Should(Equal("action-pod"))
		Expect(getNameFromObjectKey("action-pod")).Should(Equal("action-pod"))
	})

	It("executes workload pod actions without a target pod extractor", func() {
		opsRequest := newOpsRequest()
		workloadAction := NewWorkloadAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName},
				Parameters:   []opsv1alpha1.Parameter{{Name: "SQL", Value: "select 1"}},
			},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(),
			Action: &opsv1alpha1.OpsAction{
				Name: "pod-action",
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type: opsv1alpha1.PodWorkload,
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
					},
				},
			},
		}

		status, err := workloadAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.ActionTasks).Should(HaveLen(1))
		Expect(status.ActionTasks[0].Namespace).Should(Equal(namespace))
		Expect(status.ActionTasks[0].ObjectKey).Should(HavePrefix("Pod/"))
		Expect(status.ActionTasks[0].Status).Should(Equal(opsv1alpha1.ProcessingActionTaskStatus))

		pod := &corev1.Pod{}
		Expect(actionCtx.Client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(status.ActionTasks[0].ObjectKey),
		}, pod)).Should(Succeed())
		Expect(pod.Labels).Should(HaveKeyWithValue(constant.OpsRequestNameLabelKey, opsRequest.Name))
		Expect(pod.Labels).Should(HaveKeyWithValue(KBOpsActionNameLabelKey, "pod-action"))
		Expect(pod.Spec.RestartPolicy).Should(Equal(corev1.RestartPolicyNever))

		duplicateTask, err := workloadAction.createPod(actionCtx, &pod.Spec, "", 0, 0)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(duplicateTask.ObjectKey).Should(Equal(status.ActionTasks[0].ObjectKey))
	})

	It("executes workload job actions and checks invalid workload types", func() {
		opsRequest := newOpsRequest()
		workloadAction := NewWorkloadAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(),
			Action: &opsv1alpha1.OpsAction{
				Name: "job-action",
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type:         opsv1alpha1.JobWorkload,
					BackoffLimit: 2,
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
					},
				},
			},
		}

		status, err := workloadAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.ActionTasks).Should(HaveLen(1))
		Expect(status.ActionTasks[0].ObjectKey).Should(HavePrefix("Job/"))

		job := &batchv1.Job{}
		Expect(actionCtx.Client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(status.ActionTasks[0].ObjectKey),
		}, job)).Should(Succeed())
		Expect(job.Spec.BackoffLimit).Should(Equal(ptr.To[int32](2)))

		actionCtx.Action.Workload.Type = opsv1alpha1.OpsWorkloadType("Unknown")
		_, err = workloadAction.createWorkload(actionCtx, &corev1.PodSpec{}, "", 0)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

		_, err = workloadAction.CheckStatus(actionCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
	})

	It("persists a managed Job plan before dispatch and never recreates a missing bound Job", func() {
		opsRequest := newOpsRequest()
		opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
			OpsDefinitionUID:        "opsdef-uid",
			OpsDefinitionGeneration: 1,
			OpsDefinitionSpecHash:   strings.Repeat("a", 64),
			TargetSnapshotHash:      strings.Repeat("b", 64),
		}
		cli := &jobUIDClient{Client: newFakeClient()}
		action := &opsv1alpha1.OpsAction{
			Name:          "managed-job",
			FailurePolicy: opsv1alpha1.FailurePolicyFail,
			Workload: &opsv1alpha1.OpsWorkloadAction{
				Type:         opsv1alpha1.ManagedJobWorkload,
				BackoffLimit: 0,
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
				},
			},
		}
		workloadAction := NewWorkloadAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName},
				Parameters:   []opsv1alpha1.Parameter{{Name: "TOKEN", Value: "generation-7"}},
			},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{ReqCtx: reqCtx, Client: cli, Reader: cli, Action: action}

		noReaderCtx := actionCtx
		noReaderCtx.Reader = nil
		_, err := workloadAction.Execute(noReaderCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		Expect(cli.dryRunCreates).Should(BeZero())
		Expect(cli.jobCreates).Should(BeZero())

		planned, err := workloadAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(planned.ActionTasks).Should(HaveLen(1))
		Expect(planned.ActionTasks[0].TaskIndex).Should(Equal(ptr.To[int32](0)))
		Expect(planned.ActionTasks[0].DispatchState).Should(Equal(opsv1alpha1.PlannedActionTaskDispatchState))
		Expect(planned.ActionTasks[0].WorkloadUID).Should(BeEmpty())
		Expect(planned.ActionTasks[0].WorkloadSpecHash).Should(MatchRegexp("^[a-f0-9]{64}$"))
		Expect(cli.dryRunCreates).Should(Equal(1))
		Expect(cli.jobCreates).Should(BeZero())
		Expect(cli.List(reqCtx.Ctx, &batchv1.JobList{}, client.InNamespace(namespace))).Should(Succeed())
		jobList := &batchv1.JobList{}
		Expect(cli.List(reqCtx.Ctx, jobList, client.InNamespace(namespace))).Should(Succeed())
		Expect(jobList.Items).Should(BeEmpty())

		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: planned.ActionTasks}
		_, err = workloadAction.CheckStatus(noReaderCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		Expect(cli.dryRunCreates).Should(Equal(1))
		Expect(cli.jobCreates).Should(BeZero())

		unexpectedSourceTasks := append([]opsv1alpha1.ActionTask(nil), planned.ActionTasks...)
		unexpectedSourceTasks[0].TargetPodName = "unexpected"
		unexpectedSourceTasks[0].TargetPodUID = "unexpected-uid"
		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: unexpectedSourceTasks}
		_, err = workloadAction.CheckStatus(actionCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		Expect(cli.jobCreates).Should(BeZero())

		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: planned.ActionTasks}
		created, err := workloadAction.CheckStatus(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(created.IsCompleted).Should(BeFalse())
		Expect(created.ActionTasks).Should(HaveLen(1))
		Expect(created.ActionTasks[0].DispatchState).Should(Equal(opsv1alpha1.CreatedActionTaskDispatchState))
		Expect(created.ActionTasks[0].WorkloadUID).Should(Equal("managed-job-uid"))
		Expect(cli.dryRunCreates).Should(Equal(2))
		Expect(cli.jobCreates).Should(Equal(1))

		job := &batchv1.Job{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(created.ActionTasks[0].ObjectKey),
		}, job)).Should(Succeed())
		Expect(job.Spec.BackoffLimit).Should(Equal(ptr.To[int32](0)))
		selectorValue := job.Spec.Selector.MatchLabels[constant.ManagedJobSelectorLabelKey]
		Expect(selectorValue).Should(MatchRegexp("^[a-z2-7]{52}$"))
		Expect(job.Spec.Template.Labels).Should(HaveKeyWithValue(constant.ManagedJobSelectorLabelKey, selectorValue))
		Expect(opsutil.ManagedJobSelectorValue("12345678-first-complete-uid", 0)).ShouldNot(
			Equal(opsutil.ManagedJobSelectorValue("12345678-second-complete-uid", 0)))

		job.Status.Conditions = []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}}
		Expect(cli.Status().Update(reqCtx.Ctx, job)).Should(Succeed())
		terminalTasks := append([]opsv1alpha1.ActionTask(nil), created.ActionTasks...)
		terminalTasks[0].Status = opsv1alpha1.SucceedActionTaskStatus
		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: terminalTasks}
		verified, err := workloadAction.CheckStatus(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(verified.IsCompleted).Should(BeTrue())
		Expect(verified.ExistFailure).Should(BeFalse())

		terminalTasks[0].Status = opsv1alpha1.FailedActionTaskStatus
		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: terminalTasks}
		_, err = workloadAction.CheckStatus(actionCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

		drifted := job.DeepCopy()
		drifted.Spec.Template.Spec.Containers[0].Image = "busybox:changed"
		Expect(workloadAction.validateManagedJob(&created.ActionTasks[0], drifted, job.Name)).Should(HaveOccurred())
		replacement := job.DeepCopy()
		replacement.UID = types.UID("managed-job-replacement-uid")
		Expect(workloadAction.validateManagedJob(&created.ActionTasks[0], replacement, job.Name)).Should(HaveOccurred())

		job.Finalizers = []string{"test-finalizer"}
		Expect(cli.Update(reqCtx.Ctx, job)).Should(Succeed())
		Expect(cli.Delete(reqCtx.Ctx, job)).Should(Succeed())
		terminating := &batchv1.Job{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(job), terminating)).Should(Succeed())
		Expect(terminating.DeletionTimestamp.IsZero()).Should(BeFalse())
		terminalTasks[0].Status = opsv1alpha1.SucceedActionTaskStatus
		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: terminalTasks}
		_, err = workloadAction.CheckStatus(actionCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		Expect(err.Error()).Should(ContainSubstring("terminating"))

		terminating.Finalizers = nil
		Expect(cli.Update(reqCtx.Ctx, terminating)).Should(Succeed())
		deleteErr := cli.Delete(reqCtx.Ctx, terminating)
		Expect(deleteErr == nil || apierrors.IsNotFound(deleteErr)).Should(BeTrue())

		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: terminalTasks}
		_, err = workloadAction.CheckStatus(actionCtx)
		Expect(err).Should(HaveOccurred())
		Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		jobList = &batchv1.JobList{}
		Expect(cli.List(reqCtx.Ctx, jobList, client.InNamespace(namespace))).Should(Succeed())
		Expect(jobList.Items).Should(BeEmpty())
	})

	It("uses only the direct API reader to select the managed Job default ServiceAccount", func() {
		serviceAccountName := constant.GenerateDefaultServiceAccountName("cmpd")
		serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: serviceAccountName, Namespace: namespace,
		}}
		cases := []struct {
			name       string
			cacheObjs  []client.Object
			apiObjs    []client.Object
			wantSAName string
		}{
			{
				name:       "API has ServiceAccount while cache misses it",
				apiObjs:    []client.Object{serviceAccount},
				wantSAName: serviceAccountName,
			},
			{
				name:      "cache has deleted ServiceAccount while API does not",
				cacheObjs: []client.Object{serviceAccount},
			},
		}
		for _, test := range cases {
			By(test.name)
			opsRequest := newOpsRequest()
			opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
				OpsDefinitionUID:        "opsdef-uid",
				OpsDefinitionGeneration: 1,
				OpsDefinitionSpecHash:   strings.Repeat("a", 64),
				TargetSnapshotHash:      strings.Repeat("b", 64),
			}
			cacheClient := &jobUIDClient{Client: newFakeClient(test.cacheObjs...)}
			workloadAction := NewWorkloadAction(
				opsRequest,
				newCluster(),
				&opsv1alpha1.OpsDefinition{},
				&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
				newComponentSpec(),
				opsv1alpha1.ProgressStatusDetail{},
			)
			action := &opsv1alpha1.OpsAction{
				Name: "managed-job",
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type: opsv1alpha1.ManagedJobWorkload,
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
					},
				},
			}
			_, err := workloadAction.Execute(ActionContext{
				ReqCtx: reqCtx,
				Client: cacheClient,
				Reader: newFakeClient(test.apiObjs...),
				Action: action,
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cacheClient.lastDryRunJob).ShouldNot(BeNil())
			Expect(cacheClient.lastDryRunJob.Spec.Template.Spec.ServiceAccountName).Should(Equal(test.wantSAName))
		}
	})

	It("fails before managed Job dry-run when the direct ServiceAccount read fails", func() {
		opsRequest := newOpsRequest()
		opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
			OpsDefinitionUID:        "opsdef-uid",
			OpsDefinitionGeneration: 1,
			OpsDefinitionSpecHash:   strings.Repeat("a", 64),
			TargetSnapshotHash:      strings.Repeat("b", 64),
		}
		cacheClient := &jobUIDClient{Client: newFakeClient()}
		apiReader := &managedJobServiceAccountErrorReader{
			Reader: newFakeClient(),
			err:    errors.New("temporary API read failure"),
		}
		workloadAction := NewWorkloadAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		_, err := workloadAction.Execute(ActionContext{
			ReqCtx: reqCtx,
			Client: cacheClient,
			Reader: apiReader,
			Action: &opsv1alpha1.OpsAction{
				Name: "managed-job",
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type: opsv1alpha1.ManagedJobWorkload,
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
					},
				},
			},
		})
		Expect(err).Should(MatchError("temporary API read failure"))
		Expect(cacheClient.dryRunCreates).Should(BeZero())
		Expect(cacheClient.jobCreates).Should(BeZero())
	})

	It("persists one exact source Pod before dispatching an extracted managed Job", func() {
		clusterObj := newCluster()
		clusterObj.Spec.ComponentSpecs = nil
		clusterObj.Spec.Shardings = []appsv1.ClusterSharding{{
			Name: "shard",
			Template: appsv1.ClusterComponentSpec{
				Name: "redis", ComponentDef: "cmpd", Replicas: 2,
			},
		}}
		newSourcePod := func(name, uid, componentName string) *corev1.Pod {
			labels := constant.GetClusterLabels(cluster)
			labels[constant.KBAppShardingNameLabelKey] = "shard"
			labels[constant.KBAppComponentLabelKey] = componentName
			return &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: name, Namespace: namespace, UID: types.UID(uid), Labels: labels,
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
					Containers: []corev1.Container{{
						Name: "redis",
						Env: []corev1.EnvVar{
							{
								Name: "REDIS_DEFAULT_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "cluster-shard-default"},
									Key:                  constant.AccountPasswdForSecret,
								}},
							},
							{Name: "TLS_ENABLED", Value: "true"},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "tls",
						VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
							SecretName: "cluster-shard-0-tls-certs",
						}},
					}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type: corev1.PodReady, Status: corev1.ConditionTrue,
					}},
				},
			}
		}
		sourceB := newSourcePod("source-b", "source-b-uid", "shard-1")
		sourceA := newSourcePod("source-a", "source-a-uid", "shard-0")
		spoofed := newSourcePod("source-0-spoofed", "spoofed-uid", "shard-spoofed")
		opsDef := &opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
			PodInfoExtractors: []opsv1alpha1.PodInfoExtractor{{
				Name: "source",
				PodSelector: opsv1alpha1.PodSelector{
					MultiPodSelectionPolicy: opsv1alpha1.Any,
				},
				Env: []opsv1alpha1.OpsEnvVar{
					{
						Name: "REDIS_DEFAULT_PASSWORD",
						ValueFrom: &opsv1alpha1.OpsVarSource{EnvVarRef: &opsv1alpha1.EnvVarRef{
							TargetContainerName: "redis", EnvName: "REDIS_DEFAULT_PASSWORD",
						}},
					},
					{
						Name: "REDIS_TLS_ENABLED",
						ValueFrom: &opsv1alpha1.OpsVarSource{EnvVarRef: &opsv1alpha1.EnvVarRef{
							TargetContainerName: "redis", EnvName: "TLS_ENABLED",
						}},
					},
					{
						Name: "REDIS_SOURCE_POD_NAME",
						ValueFrom: &opsv1alpha1.OpsVarSource{FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						}},
					},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: "tls", MountPath: "/etc/redis/tls", ReadOnly: true}},
			}},
		}}
		action := &opsv1alpha1.OpsAction{
			Name:          "managed-job",
			FailurePolicy: opsv1alpha1.FailurePolicyFail,
			Workload: &opsv1alpha1.OpsWorkloadAction{
				Type:                 opsv1alpha1.ManagedJobWorkload,
				PodInfoExtractorName: "source",
				BackoffLimit:         0,
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "runner", Image: "redis:7"}},
				},
			},
		}
		opsDef.Spec.Actions = []opsv1alpha1.OpsAction{*action}
		opsRequest := newOpsRequest()
		opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
			OpsDefinitionUID:        "opsdef-uid",
			OpsDefinitionGeneration: 1,
			OpsDefinitionSpecHash:   strings.Repeat("a", 64),
			TargetSnapshotHash:      strings.Repeat("b", 64),
		}
		objects := []client.Object{spoofed}
		objects = append(objects, managedSourceOwnerObjects(clusterObj, "shard", sourceB)...)
		objects = append(objects, managedSourceOwnerObjects(clusterObj, "shard", sourceA)...)
		cli := &jobUIDClient{Client: newFakeClient(objects...)}
		workloadAction := NewWorkloadAction(
			opsRequest,
			clusterObj,
			opsDef,
			&opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"},
				Parameters:   []opsv1alpha1.Parameter{{Name: "shardAddToken", Value: "generation-7"}},
			},
			&clusterObj.Spec.Shardings[0].Template,
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{ReqCtx: reqCtx, Client: cli, Reader: cli, Action: action}

		planned, err := workloadAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(planned.ActionTasks).Should(HaveLen(1))
		Expect(planned.ActionTasks[0].TargetPodName).Should(Equal("source-a"))
		Expect(planned.ActionTasks[0].TargetPodUID).Should(Equal("source-a-uid"))
		Expect(planned.ActionTasks[0].DispatchState).Should(Equal(opsv1alpha1.PlannedActionTaskDispatchState))
		Expect(cli.jobCreates).Should(BeZero())

		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: planned.ActionTasks}
		created, err := workloadAction.CheckStatus(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(created.ActionTasks[0].TargetPodName).Should(Equal("source-a"))
		Expect(created.ActionTasks[0].TargetPodUID).Should(Equal("source-a-uid"))
		Expect(cli.jobCreates).Should(Equal(1))

		job := &batchv1.Job{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(created.ActionTasks[0].ObjectKey),
		}, job)).Should(Succeed())
		envMap := envByName(job.Spec.Template.Spec.Containers[0].Env)
		Expect(envMap["REDIS_DEFAULT_PASSWORD"].ValueFrom.SecretKeyRef.Name).Should(Equal("cluster-shard-default"))
		Expect(envMap["REDIS_TLS_ENABLED"].Value).Should(Equal("true"))
		Expect(envMap["REDIS_SOURCE_POD_NAME"].Value).Should(Equal("source-a"))
		Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(sourceA.Spec.Volumes[0]))
		Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(
			corev1.VolumeMount{Name: "tls", MountPath: "/etc/redis/tls", ReadOnly: true}))
		Expect(job.Spec.Template.Spec.NodeSelector).Should(BeEmpty())
	})

	It("fails closed instead of changing an extracted managed Job source Pod", func() {
		type fixture struct {
			source         *corev1.Pod
			alternate      *corev1.Pod
			client         *jobUIDClient
			workloadAction *WorkloadAction
			actionCtx      ActionContext
			planned        *ActionStatus
		}
		newFixture := func() *fixture {
			clusterObj := newCluster()
			clusterObj.Spec.ComponentSpecs = nil
			clusterObj.Spec.Shardings = []appsv1.ClusterSharding{{
				Name: "shard",
				Template: appsv1.ClusterComponentSpec{
					Name: "redis", ComponentDef: "cmpd", Replicas: 2,
				},
			}}
			newSource := func(name, uid, componentName, credential string) *corev1.Pod {
				labels := constant.GetClusterLabels(cluster)
				labels[constant.KBAppShardingNameLabelKey] = "shard"
				labels[constant.KBAppComponentLabelKey] = componentName
				return &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: name, Namespace: namespace, UID: types.UID(uid), Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: "redis",
							Env:  []corev1.EnvVar{{Name: "CREDENTIAL", Value: credential}},
						}},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{{
							Type: corev1.PodReady, Status: corev1.ConditionTrue,
						}},
					},
				}
			}
			source := newSource("source-a", "source-a-uid", "shard-0", "credential-a")
			alternate := newSource("source-b", "source-b-uid", "shard-1", "credential-b")
			opsDef := &opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
				PodInfoExtractors: []opsv1alpha1.PodInfoExtractor{{
					Name: "source",
					PodSelector: opsv1alpha1.PodSelector{
						MultiPodSelectionPolicy: opsv1alpha1.Any,
					},
					Env: []opsv1alpha1.OpsEnvVar{{
						Name: "CREDENTIAL",
						ValueFrom: &opsv1alpha1.OpsVarSource{EnvVarRef: &opsv1alpha1.EnvVarRef{
							TargetContainerName: "redis", EnvName: "CREDENTIAL",
						}},
					}},
					VolumeMounts: []corev1.VolumeMount{{Name: "tls", MountPath: "/etc/pki/tls", ReadOnly: true}},
				}},
			}}
			action := &opsv1alpha1.OpsAction{
				Name:          "managed-job",
				FailurePolicy: opsv1alpha1.FailurePolicyFail,
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type:                 opsv1alpha1.ManagedJobWorkload,
					PodInfoExtractorName: "source",
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "runner", Image: "redis:7"}},
					},
				},
			}
			opsDef.Spec.Actions = []opsv1alpha1.OpsAction{*action}
			opsRequest := newOpsRequest()
			opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
				OpsDefinitionUID:        "opsdef-uid",
				OpsDefinitionGeneration: 1,
				OpsDefinitionSpecHash:   strings.Repeat("a", 64),
				TargetSnapshotHash:      strings.Repeat("b", 64),
			}
			objects := managedSourceOwnerObjects(clusterObj, "shard", alternate)
			objects = append(objects, managedSourceOwnerObjects(clusterObj, "shard", source)...)
			cli := &jobUIDClient{Client: newFakeClient(objects...)}
			workloadAction := NewWorkloadAction(
				opsRequest,
				clusterObj,
				opsDef,
				&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}},
				&clusterObj.Spec.Shardings[0].Template,
				opsv1alpha1.ProgressStatusDetail{},
			)
			actionCtx := ActionContext{ReqCtx: reqCtx, Client: cli, Reader: cli, Action: action}
			planned, err := workloadAction.Execute(actionCtx)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(planned.ActionTasks).Should(HaveLen(1))
			Expect(planned.ActionTasks[0].TargetPodName).Should(Equal(source.Name))
			Expect(planned.ActionTasks[0].TargetPodUID).Should(Equal(string(source.UID)))
			return &fixture{
				source: source, alternate: alternate, client: cli,
				workloadAction: workloadAction, actionCtx: actionCtx, planned: planned,
			}
		}
		dispatch := func(f *fixture) error {
			f.workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: f.planned.ActionTasks}
			_, err := f.workloadAction.CheckStatus(f.actionCtx)
			return err
		}
		assertFatalWithoutJob := func(f *fixture, err error) {
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(f.client.jobCreates).Should(BeZero())
		}

		By("failing when both the planned Job and exact source Pod are absent")
		missing := newFixture()
		Expect(missing.client.Delete(reqCtx.Ctx, missing.source)).Should(Succeed())
		assertFatalWithoutJob(missing, dispatch(missing))

		By("failing instead of accepting a same-name replacement source Pod")
		replaced := newFixture()
		Expect(replaced.client.Delete(reqCtx.Ctx, replaced.source)).Should(Succeed())
		replacement := replaced.source.DeepCopy()
		replacement.ResourceVersion = ""
		replacement.UID = types.UID("replacement-uid")
		Expect(replaced.client.Create(reqCtx.Ctx, replacement)).Should(Succeed())
		assertFatalWithoutJob(replaced, dispatch(replaced))

		By("failing when the persisted source Pod starts terminating")
		terminating := newFixture()
		liveTerminating := &corev1.Pod{}
		Expect(terminating.client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(terminating.source), liveTerminating)).Should(Succeed())
		liveTerminating.Finalizers = []string{"test-finalizer"}
		Expect(terminating.client.Update(reqCtx.Ctx, liveTerminating)).Should(Succeed())
		Expect(terminating.client.Delete(reqCtx.Ctx, liveTerminating)).Should(Succeed())
		assertFatalWithoutJob(terminating, dispatch(terminating))

		By("failing when the persisted source identity tuple is incomplete")
		malformed := newFixture()
		malformed.planned.ActionTasks[0].TargetPodUID = ""
		assertFatalWithoutJob(malformed, dispatch(malformed))

		By("failing when extracted inputs change on the exact source Pod")
		drifted := newFixture()
		liveSource := &corev1.Pod{}
		Expect(drifted.client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(drifted.source), liveSource)).Should(Succeed())
		liveSource.Spec.Containers[0].Env[0].Value = "credential-changed"
		Expect(drifted.client.Update(reqCtx.Ctx, liveSource)).Should(Succeed())
		assertFatalWithoutJob(drifted, dispatch(drifted))

		By("rejecting ConfigMapKeyRef instead of copying its value")
		configMapRef := newFixture()
		liveSource = &corev1.Pod{}
		Expect(configMapRef.client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(configMapRef.source), liveSource)).Should(Succeed())
		liveSource.Spec.Containers[0].Env[0] = corev1.EnvVar{
			Name: "CREDENTIAL",
			ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}, Key: "credential",
			}},
		}
		Expect(configMapRef.client.Update(reqCtx.Ctx, liveSource)).Should(Succeed())
		assertFatalWithoutJob(configMapRef, dispatch(configMapRef))

		By("rejecting envFrom instead of reading source objects")
		envFrom := newFixture()
		liveSource = &corev1.Pod{}
		Expect(envFrom.client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(envFrom.source), liveSource)).Should(Succeed())
		liveSource.Spec.Containers[0].Env = nil
		liveSource.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}},
		}}
		Expect(envFrom.client.Update(reqCtx.Ctx, liveSource)).Should(Succeed())
		assertFatalWithoutJob(envFrom, dispatch(envFrom))

		By("rejecting a non-Secret extracted volume")
		pvcVolume := newFixture()
		liveSource = &corev1.Pod{}
		Expect(pvcVolume.client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvcVolume.source), liveSource)).Should(Succeed())
		liveSource.Spec.Volumes = []corev1.Volume{{
			Name: "tls", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}},
		}}
		Expect(pvcVolume.client.Update(reqCtx.Ctx, liveSource)).Should(Succeed())
		assertFatalWithoutJob(pvcVolume, dispatch(pvcVolume))

		By("binding a response-lost exact Job before consulting a disappeared source Pod")
		responseLost := newFixture()
		responseLost.workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: responseLost.planned.ActionTasks}
		firstDispatch, err := responseLost.workloadAction.CheckStatus(responseLost.actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(firstDispatch.ActionTasks[0].DispatchState).Should(Equal(opsv1alpha1.CreatedActionTaskDispatchState))
		responseLostJob := &batchv1.Job{}
		Expect(responseLost.client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(firstDispatch.ActionTasks[0].ObjectKey),
		}, responseLostJob)).Should(Succeed())
		Expect(responseLostJob.Spec.Template.Spec.NodeSelector).Should(BeEmpty())
		Expect(responseLostJob.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(HaveField("Name", "tls")))
		Expect(responseLostJob.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(HaveField("Name", "tls")))
		initialDryRuns := responseLost.client.dryRunCreates
		initialCreates := responseLost.client.jobCreates
		Expect(responseLost.client.Delete(reqCtx.Ctx, responseLost.source)).Should(Succeed())
		jobOnlyReader := &managedJobOnlyReader{Reader: responseLost.client}
		responseLost.actionCtx.Reader = jobOnlyReader
		responseLost.workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: responseLost.planned.ActionTasks}
		bound, err := responseLost.workloadAction.CheckStatus(responseLost.actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(bound.ActionTasks[0].DispatchState).Should(Equal(opsv1alpha1.CreatedActionTaskDispatchState))
		Expect(bound.ActionTasks[0].WorkloadUID).Should(Equal("managed-job-uid"))
		Expect(responseLost.client.dryRunCreates).Should(Equal(initialDryRuns))
		Expect(responseLost.client.jobCreates).Should(Equal(initialCreates))
		Expect(jobOnlyReader.jobGets).Should(Equal(1))
		Expect(jobOnlyReader.unexpectedRead).Should(BeZero())
	})

	It("binds a matching managed Job after create confirmation is lost without another dry-run or create", func() {
		opsRequest := newOpsRequest()
		opsRequest.Spec.CustomOps.ExecutionSnapshot = &opsv1alpha1.CustomOpsExecutionSnapshot{
			OpsDefinitionUID:        "opsdef-uid",
			OpsDefinitionGeneration: 1,
			OpsDefinitionSpecHash:   strings.Repeat("a", 64),
			TargetSnapshotHash:      strings.Repeat("b", 64),
		}
		cli := &jobUIDClient{Client: newFakeClient()}
		action := &opsv1alpha1.OpsAction{
			Name:          "managed-job",
			FailurePolicy: opsv1alpha1.FailurePolicyFail,
			Workload: &opsv1alpha1.OpsWorkloadAction{
				Type:         opsv1alpha1.ManagedJobWorkload,
				BackoffLimit: 0,
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
				},
			},
		}
		workloadAction := NewWorkloadAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName},
				Parameters:   []opsv1alpha1.Parameter{{Name: "TOKEN", Value: "generation-7"}},
			},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{ReqCtx: reqCtx, Client: cli, Reader: cli, Action: action}

		planned, err := workloadAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cli.dryRunCreates).Should(Equal(1))
		Expect(cli.jobCreates).Should(BeZero())

		// Simulate a successful Create whose Created/UID status update was not observed.
		live, err := workloadAction.buildExpectedManagedJob(actionCtx, 0, nil, nil)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cli.Create(reqCtx.Ctx, live)).Should(Succeed())
		Expect(cli.jobCreates).Should(Equal(1))

		workloadAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: planned.ActionTasks}
		bound, err := workloadAction.CheckStatus(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(bound.ActionTasks).Should(HaveLen(1))
		Expect(bound.ActionTasks[0].DispatchState).Should(Equal(opsv1alpha1.CreatedActionTaskDispatchState))
		Expect(bound.ActionTasks[0].WorkloadUID).Should(Equal("managed-job-uid"))
		Expect(cli.dryRunCreates).Should(Equal(1))
		Expect(cli.jobCreates).Should(Equal(1))
	})

	It("executes workload actions for all selected target pods", func() {
		clusterObj := newCluster()
		clusterObj.Spec.Shardings = []appsv1.ClusterSharding{{
			Name:     "shard",
			Shards:   2,
			Template: *newComponentSpec(),
		}}
		labels := constant.GetClusterLabels(cluster)
		labels[constant.KBAppShardingNameLabelKey] = "shard"
		targetPodA := newTargetPod("target-a")
		targetPodA.Labels = labels
		targetPodA.Labels[constant.KBAppComponentLabelKey] = "shard-0"
		targetPodB := newTargetPod("target-b")
		targetPodB.Labels = labels
		targetPodB.Labels[constant.KBAppComponentLabelKey] = "shard-1"

		opsRequest := newOpsRequest()
		workloadAction := NewWorkloadAction(
			opsRequest,
			clusterObj,
			&opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
				PodInfoExtractors: []opsv1alpha1.PodInfoExtractor{{
					Name: "targets",
					PodSelector: opsv1alpha1.PodSelector{
						MultiPodSelectionPolicy: opsv1alpha1.All,
					},
					VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
				}},
			}},
			&opsv1alpha1.CustomOpsComponent{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"},
				Parameters:   []opsv1alpha1.Parameter{{Name: "SQL", Value: "select 1"}},
			},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(targetPodA, targetPodB),
			Action: &opsv1alpha1.OpsAction{
				Name: "workload-all",
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type:                 opsv1alpha1.PodWorkload,
					PodInfoExtractorName: "targets",
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "runner", Image: "busybox"}},
					},
				},
			},
		}

		status, err := workloadAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.ActionTasks).Should(HaveLen(2))
		Expect(status.ActionTasks[0].TargetPodName).Should(Equal("target-a"))
		Expect(status.ActionTasks[1].TargetPodName).Should(Equal("target-b"))
		for i := range status.ActionTasks {
			pod := &corev1.Pod{}
			Expect(actionCtx.Client.Get(reqCtx.Ctx, client.ObjectKey{
				Namespace: namespace,
				Name:      getNameFromObjectKey(status.ActionTasks[i].ObjectKey),
			}, pod)).Should(Succeed())
			Expect(pod.Spec.NodeSelector).Should(HaveKeyWithValue(corev1.LabelHostname, "node-1"))
			Expect(pod.Spec.Containers[0].Env).Should(ContainElement(corev1.EnvVar{Name: "SQL", Value: "select 1"}))
			Expect(pod.Spec.Containers[0].VolumeMounts).Should(ContainElement(corev1.VolumeMount{Name: "data", MountPath: "/data"}))
		}
	})

	It("executes exec actions against selected sharding pods", func() {
		clusterObj := newCluster()
		clusterObj.Spec.Shardings = []appsv1.ClusterSharding{{
			Name:     "shard",
			Shards:   1,
			Template: *newComponentSpec(),
		}}
		labels := constant.GetClusterLabels(cluster)
		labels[constant.KBAppShardingNameLabelKey] = "shard"
		labels[constant.KBAppComponentLabelKey] = "shard-0"
		targetPod := newTargetPod("target-0")
		targetPod.Labels = labels
		targetPod.Status.Phase = corev1.PodRunning
		targetPod.Status.Conditions = []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}}
		opsRequest := newOpsRequest()
		opsRequest.Spec.CustomOps.ServiceAccountName = ptr.To("custom-sa")

		execAction := NewExecAction(
			opsRequest,
			clusterObj,
			&opsv1alpha1.OpsDefinition{Spec: opsv1alpha1.OpsDefinitionSpec{
				PodInfoExtractors: []opsv1alpha1.PodInfoExtractor{{
					Name: "target",
					PodSelector: opsv1alpha1.PodSelector{
						MultiPodSelectionPolicy: opsv1alpha1.Any,
					},
				}},
			}},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		actionCtx := ActionContext{
			ReqCtx: reqCtx,
			Client: newFakeClient(targetPod),
			Action: &opsv1alpha1.OpsAction{
				Name: "exec-action",
				Exec: &opsv1alpha1.OpsExecAction{
					PodInfoExtractorName: "target",
					Command:              []string{"echo", "ok"},
				},
			},
		}

		status, err := execAction.Execute(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.ActionTasks).Should(HaveLen(1))
		Expect(status.ActionTasks[0].TargetPodName).Should(Equal("target-0"))
		Expect(status.ActionTasks[0].ObjectKey).Should(HavePrefix("Pod/"))

		pod := &corev1.Pod{}
		Expect(actionCtx.Client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(status.ActionTasks[0].ObjectKey),
		}, pod)).Should(Succeed())
		Expect(pod.Spec.ServiceAccountName).Should(Equal("custom-sa"))
		Expect(pod.Spec.Containers[0].Args).Should(Equal([]string{
			"-n", namespace, "exec", "target-0", "-c", "db", "--", "echo", "ok",
		}))

		Expect(actionCtx.Client.Delete(reqCtx.Ctx, pod)).Should(Succeed())
		execAction.progressDetail = opsv1alpha1.ProgressStatusDetail{ActionTasks: status.ActionTasks}
		status, err = execAction.CheckStatus(actionCtx)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status.IsCompleted).Should(BeFalse())
		Expect(status.ActionTasks[0].ObjectKey).Should(HavePrefix("Pod/"))
		Expect(actionCtx.Client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      getNameFromObjectKey(status.ActionTasks[0].ObjectKey),
		}, &corev1.Pod{})).Should(Succeed())
	})

	It("returns nil when action payloads do not match their action type", func() {
		opsRequest := newOpsRequest()
		emptyWorkload := NewWorkloadAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		status, err := emptyWorkload.Execute(ActionContext{ReqCtx: reqCtx, Client: newFakeClient(), Action: &opsv1alpha1.OpsAction{}})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status).Should(BeNil())

		emptyExec := NewExecAction(
			opsRequest,
			newCluster(),
			&opsv1alpha1.OpsDefinition{},
			&opsv1alpha1.CustomOpsComponent{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: compName}},
			newComponentSpec(),
			opsv1alpha1.ProgressStatusDetail{},
		)
		status, err = emptyExec.Execute(ActionContext{ReqCtx: reqCtx, Client: newFakeClient(), Action: &opsv1alpha1.OpsAction{}})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status).Should(BeNil())
	})
})

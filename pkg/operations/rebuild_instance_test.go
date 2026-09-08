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

package operations

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("OpsUtil functions", func() {

	var (
		randomStr            = testCtx.GetRandomStr()
		compDefName          = "test-compdef-" + randomStr
		clusterName          = "test-cluster-" + randomStr
		targetNodeName       = "test-node-1"
		rebuildInstanceCount = 2
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.OpsRequestSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml, client.GracePeriodSeconds(0))
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test Rebuild-Instance opsRequest", func() {
		// Run the real workload revision/status producers against the current Pods.
		// Tests explicitly choose when workload reconciliation catches up with Ops;
		// no hand-written InstanceStatus can make an unallocated name appear ready.
		publishInstanceStatus := func(opsRes *OpsResource) *workloads.InstanceSet {
			comp := opsRes.Cluster.Spec.GetComponentByName(defaultCompName)
			its := &workloads.InstanceSet{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
				Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, defaultCompName)}, its)).To(Succeed())
			its.Spec.Replicas = ptr.To(comp.Replicas)
			its.Spec.FlatInstanceOrdinal = comp.FlatInstanceOrdinal
			its.Spec.OfflineInstances = slices.Clone(comp.OfflineInstances)
			its.Spec.Instances = nil
			for _, template := range comp.Instances {
				its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{
					Name: template.Name, Replicas: template.Replicas,
					Ordinals: workloads.Ordinals{Discrete: template.Ordinals.Discrete},
				})
			}
			Expect(k8sClient.Update(ctx, its)).To(Succeed())
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			pods := &corev1.PodList{}
			Expect(k8sClient.List(ctx, pods, client.InNamespace(its.Namespace), client.MatchingLabels{
				constant.AppInstanceLabelKey: opsRes.Cluster.Name, constant.KBAppComponentLabelKey: defaultCompName,
			})).To(Succeed())
			for i := range pods.Items {
				Expect(tree.Add(&pods.Items[i])).To(Succeed())
			}
			_, err := instanceset.NewRevisionUpdateReconciler().Reconcile(tree)
			Expect(err).NotTo(HaveOccurred())
			_, err = instanceset.NewStatusReconciler().Reconcile(tree)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Status().Update(ctx, its)).To(Succeed())
			return its
		}

		createRebuildInstanceOps := func(backupName string, inPlace bool, instanceNames ...string) *opsv1alpha1.OpsRequest {
			opsName := "rebuild-instance-" + testCtx.GetRandomStr()
			ops := testops.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.RebuildInstanceType)
			var instances []opsv1alpha1.Instance
			for _, insName := range instanceNames {
				instances = append(instances, opsv1alpha1.Instance{
					Name:           insName,
					TargetNodeName: targetNodeName,
				})
			}
			ops.Spec.RebuildFrom = []opsv1alpha1.RebuildInstance{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					Instances:    instances,
					BackupName:   backupName,
					InPlace:      inPlace,
				},
			}
			opsRequest := testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			return opsRequest
		}

		createComponentObject := func(ops *OpsResource) {
			// mock create the component object
			cluster := ops.Cluster
			comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(BeNil())
			Expect(testCtx.CreateObj(ctx, comp)).Should(Succeed())
		}

		prepareOpsRes := func(backupName string, inPlace bool) *OpsResource {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			createComponentObject(opsRes)

			podList := initInstanceSetPods(ctx, k8sClient, opsRes)
			// fake to create the source pvc.
			for i := range podList {
				pvcName := fmt.Sprintf("%s-%s", testapps.DataVolumeName, podList[i].Name)
				pvName := fmt.Sprintf("%s-%s", testapps.DataVolumeName, podList[i].Name)
				testapps.NewPersistentVolumeFactory(podList[i].Namespace, pvName, pvcName).
					SetStorage("20Gi").
					SetPersistentVolumeReclaimPolicy(corev1.PersistentVolumeReclaimDelete).
					Create(&testCtx)
				testapps.NewPersistentVolumeClaimFactory(podList[i].Namespace, pvcName, clusterName, defaultCompName, testapps.DataVolumeName).
					SetStorage("20Gi").SetVolumeName(pvName).Create(&testCtx)
			}

			By("Test the functions in ops_util.go")
			opsRes.OpsRequest = createRebuildInstanceOps(backupName, inPlace, podList[0].Name, podList[1].Name)
			return opsRes
		}

		fakePVCSByRestore := func(opsRequest *opsv1alpha1.OpsRequest) *corev1.PersistentVolumeClaimList {
			pvcList := &corev1.PersistentVolumeClaimList{}
			for i := 0; i < 2; i++ {
				pvcName := fmt.Sprintf("rebuild-%s-%s-%d", opsRequest.UID[:8], common.CutString(defaultCompName+"-"+testapps.DataVolumeName, 30), i)
				pv := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName, defaultCompName, testapps.DataVolumeName).
					AddAnnotations(rebuildFromAnnotation, opsRequest.Name).
					SetStorage("20Gi").Create(&testCtx).GetObject()
				pvcList.Items = append(pvcList.Items, *pv)
			}
			return pvcList
		}

		fakeTmpPVCBoundPV := func(pvcList *corev1.PersistentVolumeClaimList) []*corev1.PersistentVolume {
			var pvs []*corev1.PersistentVolume
			for i := range pvcList.Items {
				pvc := &pvcList.Items[i]
				if _, ok := pvc.Annotations[rebuildFromAnnotation]; !ok {
					// skip the pvc if it is not a tmp pvc.
					continue
				}
				pvName := pvc.Name + "-pv"
				pv := testapps.NewPersistentVolumeFactory(pvc.Namespace, pvc.Name+"-pv", pvc.Name).
					SetStorage("20Gi").
					SetClaimRef(pvc).
					Create(&testCtx).GetObject()
				pvs = append(pvs, pv)
				Expect(testapps.ChangeObj(&testCtx, pvc, func(p *corev1.PersistentVolumeClaim) {
					p.Spec.VolumeName = pvName
				})).Should(Succeed())
			}

			return pvs
		}

		It("fails rebuild OpsRequest when the instance does not exist", func() {
			By("init operations resources")
			opsRes := prepareOpsRes("", true)
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("fake component phase to Failed so the phase gate passes")
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				compStatus := opsRes.Cluster.Status.Components[defaultCompName]
				compStatus.Phase = appsv1.FailedComponentPhase
				opsRes.Cluster.Status.Components[defaultCompName] = compStatus
			})).Should(Succeed())

			By("create a rebuild ops naming an instance with no pod and no retained PVC")
			missingName := fmt.Sprintf("%s-%s-99", clusterName, defaultCompName)
			opsRes.OpsRequest = createRebuildInstanceOps("", true, missingName)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase

			By("expect terminal failure instead of endless retry")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
			Expect(opsRes.OpsRequest.Status.Conditions[0].Message).Should(ContainSubstring(fmt.Sprintf(`instance "%s" not found`, missingName)))
		})

		It("test rebuild instance when cluster/component are mismatched", func() {
			By("init operations resources ")
			opsRes := prepareOpsRes("", true)
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("fake cluster phase to Abnormal and component phase to Running")
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1.AbnormalClusterPhase
			})).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase

			By("expect for opsRequest phase is Failed if the phase of component is not matched")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
			Expect(opsRes.OpsRequest.Status.Conditions[0].Message).Should(ContainSubstring(fmt.Sprintf(`the phase of component "%s" can not be %s`, defaultCompName, appsv1.RunningComponentPhase)))

			By("fake component phase to Failed")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				compStatus := opsRes.Cluster.Status.Components[defaultCompName]
				compStatus.Phase = appsv1.FailedComponentPhase
				opsRes.Cluster.Status.Components[defaultCompName] = compStatus
			})).Should(Succeed())

			By("expect for opsRequest phase is Failed due to the pod is Available")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))

			By("fake pod is unavailable")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase
			for _, ins := range opsRes.OpsRequest.Spec.RebuildFrom[0].Instances {
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.OpsRequest.Namespace}, func(pod *corev1.Pod) {
					pod.Status.Conditions = nil
				})()).Should(Succeed())
			}
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		})

		It("covers in-place rebuild pvc and pv metadata helpers", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			fakeScheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(fakeScheme)).Should(Succeed())
			Expect(opsv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())

			targetPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateClusterComponentName(clusterName, defaultCompName) + "-0", Namespace: testCtx.DefaultNamespace},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "source-pvc"},
						},
					}},
				},
			}
			synthesizedComp := &component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: clusterName,
				Name:        defaultCompName,
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec:       corev1.PersistentVolumeClaimSpec{},
				}},
			}
			opsRequest := &opsv1alpha1.OpsRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: opsv1alpha1.GroupVersion.String(),
					Kind:       "OpsRequest",
				},
				ObjectMeta: metav1.ObjectMeta{Name: "rebuild-ops", Namespace: testCtx.DefaultNamespace, UID: types.UID("12345678abcd")},
			}
			opsRes := &OpsResource{
				Cluster:    &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: testCtx.DefaultNamespace}},
				OpsRequest: opsRequest,
			}

			pvcMap, volumes, volumeMounts, err := getPVCMapAndVolumes(opsRes, synthesizedComp, targetPod, "", "rebuild", 0, false)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pvcMap).Should(HaveKey("source-pvc"))
			Expect(volumes).Should(HaveLen(1))
			Expect(volumeMounts).Should(ContainElement(corev1.VolumeMount{Name: "data", MountPath: "/kb-tmp/0"}))
			Expect(pvcMap["source-pvc"].Annotations).Should(HaveKeyWithValue(rebuildFromAnnotation, opsRequest.Name))

			pvcMap, volumes, volumeMounts, err = getPVCMapAndVolumes(opsRes, synthesizedComp, targetPod, "", "rebuild", 0, true)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pvcMap["source-pvc"].Name).Should(Equal("source-pvc"))
			Expect(volumes).Should(BeEmpty())
			Expect(volumeMounts).Should(BeEmpty())

			templateName, ordinal, err := instancetemplate.GetTemplateNameAndOrdinal(constant.GenerateWorkloadNamePattern(clusterName, defaultCompName), targetPod.Name)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(templateName).Should(BeEmpty())
			Expect(ordinal).Should(Equal(int32(0)))
			templateName, ordinal, err = instancetemplate.GetTemplateNameAndOrdinal("cluster-comp", "cluster-comp-template-3")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(templateName).Should(Equal("template"))
			Expect(ordinal).Should(Equal(int32(3)))
			_, _, err = instancetemplate.GetTemplateNameAndOrdinal("cluster-comp", "cluster-comp-template-")
			Expect(err).Should(HaveOccurred())
			_, _, err = instancetemplate.GetTemplateNameAndOrdinal("cluster-comp", "cluster-comp-template-x")
			Expect(err).Should(HaveOccurred())

			helper := &inplaceRebuildHelper{synthesizedComp: synthesizedComp, targetPod: targetPod}
			tmpPVCForPod := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "tmp-pod-pvc", Namespace: testCtx.DefaultNamespace},
			}
			helper.pvcMap = map[string]*corev1.PersistentVolumeClaim{"source-pvc": tmpPVCForPod}
			helper.volumes = []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: tmpPVCForPod.Name},
				},
			}}
			helper.volumeMounts = []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}
			helper.instance = opsv1alpha1.Instance{TargetNodeName: targetNodeName}
			cli := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			Expect(helper.createTmpPVCsAndPod(reqCtx, cli, opsRequest, "tmp-rebuild-pod")).Should(Succeed())
			Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: tmpPVCForPod.Name, Namespace: tmpPVCForPod.Namespace}, &corev1.PersistentVolumeClaim{})).Should(Succeed())
			createdPod := &corev1.Pod{}
			Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: "tmp-rebuild-pod", Namespace: targetPod.Namespace}, createdPod)).Should(Succeed())
			Expect(createdPod.Spec.RestartPolicy).Should(Equal(corev1.RestartPolicyNever))
			Expect(createdPod.Spec.NodeSelector).Should(HaveKeyWithValue(corev1.LabelHostname, targetNodeName))

			builtTmpPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "tmp-pvc", Namespace: testCtx.DefaultNamespace}}
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			liveTmpPVC, err := helper.getLiveTmpPVCOrBuilt(reqCtx, cli, builtTmpPVC)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(liveTmpPVC).Should(Equal(builtTmpPVC))

			existingTmpPVC := builtTmpPVC.DeepCopy()
			existingTmpPVC.Labels = map[string]string{"live": "true"}
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(existingTmpPVC).Build()
			liveTmpPVC, err = helper.getLiveTmpPVCOrBuilt(reqCtx, cli, builtTmpPVC)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(liveTmpPVC.Labels).Should(HaveKeyWithValue("live", "true"))
			_, err = helper.getSourcePVC(reqCtx, cli, "missing", testCtx.DefaultNamespace)
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())

			sourcePV := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{Name: "source-pv"},
				Spec:       corev1.PersistentVolumeSpec{PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete},
			}
			sourcePVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "source-pvc", Namespace: testCtx.DefaultNamespace, UID: types.UID("source-uid")},
				Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: sourcePV.Name},
			}
			restoredPV := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{Name: "restored-pv"},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
					ClaimRef: &corev1.ObjectReference{
						Name:      builtTmpPVC.Name,
						Namespace: builtTmpPVC.Namespace,
					},
				},
			}
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(sourcePV, sourcePVC, restoredPV, builtTmpPVC).Build()

			foundPV, err := helper.getRestoredPV(reqCtx, cli, &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "tmp-by-volume", Namespace: testCtx.DefaultNamespace},
				Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: restoredPV.Name},
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(foundPV.Name).Should(Equal(restoredPV.Name))

			Expect(helper.retainAndAnnotatePV(reqCtx, cli, opsRequest.Name, restoredPV, builtTmpPVC, sourcePVC)).Should(Succeed())
			Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoredPV.Name}, restoredPV)).Should(Succeed())
			Expect(restoredPV.Spec.PersistentVolumeReclaimPolicy).Should(Equal(corev1.PersistentVolumeReclaimRetain))
			Expect(restoredPV.Labels).Should(HaveKeyWithValue(rebuildTmpPVCNameLabel, builtTmpPVC.Name))
			Expect(restoredPV.Annotations).Should(HaveKeyWithValue(rebuildFromAnnotation, opsRequest.Name))
			Expect(restoredPV.Annotations).Should(HaveKeyWithValue(sourcePVReclaimPolicyAnnotation, string(corev1.PersistentVolumeReclaimDelete)))

			Expect(helper.preBindPVToSourcePVC(reqCtx, cli, restoredPV, builtTmpPVC, sourcePVC.Name, sourcePVC)).Should(Succeed())
			Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoredPV.Name}, restoredPV)).Should(Succeed())
			Expect(restoredPV.Spec.ClaimRef.Name).Should(Equal(sourcePVC.Name))
			Expect(restoredPV.Spec.ClaimRef.UID).Should(BeEmpty())

			activeOwner := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "other-rebuild", Namespace: testCtx.DefaultNamespace},
				Status:     opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase},
			}
			activeSourcePV := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "active-source-pv",
					Annotations: map[string]string{rebuildFromAnnotation: activeOwner.Name},
				},
				Spec: corev1.PersistentVolumeSpec{
					ClaimRef: &corev1.ObjectReference{Name: sourcePVC.Name, Namespace: sourcePVC.Namespace},
				},
			}
			activeSourcePVC := sourcePVC.DeepCopy()
			activeSourcePVC.Spec.VolumeName = activeSourcePV.Name
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(activeOwner, activeSourcePV, activeSourcePVC).Build()
			err = helper.failIfSourcePVCBoundToOtherActiveRebuildPV(reqCtx, cli, opsRequest, activeSourcePVC, restoredPV)
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			activeOwner.Status.Phase = opsv1alpha1.OpsSucceedPhase
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(activeOwner, activeSourcePV, activeSourcePVC).Build()
			Expect(helper.failIfSourcePVCBoundToOtherActiveRebuildPV(reqCtx, cli, opsRequest, activeSourcePVC, restoredPV)).Should(Succeed())

			activeSourcePV.Annotations = nil
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(activeSourcePV, activeSourcePVC).Build()
			Expect(helper.failIfSourcePVCBoundToOtherActiveRebuildPV(reqCtx, cli, opsRequest, activeSourcePVC, restoredPV)).Should(Succeed())

			restoredByLabel := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{Name: "restored-by-label", Labels: map[string]string{rebuildTmpPVCNameLabel: builtTmpPVC.Name}},
			}
			cli = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(restoredByLabel).Build()
			foundPV, err = helper.getRestoredPV(reqCtx, cli, builtTmpPVC)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(foundPV.Name).Should(Equal(restoredByLabel.Name))

			cli = fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			_, err = helper.getRestoredPV(reqCtx, cli, builtTmpPVC)
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		})

		sourcePVCsShouldRebindPVs := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, pvcList *corev1.PersistentVolumeClaimList) {
			// fake the pvs and bound them to the tmp pvcs
			pvs := fakeTmpPVCBoundPV(pvcList)

			sourcePVCTemplateList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, sourcePVCTemplateList,
				client.MatchingLabels{constant.KBAppComponentLabelKey: defaultCompName},
				client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			sourcePVCTemplates := map[string]*corev1.PersistentVolumeClaim{}
			for i := range sourcePVCTemplateList.Items {
				pvc := sourcePVCTemplateList.Items[i].DeepCopy()
				if _, ok := pvc.Annotations[rebuildFromAnnotation]; ok {
					continue
				}
				sourcePVCTemplates[pvc.Name] = pvc
			}
			sourcePVCToPV := map[string]string{}
			sourcePVCToTmpPVC := map[string]string{}
			Eventually(func(g Gomega) {
				_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
				g.Expect(err).Should(Succeed())
				sourcePVCToPV = map[string]string{}
				sourcePVCToTmpPVC = map[string]string{}
				for i := range pvs {
					pv := &corev1.PersistentVolume{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvs[i]), pv)).Should(Succeed())
					g.Expect(pv.Spec.ClaimRef).ShouldNot(BeNil())
					g.Expect(pv.Spec.PersistentVolumeReclaimPolicy).Should(Equal(corev1.PersistentVolumeReclaimRetain))
					g.Expect(pv.Annotations[rebuildFromAnnotation]).Should(Equal(opsRes.OpsRequest.Name))
					sourcePVCToPV[pv.Spec.ClaimRef.Name] = pv.Name
					sourcePVCToTmpPVC[pv.Spec.ClaimRef.Name] = pv.Labels[rebuildTmpPVCNameLabel]
				}
				g.Expect(sourcePVCToPV).Should(HaveLen(rebuildInstanceCount))
			}).Should(Succeed())
			for _, ins := range opsRes.OpsRequest.Spec.RebuildFrom[0].Instances {
				pod := &corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.OpsRequest.Namespace}, pod)
				Expect(client.IgnoreNotFound(err)).Should(Succeed())
				if err == nil {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pod))).Should(Succeed())
				}
			}

			for sourcePVCName := range sourcePVCToPV {
				Eventually(func(g Gomega) {
					pvc := &corev1.PersistentVolumeClaim{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: sourcePVCName, Namespace: opsRes.OpsRequest.Namespace}, pvc)
					g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
				}).Should(Succeed())
			}

			initInstanceSetPods(ctx, k8sClient, opsRes)
			By("expect rebuild to wait while InstanceSet has not recreated the source PVCs")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(Succeed())
			for _, detail := range opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails {
				Expect(detail.Message).Should(Equal("Waiting for source PVCs to bind restored PVs"))
			}

			for sourcePVCName := range sourcePVCToPV {
				sourcePVCTemplate := sourcePVCTemplates[sourcePVCName]
				Expect(sourcePVCTemplate).ShouldNot(BeNil())
				recreatedSourcePVC := sourcePVCTemplate.DeepCopy()
				recreatedSourcePVC.ResourceVersion = ""
				recreatedSourcePVC.UID = ""
				recreatedSourcePVC.CreationTimestamp = metav1.Time{}
				recreatedSourcePVC.DeletionTimestamp = nil
				recreatedSourcePVC.ManagedFields = nil
				recreatedSourcePVC.Finalizers = nil
				recreatedSourcePVC.Spec.VolumeName = ""
				Expect(k8sClient.Create(ctx, recreatedSourcePVC)).Should(Succeed())
			}

			Eventually(func(g Gomega) {
				_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
				g.Expect(err).Should(Succeed())
				for sourcePVCName, pvName := range sourcePVCToPV {
					pvc := &corev1.PersistentVolumeClaim{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: sourcePVCName, Namespace: opsRes.OpsRequest.Namespace}, pvc)).Should(Succeed())
					g.Expect(pvc.Spec.VolumeName).Should(Equal(pvName))
					pv := &corev1.PersistentVolume{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvName}, pv)).Should(Succeed())
					g.Expect(pv.Spec.ClaimRef).ShouldNot(BeNil())
					g.Expect(pv.Spec.ClaimRef.Name).Should(Equal(sourcePVCName))
					g.Expect(pv.Spec.ClaimRef.UID).Should(Equal(pvc.UID))
				}
			}).Should(Succeed())

			By("mock the source pvcs are bound to the restored pvs")
			for sourcePVCName, pvName := range sourcePVCToPV {
				pvc := &corev1.PersistentVolumeClaim{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: sourcePVCName, Namespace: opsRes.OpsRequest.Namespace}, pvc)).Should(Succeed())
				Expect(testapps.ChangeObj(&testCtx, pvc, func(p *corev1.PersistentVolumeClaim) {
					p.Spec.VolumeName = pvName
				})).Should(Succeed())
				Expect(testapps.ChangeObjStatus(&testCtx, pvc, func() {
					pvc.Status.Phase = corev1.ClaimBound
				})).Should(Succeed())
				pv := &corev1.PersistentVolume{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvName}, pv)).Should(Succeed())
				Expect(testapps.ChangeObjStatus(&testCtx, pv, func() {
					pv.Status.Phase = corev1.VolumeBound
				})).Should(Succeed())
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(Succeed())

			Expect(k8sClient.List(ctx, pvcList, client.MatchingLabels{constant.KBAppComponentLabelKey: defaultCompName}, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			reCreatePVCCount := 0
			for i := range pvcList.Items {
				pvc := &pvcList.Items[i]
				pvName, ok := sourcePVCToPV[pvc.Name]
				if !ok {
					continue
				}
				reCreatePVCCount += 1
				Expect(pvc.Spec.VolumeName).Should(Equal(pvName))
				Expect(testapps.ChangeObj(&testCtx, pvc, func(p *corev1.PersistentVolumeClaim) {
					if p.Labels == nil {
						p.Labels = map[string]string{}
					}
					p.Labels[testCtx.TestObjLabelKey] = "true"
					p.Finalizers = nil
				})).Should(Succeed())
			}
			Expect(reCreatePVCCount).Should(Equal(rebuildInstanceCount))
			By("expect to revert the reclaim policy to Delete")
			for i := range pvs {
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pvs[i]), func(g Gomega, pv *corev1.PersistentVolume) {
					g.Expect(pv.Spec.PersistentVolumeReclaimPolicy).Should(Equal(corev1.PersistentVolumeReclaimDelete))
					g.Expect(pv.Annotations).ShouldNot(HaveKey(rebuildFromAnnotation))
					g.Expect(pv.Annotations).ShouldNot(HaveKey(sourcePVReclaimPolicyAnnotation))
					g.Expect(pv.Labels).ShouldNot(HaveKey(rebuildTmpPVCNameLabel))
				}))
			}
			for _, tmpPVCName := range sourcePVCToTmpPVC {
				Eventually(func(g Gomega) {
					tmpPVC := &corev1.PersistentVolumeClaim{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: tmpPVCName, Namespace: opsRes.OpsRequest.Namespace}, tmpPVC)
					g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
				}).Should(Succeed())
			}
		}

		sourcePVCsShouldDynamicReprovision := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			sourcePVCTemplateList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, sourcePVCTemplateList,
				client.MatchingLabels{constant.KBAppComponentLabelKey: defaultCompName},
				client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			sourcePVCTemplates := map[string]*corev1.PersistentVolumeClaim{}
			targetSourcePVCs := map[string]bool{}
			for _, ins := range opsRes.OpsRequest.Spec.RebuildFrom[0].Instances {
				targetSourcePVCs[fmt.Sprintf("%s-%s", testapps.DataVolumeName, ins.Name)] = true
			}
			for i := range sourcePVCTemplateList.Items {
				pvc := sourcePVCTemplateList.Items[i].DeepCopy()
				if _, ok := pvc.Annotations[rebuildFromAnnotation]; ok {
					continue
				}
				if !targetSourcePVCs[pvc.Name] {
					continue
				}
				Expect(testapps.ChangeObj(&testCtx, pvc, func(p *corev1.PersistentVolumeClaim) {
					p.Finalizers = append(p.Finalizers, "kubernetes.io/pvc-protection")
				})).Should(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).Should(Succeed())
				sourcePVCTemplates[pvc.Name] = pvc
			}
			initialSourcePVCNames := make([]string, 0, len(sourcePVCTemplates))
			for sourcePVCName := range sourcePVCTemplates {
				initialSourcePVCNames = append(initialSourcePVCNames, sourcePVCName)
			}
			slices.Sort(initialSourcePVCNames)
			preDeletingSourcePVCName := initialSourcePVCNames[0]
			Expect(k8sClient.Delete(ctx, sourcePVCTemplates[preDeletingSourcePVCName])).Should(Succeed())

			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(Succeed())
			By("expect source PVC deletion to be requested before deleting the old target pod")
			for sourcePVCName := range sourcePVCTemplates {
				Eventually(func(g Gomega) {
					pvc := &corev1.PersistentVolumeClaim{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: sourcePVCName, Namespace: opsRes.OpsRequest.Namespace}, pvc)
					if apierrors.IsNotFound(err) {
						return
					}
					g.Expect(err).Should(Succeed())
					g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
					g.Expect(pvc.Finalizers).Should(ContainElement("kubernetes.io/pvc-protection"))
				}).Should(Succeed())
			}
			for _, ins := range opsRes.OpsRequest.Spec.RebuildFrom[0].Instances {
				Eventually(func(g Gomega) {
					pod := &corev1.Pod{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.OpsRequest.Namespace}, pod)
					g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
				}).Should(Succeed())
			}

			By("expect old source PVCs to keep PVC protection until the workload releases them")
			for sourcePVCName := range sourcePVCTemplates {
				Eventually(func(g Gomega) {
					pvc := &corev1.PersistentVolumeClaim{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: sourcePVCName, Namespace: opsRes.OpsRequest.Namespace}, pvc)).Should(Succeed())
					g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
					g.Expect(pvc.Finalizers).Should(ContainElement("kubernetes.io/pvc-protection"))
				}).Should(Succeed())
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true,
					client.InNamespace(opsRes.OpsRequest.Namespace), client.MatchingFields{"metadata.name": sourcePVCName})
			}

			By("expect rebuild to wait for the rebuilt instance")
			for _, detail := range opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails {
				Expect(detail.Message).Should(Equal(waitingForInstanceReadyMessage))
			}
		}

		waitForInstanceToAvailable := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, ignoreRoleCheck bool) {
			By("waiting for the rebuild instance to ready")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				compStatus := ops.Status.Components[defaultCompName]
				g.Expect(compStatus.ProgressDetails[0].Message).Should(Equal(waitingForInstanceReadyMessage))
				g.Expect(compStatus.ProgressDetails[1].Message).Should(Equal(waitingForInstanceReadyMessage))
			}))

			By("fake th rebuild pods to ready ")
			// recreate the instances and fake it to ready.
			pods := initInstanceSetPods(ctx, k8sClient, opsRes)
			if ignoreRoleCheck {
				for i := range pods {
					Expect(testapps.ChangeObj(&testCtx, pods[i], func(pod *corev1.Pod) {
						if pod.Labels != nil {
							delete(pod.Labels, constant.RoleLabelKey)
						}
					})).Should(Succeed())
				}
			}
		}

		It("test rebuild instance with no backup", func() {
			By("init operations resources ")
			opsRes := prepareOpsRes("", true)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			publishInstanceStatus(opsRes)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			matchingLabels := client.MatchingLabels{
				constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
				constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
			}

			By("expect to dynamically reprovision the source pvcs without helper tmp resources.")
			sourcePVCsShouldDynamicReprovision(reqCtx, opsRes)

			By("expect no-backup rebuild to skip tmp pod/pvc creation")
			podList := &corev1.PodList{}
			Expect(k8sClient.List(ctx, podList, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			Expect(podList.Items).Should(BeEmpty())
			pvcList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, pvcList, client.MatchingLabels{constant.KBAppComponentLabelKey: defaultCompName}, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			tmpPVCCount := 0
			for i := range pvcList.Items {
				if _, ok := pvcList.Items[i].Annotations[rebuildFromAnnotation]; ok {
					tmpPVCCount += 1
				}
			}
			Expect(tmpPVCCount).Should(BeZero())

			By("expect the opsRequest to succeed")
			waitForInstanceToAvailable(reqCtx, opsRes, false)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
			}))

			By("expect no tmp pods were left behind")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testapps.List(&testCtx, generics.PodSignature, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(HaveLen(0))

			By("check its' schedule once annotation")
			podPrefix := constant.GenerateWorkloadNamePattern(clusterName, defaultCompName)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(its), func(g Gomega, its *workloads.InstanceSet) {
				mapping, err := instanceset.ParseNodeSelectorOnceAnnotation(its)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(mapping).To(HaveKeyWithValue(podPrefix+"-0", targetNodeName))
				g.Expect(mapping).To(HaveKeyWithValue(podPrefix+"-1", targetNodeName))
			})).Should(Succeed())
		})

		It("pre-binds restored PV claimRef to the source PVC", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			sourcePVCName := "data-rebuild-source-" + testCtx.GetRandomStr()
			tmpPVC := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, "rebuild-tmp-"+testCtx.GetRandomStr(), clusterName, defaultCompName, testapps.DataVolumeName).
				SetStorage("20Gi").
				Create(&testCtx).
				GetObject()
			sourcePVC := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, sourcePVCName, clusterName, defaultCompName, testapps.DataVolumeName).
				SetStorage("20Gi").
				Create(&testCtx).
				GetObject()
			pv := testapps.NewPersistentVolumeFactory(tmpPVC.Namespace, "restored-pv-"+testCtx.GetRandomStr(), tmpPVC.Name).
				SetStorage("20Gi").
				SetClaimRef(tmpPVC).
				Create(&testCtx).
				GetObject()

			helper := &inplaceRebuildHelper{}
			Expect(helper.preBindPVToSourcePVC(reqCtx, k8sClient, pv, tmpPVC, sourcePVCName, sourcePVC)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pv), pv)).Should(Succeed())
			Expect(pv.Spec.ClaimRef.Name).Should(Equal(sourcePVCName))
			Expect(pv.Spec.ClaimRef.Namespace).Should(Equal(testCtx.DefaultNamespace))
			Expect(pv.Spec.ClaimRef.UID).Should(Equal(sourcePVC.UID))

			wrongBoundSourcePVC := sourcePVC.DeepCopy()
			wrongBoundSourcePVC.Spec.VolumeName = "old-source-pv"
			Expect(helper.preBindPVToSourcePVC(reqCtx, k8sClient, pv, tmpPVC, sourcePVCName, wrongBoundSourcePVC)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pv), pv)).Should(Succeed())
			Expect(pv.Spec.ClaimRef.Name).Should(Equal(sourcePVCName))
			Expect(pv.Spec.ClaimRef.Namespace).Should(Equal(testCtx.DefaultNamespace))
			Expect(pv.Spec.ClaimRef.UID).Should(BeEmpty())

			recreatedSourcePVC := sourcePVC.DeepCopy()
			recreatedSourcePVC.UID = types.UID("recreated-source-pvc")
			Expect(helper.preBindPVToSourcePVC(reqCtx, k8sClient, pv, tmpPVC, sourcePVCName, recreatedSourcePVC)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pv), pv)).Should(Succeed())
			Expect(pv.Spec.ClaimRef.UID).Should(Equal(recreatedSourcePVC.UID))
		})

		It("sets volumeName on an unbound replacement source PVC", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			sourcePVC := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, "data-rebuild-unbound-"+testCtx.GetRandomStr(), clusterName, defaultCompName, testapps.DataVolumeName).
				SetStorage("20Gi").
				Create(&testCtx).
				GetObject()

			helper := &inplaceRebuildHelper{}
			Expect(helper.setSourcePVCVolumeNameForRebuild(reqCtx, k8sClient, sourcePVC, "restored-pv-"+testCtx.GetRandomStr())).Should(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sourcePVC), sourcePVC)).Should(Succeed())
			Expect(sourcePVC.Spec.VolumeName).Should(ContainSubstring("restored-pv-"))
		})

		It("preserves rebuild identity metadata when reverting the reclaim policy", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRequestName := "rebuild-reclaim-revert-" + testCtx.GetRandomStr()
			tmpPVCName := "rebuild-tmp-" + testCtx.GetRandomStr()
			pvName := "restored-pv-" + testCtx.GetRandomStr()
			pv := testapps.NewPersistentVolumeFactory(testCtx.DefaultNamespace, pvName, tmpPVCName).
				SetStorage("20Gi").
				Create(&testCtx).
				GetObject()
			Expect(testapps.ChangeObj(&testCtx, pv, func(p *corev1.PersistentVolume) {
				if p.Labels == nil {
					p.Labels = map[string]string{}
				}
				p.Labels[rebuildTmpPVCNameLabel] = tmpPVCName
				if p.Annotations == nil {
					p.Annotations = map[string]string{}
				}
				p.Annotations[rebuildFromAnnotation] = opsRequestName
				p.Annotations[sourcePVReclaimPolicyAnnotation] = string(corev1.PersistentVolumeReclaimDelete)
				p.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, pv, func() {
				pv.Status.Phase = corev1.VolumeBound
			})).Should(Succeed())

			helper := &inplaceRebuildHelper{}
			Expect(helper.revertReclaimPolicy(reqCtx, k8sClient, pv)).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pv), func(g Gomega, p *corev1.PersistentVolume) {
				// Reclaim policy must be reverted to the value recorded in
				// sourcePVReclaimPolicyAnnotation.
				g.Expect(p.Spec.PersistentVolumeReclaimPolicy).Should(Equal(corev1.PersistentVolumeReclaimDelete))
				// Rebuild identity metadata must be preserved so a later
				// re-entry can still resolve the restored PV by its tmp
				// PVC label even after the tmp PVC itself has been cleaned
				// up.
				g.Expect(p.Labels).Should(HaveKeyWithValue(rebuildTmpPVCNameLabel, tmpPVCName))
				g.Expect(p.Annotations).Should(HaveKeyWithValue(rebuildFromAnnotation, opsRequestName))
				g.Expect(p.Annotations).Should(HaveKeyWithValue(sourcePVReclaimPolicyAnnotation, string(corev1.PersistentVolumeReclaimDelete)))
			})).Should(Succeed())
		})

		testRebuildInstanceWithBackup := func(ignoreRoleCheck bool) {
			By("init operation resources and backup")
			actionSet := testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))
			backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, testdp.BackupName).
				SetBackupPolicyName(testdp.BackupPolicyName).
				SetBackupMethod(testdp.BackupMethodName).
				AddLabels(dptypes.BackupTypeLabelKey, string(dpv1alpha1.BackupTypeFull)).
				Create(&testCtx).GetObject()
			// fake backup is completed
			Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
				backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
				backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
					Name:          backup.Spec.BackupMethod,
					ActionSetName: actionSet.Name,
					TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
						VolumeMounts: []corev1.VolumeMount{
							{Name: testapps.DataVolumeName, MountPath: "/test"},
						},
					},
				}
			})).Should(Succeed())
			opsRes := prepareOpsRes(backup.Name, true)
			_ = testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			publishInstanceStatus(opsRes)
			if ignoreRoleCheck {
				Expect(testapps.ChangeObj(&testCtx, opsRes.OpsRequest, func(request *opsv1alpha1.OpsRequest) {
					if request.Annotations == nil {
						request.Annotations = map[string]string{}
					}
					request.Annotations[ignoreRoleCheckAnnotationKey] = "true"
				})).Should(Succeed())
			}
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("expect for the prepareData Restore CR has been created.")
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			matchingLabels := client.MatchingLabels{
				constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
				constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
			}
			restoreList := &dpv1alpha1.RestoreList{}
			Expect(k8sClient.List(ctx, restoreList, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			Expect(restoreList.Items).Should(HaveLen(rebuildInstanceCount))

			By("fake to create the pvcs which should be created by Restore Controller and change restore phase to Completed")
			// create the pvcs
			pvcList := fakePVCSByRestore(opsRes.OpsRequest)
			fakeRestoresToCompleted := func() {
				// fake restores to Completed
				for i := range restoreList.Items {
					restore := &restoreList.Items[i]
					Expect(testapps.ChangeObjStatus(&testCtx, restore, func() {
						restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
					})).Should(Succeed())
				}
			}
			fakeRestoresToCompleted()

			By("expect to create the source pvcs and the pvs have rebind them.")
			sourcePVCsShouldRebindPVs(reqCtx, opsRes, pvcList)

			By("expect to create the postReady restore after the instances are available")
			waitForInstanceToAvailable(reqCtx, opsRes, ignoreRoleCheck)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(k8sClient.List(ctx, restoreList, matchingLabels, client.InNamespace(opsRes.OpsRequest.Namespace))).Should(Succeed())
			// The number of restores should be twice the number of instances that need to be restored.
			Expect(restoreList.Items).Should(HaveLen(rebuildInstanceCount * 2))

			By("fake the postReady restores to Completed and expect the opsRequest to Succeed.")
			fakeRestoresToCompleted()
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
			}))
		}

		It("test rebuild instance with backup", func() {
			testRebuildInstanceWithBackup(false)
		})

		It("test rebuild instance with backup and ignore role check", func() {
			testRebuildInstanceWithBackup(true)
		})

		DescribeTable("rebuilds from the published allocation without predicting replacement names", func(flat bool) {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			comp := &opsRes.Cluster.Spec.ComponentSpecs[0]
			comp.FlatInstanceOrdinal = flat
			comp.Instances = []appsv1.InstanceTemplate{{Name: "large", Replicas: ptr.To(int32(1))}}
			Expect(k8sClient.Update(ctx, opsRes.Cluster)).To(Succeed())
			createComponentObject(opsRes)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			prefix := its.Name
			original, healthy, named := prefix+"-0", prefix+"-1", prefix+"-large-0"
			if flat {
				original, healthy, named = prefix+"-7", prefix+"-9", prefix+"-42"
				// Existing owner allocation: the names deliberately do not encode templates.
				its.Status.AssignedOrdinals = map[string]workloads.Ordinals{
					"": {Discrete: []int32{7, 9}}, "large": {Discrete: []int32{42}},
				}
				Expect(k8sClient.Status().Update(ctx, its)).To(Succeed())
			}
			testapps.MockInstanceSetPod(&testCtx, its, clusterName, defaultCompName, healthy, "leader")
			oldPod := testapps.MockInstanceSetPod(&testCtx, its, clusterName, defaultCompName, named, "follower")
			Expect(testapps.ChangeObjStatus(&testCtx, oldPod, func() { oldPod.Status.Conditions = nil })).To(Succeed())
			// The first requested instance has no Pod, but its retained volume still exists.
			pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, "data-"+original,
				clusterName, defaultCompName, "data").SetStorage("1Gi").Create(&testCtx).GetObject()
			ops := testops.NewOpsRequestObj("rebuild-allocated-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.RebuildInstanceType)
			ops.Spec.Force = true
			ops.Spec.RebuildFrom = []opsv1alpha1.RebuildInstance{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				Instances:    []opsv1alpha1.Instance{{Name: named}, {Name: original}},
			}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			opsRes.OpsRequest.Status.Progress = "0/2"
			opsRes.Runtimes = map[string]OpsRuntime{defaultCompName: newOpsRuntime(ctx, k8sClient, "")}
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			handler := rebuildInstanceOpsHandler{}

			By("waiting for source publication instead of falling back to names or PVC labels")
			err := handler.SaveLastConfiguration(reqCtx, k8sClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).To(BeTrue())
			its = publishInstanceStatus(opsRes)
			Expect(its.FindInstanceStatus(original).CurrentState).To(Equal(workloads.InstanceCurrentStateAbsent))
			// An unrelated offline identity with unknown template must not block rebuild.
			its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
				PodName: prefix + "-99", DesiredState: workloads.InstanceDesiredStateOffline,
				CurrentState: workloads.InstanceCurrentStateAbsent,
			})
			Expect(k8sClient.Status().Update(ctx, its)).To(Succeed())
			Expect(handler.SaveLastConfiguration(reqCtx, k8sClient, opsRes)).To(Succeed())
			Expect(handler.Action(reqCtx, k8sClient, opsRes)).To(Succeed())
			Expect(opsRes.OpsRequest.Status.LastConfiguration.Components[defaultCompName].SourceInstanceAssignments).To(HaveLen(3))
			Expect(k8sClient.Status().Update(ctx, opsRes.OpsRequest)).To(Succeed())

			reconcile := func() opsv1alpha1.OpsPhase {
				phase, _, err := handler.ReconcileAction(reqCtx, k8sClient, opsRes)
				Expect(err).NotTo(HaveOccurred())
				// Each iteration resumes from persisted state, as after a controller restart.
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).To(Succeed())
				comp = &opsRes.Cluster.Spec.ComponentSpecs[0]
				return phase
			}
			By("writing expansion once and waiting across repeated reconciles with old status")
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(comp.Replicas).To(Equal(int32(5)))
			for i := 0; i < 2; i++ {
				Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase))
				Expect(comp.Replicas).To(Equal(int32(5)))
				Expect(opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails).To(BeEmpty())
			}
			By("letting the actual allocator publish replacements before any new Pod exists")
			its = publishInstanceStatus(opsRes)
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase))
			progress := opsRes.OpsRequest.Status.Components[defaultCompName]
			Expect(progress.ProgressDetails).To(HaveLen(2))
			replacements := map[string]string{}
			for _, name := range []string{original, named} {
				detail := handler.getInstanceProgressDetail(progress, name)
				replacement := handler.getScalingOutPodNameFromMessage(detail.Message)
				Expect(replacement).NotTo(BeEmpty())
				Expect(replacement).NotTo(BeElementOf(original, named, healthy))
				Expect(its.FindInstanceStatus(replacement).CurrentState).To(Equal(workloads.InstanceCurrentStateAbsent))
				expectedTemplate := ""
				if name == named {
					expectedTemplate = "large"
				}
				Expect(*its.FindInstanceStatus(replacement).TemplateName).To(Equal(expectedTemplate))
				replacements[name] = replacement
			}
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(comp.OfflineInstances).To(BeEmpty())
			Expect(opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails).To(Equal(progress.ProgressDetails))

			By("checking actual Pod availability, not the identity or UpToDate fields")
			var newPods []*corev1.Pod
			for _, replacement := range replacements {
				pod := testapps.MockInstanceSetPod(&testCtx, its, clusterName, defaultCompName, replacement, "follower")
				Expect(testapps.ChangeObjStatus(&testCtx, pod, func() { pod.Status.Conditions = nil })).To(Succeed())
				newPods = append(newPods, pod)
			}
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(comp.OfflineInstances).To(BeEmpty())
			for _, pod := range newPods {
				Expect(testapps.ChangeObjStatus(&testCtx, pod, func() { testk8s.MockPodAvailable(pod, metav1.Now()) })).To(Succeed())
			}
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(comp.Replicas).To(Equal(int32(3)))
			Expect(comp.Instances[0].GetReplicas()).To(Equal(int32(1)))
			Expect(comp.OfflineInstances).To(ConsistOf(original, named))
			publishInstanceStatus(opsRes)
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsRunningPhase)) // old named Pod still exists
			testk8s.MockPodIsTerminating(ctx, testCtx, oldPod)
			testk8s.RemovePodFinalizer(ctx, testCtx, oldPod)
			publishInstanceStatus(opsRes)
			Expect(reconcile()).To(Equal(opsv1alpha1.OpsSucceedPhase))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})).To(Succeed())
		}, Entry("flat ordinals", true), Entry("non-flat ordinals", false))

		It("uses the published template for an in-place flat instance and its restored PVC labels", func() {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			comp := &opsRes.Cluster.Spec.ComponentSpecs[0]
			comp.FlatInstanceOrdinal = true
			comp.Instances = []appsv1.InstanceTemplate{{Name: "large", Replicas: ptr.To(int32(1))}}
			Expect(k8sClient.Update(ctx, opsRes.Cluster)).To(Succeed())
			createComponentObject(opsRes)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			its.Status.AssignedOrdinals = map[string]workloads.Ordinals{
				"": {Discrete: []int32{8, 9}}, "large": {Discrete: []int32{7}},
			}
			Expect(k8sClient.Status().Update(ctx, its)).To(Succeed())
			pod := testapps.MockInstanceSetPod(&testCtx, its, clusterName, defaultCompName, its.Name+"-7", "follower")
			opsRes.OpsRequest = createRebuildInstanceOps("", true, pod.Name)
			opsRes.Runtimes = map[string]OpsRuntime{defaultCompName: newOpsRuntime(ctx, k8sClient, "")}
			handler := rebuildInstanceOpsHandler{}
			rebuild := opsRes.OpsRequest.Spec.RebuildFrom[0]
			_, err := handler.prepareInplaceRebuildHelper(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, opsRes, rebuild, rebuild.Instances[0], 0)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting)).To(BeTrue())
			its = publishInstanceStatus(opsRes)
			Expect(*its.FindInstanceStatus(pod.Name).TemplateName).To(Equal("large"))
			helper, err := handler.prepareInplaceRebuildHelper(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, opsRes, rebuild, rebuild.Instances[0], 0)
			Expect(err).NotTo(HaveOccurred())
			template, err := instanceTemplateByName(its.Status.InstanceStatus, pod.Name)
			Expect(err).NotTo(HaveOccurred())
			pvcs, _, _, err := getPVCMapAndVolumes(opsRes, helper.synthesizedComp, pod, template, "rebuild", 0, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).NotTo(BeEmpty())
			for _, pvc := range pvcs {
				Expect(pvc.Labels[constant.KBAppInstanceTemplateLabelKey]).To(Equal("large"))
			}
		})

		It("rejects flat replacement placement before waiting for status or mutating resources", func() {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			opsRes.Cluster.Spec.ComponentSpecs[0].FlatInstanceOrdinal = true
			opsRes.OpsRequest = createRebuildInstanceOps("", false, clusterName+"-mysql-7")
			beforeCluster, beforeOps := opsRes.Cluster.DeepCopy(), opsRes.OpsRequest.DeepCopy()
			handler := rebuildInstanceOpsHandler{}
			for _, action := range []func(intctrlutil.RequestCtx, client.Client, *OpsResource) error{handler.SaveLastConfiguration, handler.Action} {
				err := action(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, opsRes)
				Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("targetNodeName"))
				Expect(opsRes.Cluster).To(Equal(beforeCluster))
				Expect(opsRes.OpsRequest).To(Equal(beforeOps))
			}
			opsRes.OpsRequest.Spec.RebuildFrom[0].InPlace = true
			Expect(handler.SaveLastConfiguration(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, opsRes)).To(Succeed())
			Expect(opsRes.OpsRequest.Status.LastConfiguration.Components[defaultCompName].SourceInstanceAssignments).To(BeEmpty())
		})

		It("rebuild instance with horizontal scaling", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			createComponentObject(opsRes)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			podList := testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)
			publishInstanceStatus(opsRes)
			opsRes.OpsRequest = createRebuildInstanceOps("", false, podList[1].Name, podList[2].Name)

			By("mock cluster/component phase to Failed")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				compStatus := opsRes.Cluster.Status.Components[defaultCompName]
				compStatus.Phase = appsv1.FailedComponentPhase
				opsRes.Cluster.Status.Phase = appsv1.FailedClusterPhase
				opsRes.Cluster.Status.Components[defaultCompName] = compStatus
			})).Should(Succeed())

			By("mock pods are available")
			for i := range podList {
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: podList[i].Name, Namespace: opsRes.Cluster.Namespace}, func(pod *corev1.Pod) {
					pod.Status.Conditions = nil
				})()).Should(Succeed())
			}

			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("save last configuration")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)

			By("expect opsRequest is failed when not existing available pod")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
			Expect(opsRes.OpsRequest.Status.Conditions[2].Message).Should(ContainSubstring("Due to insufficient available instances"))

			By("mock the leader pod is available")
			Expect(testapps.ChangeObjStatus(&testCtx, podList[0], func() {
				testk8s.MockPodAvailable(podList[0], metav1.Now())
			})).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("expect to scale out two replicas ")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(opsRes.Cluster.Spec.GetComponentByName(defaultCompName).Replicas).Should(BeEquivalentTo(5))

			By("its have expected nodeSelector")
			podPrefix := constant.GenerateWorkloadNamePattern(clusterName, defaultCompName)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(its), func(g Gomega, its *workloads.InstanceSet) {
				mapping, err := instanceset.ParseNodeSelectorOnceAnnotation(its)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(mapping).To(HaveKeyWithValue(podPrefix+"-3", targetNodeName))
				g.Expect(mapping).To(HaveKeyWithValue(podPrefix+"-4", targetNodeName))
			})).Should(Succeed())

			By("mock the new pods to available")
			testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, podPrefix+"-3", "follower")
			testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, podPrefix+"-4", "follower")

			By("expect specified instances to take offline")
			publishInstanceStatus(opsRes)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			compSpec := opsRes.Cluster.Spec.GetComponentByName(defaultCompName)
			Expect(compSpec.Replicas).Should(BeEquivalentTo(3))
			Expect(slices.Contains(compSpec.OfflineInstances, podList[1].Name)).Should(BeTrue())
			Expect(slices.Contains(compSpec.OfflineInstances, podList[2].Name)).Should(BeTrue())

			By("delete the pods and expect opsRequest is succeed")
			testk8s.MockPodIsTerminating(ctx, testCtx, podList[1])
			testk8s.RemovePodFinalizer(ctx, testCtx, podList[1])
			testk8s.MockPodIsTerminating(ctx, testCtx, podList[2])
			testk8s.RemovePodFinalizer(ctx, testCtx, podList[2])
			publishInstanceStatus(opsRes)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

	})
})

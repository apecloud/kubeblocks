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

package backup

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func newRequestTestFixture(t *testing.T) (*Request, *corev1.Pod) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns", Labels: map[string]string{constant.RoleLabelKey: "leader"}},
		Spec: corev1.PodSpec{
			NodeName: "node-0",
			Containers: []corev1.Container{{
				Name:  "db",
				Env:   []corev1.EnvVar{{Name: "EXISTING", Value: "old"}},
				Ports: []corev1.ContainerPort{{Name: "mysql", ContainerPort: 3306}},
			}},
			Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}}},
		},
	}
	req := &Request{
		RequestCtx: ctrlutil.RequestCtx{Ctx: context.Background(), Req: ctrl.Request{}},
		Client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup", Namespace: "ns", UID: types.UID("1234567890abcdef"),
				Labels: map[string]string{dptypes.ClusterUIDLabelKey: "uid", constant.AppInstanceLabelKey: "cluster", constant.KBAppComponentLabelKey: "mysql"},
			},
			Spec: dpv1alpha1.BackupSpec{RetentionPeriod: "7d"},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{Spec: dpv1alpha1.BackupPolicySpec{BackoffLimit: func() *int32 { v := int32(1); return &v }(), PathPrefix: "policy-path"}},
		BackupRepo:   &dpv1alpha1.BackupRepo{Spec: dpv1alpha1.BackupRepoSpec{PathPrefix: "repo-path"}, Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"}},
		BackupMethod: &dpv1alpha1.BackupMethod{Env: []corev1.EnvVar{{Name: "EXISTING", Value: "method"}}},
		Target: &dpv1alpha1.BackupTarget{
			Name:        "target",
			PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll},
		},
		TargetPods:           []*corev1.Pod{pod},
		WorkerServiceAccount: "worker",
	}
	return req, pod
}

func TestRequestGetBackupType(t *testing.T) {
	req, _ := newRequestTestFixture(t)
	assert.Empty(t, req.GetBackupType())

	req.BackupMethod.SnapshotVolumes = func() *bool { v := true; return &v }()
	assert.Equal(t, string(dpv1alpha1.BackupTypeFull), req.GetBackupType())

	req.ActionSet = &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeIncremental}}
	assert.Equal(t, string(dpv1alpha1.BackupTypeIncremental), req.GetBackupType())
}

func TestRequestBuildActionBranches(t *testing.T) {
	oldNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	oldServiceAccount := viper.GetString(dptypes.CfgKeyExecWorkerServiceAccountName)
	defer func() {
		viper.Set(constant.CfgKeyCtrlrMgrNS, oldNamespace)
		viper.Set(dptypes.CfgKeyExecWorkerServiceAccountName, oldServiceAccount)
	}()
	viper.Set(constant.CfgKeyCtrlrMgrNS, "kb-system")
	viper.Set(dptypes.CfgKeyExecWorkerServiceAccountName, "exec-worker")

	req, pod := newRequestTestFixture(t)
	_, err := req.buildAction(pod, "invalid", &dpv1alpha1.ActionSpec{})
	assert.Error(t, err)
	_, err = req.buildAction(pod, "invalid", &dpv1alpha1.ActionSpec{Exec: &dpv1alpha1.ExecActionSpec{}, Job: &dpv1alpha1.JobActionSpec{}})
	assert.Error(t, err)

	execAction, err := req.buildAction(pod, "exec", &dpv1alpha1.ActionSpec{Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"echo", "ok"}}})
	assert.NoError(t, err)
	assert.IsType(t, &action.ExecAction{}, execAction)
	assert.Equal(t, "exec", execAction.GetName())

	jobAction, err := req.buildAction(pod, "job", &dpv1alpha1.ActionSpec{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "busybox:$(EXISTING)", Command: []string{"backup"}}}})
	assert.NoError(t, err)
	assert.IsType(t, &action.JobAction{}, jobAction)
	assert.Equal(t, dpv1alpha1.ActionTypeJob, jobAction.Type())
}

func TestRequestBuildBackupDataActions(t *testing.T) {
	req, pod := newRequestTestFixture(t)
	req.ActionSet = &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull, Backup: &dpv1alpha1.BackupActionSpec{BackupData: &dpv1alpha1.BackupDataActionSpec{JobActionSpec: dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "busybox"}}}}}}
	backupDataAction, err := req.buildBackupDataAction(pod, "backup-data")
	assert.NoError(t, err)
	assert.IsType(t, &action.JobAction{}, backupDataAction)

	req.ActionSet.Spec.BackupType = dpv1alpha1.BackupTypeContinuous
	backupDataAction, err = req.buildBackupDataAction(pod, "continuous")
	assert.NoError(t, err)
	assert.IsType(t, &action.StatefulSetAction{}, backupDataAction)
	assert.Contains(t, req.buildContinuousSyncProgressCommand(), "retryTimes")

	req.ActionSet.Spec.BackupType = dpv1alpha1.BackupType("Unknown")
	_, err = req.buildBackupDataAction(pod, "unsupported")
	assert.Error(t, err)
}

func TestRequestBuildActionsIncludesPreAndPostHooks(t *testing.T) {
	oldNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	defer viper.Set(constant.CfgKeyCtrlrMgrNS, oldNamespace)
	viper.Set(constant.CfgKeyCtrlrMgrNS, "kb-system")

	req, pod := newRequestTestFixture(t)
	req.ActionSet = &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{
		BackupType: dpv1alpha1.BackupTypeFull,
		Backup: &dpv1alpha1.BackupActionSpec{
			PreBackup:  []dpv1alpha1.ActionSpec{{Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"pre"}}}},
			BackupData: &dpv1alpha1.BackupDataActionSpec{JobActionSpec: dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "busybox", Command: []string{"backup"}}}},
			PostBackup: []dpv1alpha1.ActionSpec{{Job: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "busybox", Command: []string{"post"}}}}},
		},
	}}

	actions, err := req.BuildActions()
	assert.NoError(t, err)
	assert.Len(t, actions[pod.Name], 3)
	assert.Equal(t, dpv1alpha1.ActionTypeJob, actions[pod.Name][0].Type())
	assert.Equal(t, dpv1alpha1.ActionTypeJob, actions[pod.Name][1].Type())
	assert.Equal(t, dpv1alpha1.ActionTypeJob, actions[pod.Name][2].Type())
	for i, act := range actions[pod.Name] {
		objectRef := act.BuildObjectRef()
		assert.NotNil(t, objectRef)
		assert.Equal(t, batchv1.SchemeGroupVersion.String(), objectRef.APIVersion)
		assert.Equal(t, constant.JobKind, objectRef.Kind)
		assert.Equal(t, GenerateBackupJobName(req.Backup, act.GetName()), objectRef.Name)
		if i == 0 {
			assert.Equal(t, "kb-system", objectRef.Namespace)
		} else {
			assert.Equal(t, req.Namespace, objectRef.Namespace)
		}
	}
}

var _ = Describe("Request Test", func() {
	buildRequest := func() *Request {
		return &Request{
			RequestCtx: ctrlutil.RequestCtx{
				Log:      logger,
				Ctx:      testCtx.Ctx,
				Recorder: recorder,
			},
			Client: testCtx.Cli,
		}
	}

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// namespaced
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
	}

	var clusterInfo *testdp.BackupClusterInfo

	BeforeEach(func() {
		cleanEnv()
		clusterInfo = testdp.NewFakeCluster(&testCtx)
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("with default settings", func() {
		var (
			backup       *dpv1alpha1.Backup
			actionSet    *dpv1alpha1.ActionSet
			backupPolicy *dpv1alpha1.BackupPolicy
			backupRepo   *dpv1alpha1.BackupRepo

			targetPod *corev1.Pod
			request   *Request
		)

		BeforeEach(func() {
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))
			backup = testdp.NewFakeBackup(&testCtx, nil)
			backupRepo = testapps.CreateCustomizedObj(&testCtx, "backup/backuprepo.yaml", &dpv1alpha1.BackupRepo{}, nil)
			backupPolicy = testdp.NewBackupPolicyFactory(testCtx.DefaultNamespace, testdp.BackupPolicyName).
				SetBackupRepoName(testdp.BackupRepoName).
				SetPathPrefix(testdp.BackupPathPrefix).
				SetTarget(constant.AppInstanceLabelKey, testdp.ClusterName,
					constant.KBAppComponentLabelKey, testdp.ComponentName,
					constant.RoleLabelKey, testapps.Leader).
				AddBackupMethod(testdp.BackupMethodName, false, testdp.ActionSetName).
				Create(&testCtx).GetObject()

			targetPod = clusterInfo.TargetPod
			request = buildRequest()
		})

		Context("build action", func() {
			It("should build action", func() {
				request.Backup = backup
				request.ActionSet = actionSet
				request.TargetPods = []*corev1.Pod{targetPod}
				request.BackupPolicy = backupPolicy
				request.BackupMethod = &backupPolicy.Spec.BackupMethods[0]
				request.BackupRepo = backupRepo
				request.Target = backupPolicy.Spec.Target
				_, err := request.BuildActions()
				Expect(err).NotTo(HaveOccurred())
			})

			It("build create volume snapshot action", func() {
				request.TargetPods = []*corev1.Pod{targetPod}
				request.BackupMethod = &dpv1alpha1.BackupMethod{
					Name:            testdp.VSBackupMethodName,
					SnapshotVolumes: boolptr.True(),
				}
				_, err := request.buildCreateVolumeSnapshotAction(targetPod, "CreateVolumeSnapshot", 0)
				Expect(err).Should(HaveOccurred())
			})

		})
	})
})

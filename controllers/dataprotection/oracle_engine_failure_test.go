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

package dataprotection

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dp "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type oracleFailureFixture struct {
	cluster       *appsv1.Cluster
	comp          *appsv1.Component
	its           *workloads.InstanceSet
	pod           *corev1.Pod
	prepare, post *dp.Restore
	backup        *dp.Backup
	synth         *component.SynthesizedComponent
}

func oracleFailureCase() *oracleFailureFixture {
	now := time.Date(2026, 10, 25, 12, 40, 0, 0, time.UTC)
	owner := func(kind, name string, uid types.UID) []metav1.OwnerReference {
		yes := true
		return []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: kind, Name: name, UID: uid, Controller: &yes}}
	}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "ns", UID: "cluster-uid", Annotations: map[string]string{constant.RestoreFromBackupAnnotationKey: `{"oracle":{"name":"backup","restoreTime":"2026-10-25T12:34:56Z"}}`}}}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "target-oracle", Namespace: "ns", UID: "component-uid", Annotations: cluster.Annotations, OwnerReferences: owner("Cluster", cluster.Name, cluster.UID)}, Status: appsv1.ComponentStatus{Phase: appsv1.CreatingComponentPhase}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: comp.Name, Namespace: "ns", UID: "workload-uid", OwnerReferences: owner("Component", comp.Name, comp.UID)}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: its.Name + "-0", Namespace: "ns", UID: "pod-uid", OwnerReferences: owner("InstanceSet", its.Name, its.UID)}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "oracle", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, FinishedAt: metav1.NewTime(now), Message: `{"schema":"kubeblocks.io/oracle-restore-failure/v1","reason":"MissingArchive","code":"RMAN-06054","targetTime":"2026-10-25 12:34:56"}`}}}}}}
	pod.OwnerReferences[0].APIVersion = workloads.GroupVersion.String()
	prepare := &dp.Restore{ObjectMeta: metav1.ObjectMeta{Name: "target-oracle-cluster--preparedata", Namespace: "ns", UID: "prepare-uid", OwnerReferences: owner("Component", comp.Name, comp.UID)}, Spec: dp.RestoreSpec{Backup: dp.BackupRef{Name: "backup", Namespace: "ns"}, RestoreTime: "2026-10-25T12:34:56Z", PrepareDataConfig: &dp.PrepareDataConfig{}}, Status: dp.RestoreStatus{Phase: dp.RestorePhaseCompleted, CompletionTimestamp: &metav1.Time{Time: now.Add(-time.Minute)}}}
	post := &dp.Restore{ObjectMeta: metav1.ObjectMeta{Name: "target-oracle-cluster--postready", Namespace: "ns", UID: "post-uid", OwnerReferences: owner("Component", comp.Name, comp.UID), Finalizers: []string{dptypes.DataProtectionFinalizerName}, Annotations: map[string]string{"dataprotection.kubeblocks.io/engine-recovery-check": "true"}}, Spec: dp.RestoreSpec{Backup: prepare.Spec.Backup, RestoreTime: prepare.Spec.RestoreTime, ReadyConfig: &dp.ReadyConfig{}}}
	backup := &dp.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns"}, Status: dp.BackupStatus{BackupMethod: &dp.BackupMethod{Name: "oracle-archivelog", TargetVolumes: &dp.TargetVolumeInfo{VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}}}}
	synth := &component.SynthesizedComponent{Name: "oracle", Namespace: "ns", ClusterName: "target", Replicas: 1, Annotations: cluster.Annotations, PodSpec: &corev1.PodSpec{}, VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}}}
	return &oracleFailureFixture{cluster, comp, its, pod, prepare, post, backup, synth}
}
func oracleFailureScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, batchv1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme, dp.AddToScheme} {
		if err := add(s); err != nil {
			panic(err)
		}
	}
	return s
}
func (f *oracleFailureFixture) client(extra ...client.Object) client.Client {
	objs := []client.Object{f.cluster, f.comp, f.its, f.pod, f.prepare, f.backup}
	objs = append(objs, extra...)
	return fake.NewClientBuilder().WithScheme(oracleFailureScheme()).WithStatusSubresource(&dp.Restore{}).WithObjects(objs...).Build()
}
func TestOracleFailureCreatesRestoreBeforeComponentReady(t *testing.T) {
	f := oracleFailureCase()
	cli := f.client()
	mgr := plan.NewRestoreManager(context.Background(), cli, f.cluster, oracleFailureScheme(), nil, 1, 0)
	_ = mgr.DoRestore(f.synth, f.comp, false)
	post := &dp.Restore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.post), post); err != nil {
		t.Fatalf("failure must create a postReady record while Component is not Running: %v", err)
	}
	if post.Annotations["dataprotection.kubeblocks.io/engine-recovery-check"] != "true" {
		t.Fatal("early postReady must not run normal actions before failure/ready check")
	}
}
func TestOracleFailureReconcilesToFailed(t *testing.T) {
	f := oracleFailureCase()
	cli := f.client(f.post)
	r := &RestoreReconciler{Client: cli, Scheme: oracleFailureScheme(), Recorder: record.NewFakeRecorder(20)}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.post)})
	if err != nil {
		t.Fatal(err)
	}
	got := &dp.Restore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.post), got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != dp.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
	found := false
	for _, c := range got.Status.Conditions {
		if c.Type == "PostReady" && strings.Contains(c.Message, "RMAN-06054") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing engine failure condition: %#v", got.Status.Conditions)
	}
}

func TestOracleFailureRejectsUnrelatedSignals(t *testing.T) {
	cases := map[string]func(*oracleFailureFixture){
		"malformed JSON": func(f *oracleFailureFixture) {
			f.pod.Status.ContainerStatuses[0].State.Terminated.Message = "RMAN-06054: missing archive"
		},
		"unknown schema": func(f *oracleFailureFixture) {
			term := f.pod.Status.ContainerStatuses[0].State.Terminated
			term.Message = strings.ReplaceAll(term.Message, "/v1", "/v2")
		},
		"unknown reason": func(f *oracleFailureFixture) {
			term := f.pod.Status.ContainerStatuses[0].State.Terminated
			term.Message = strings.ReplaceAll(term.Message, "MissingArchive", "Timeout")
		},
		"storage error": func(f *oracleFailureFixture) {
			term := f.pod.Status.ContainerStatuses[0].State.Terminated
			term.Message = strings.ReplaceAll(term.Message, "RMAN-06054", "ORA-00308")
		},
		"unknown code": func(f *oracleFailureFixture) {
			term := f.pod.Status.ContainerStatuses[0].State.Terminated
			term.Message = strings.ReplaceAll(term.Message, "RMAN-06054", "RMAN-00000")
		},
		"old target time": func(f *oracleFailureFixture) {
			term := f.pod.Status.ContainerStatuses[0].State.Terminated
			term.Message = strings.ReplaceAll(term.Message, "12:34:56", "12:34:55")
		},
		"before preparation": func(f *oracleFailureFixture) {
			f.pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(f.prepare.Status.CompletionTimestamp.Add(-time.Second))
		},
		"no exit time": func(f *oracleFailureFixture) {
			f.pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.Time{}
		},
		"successful exit": func(f *oracleFailureFixture) { f.pod.Status.ContainerStatuses[0].State.Terminated.ExitCode = 0 },
		"ready container": func(f *oracleFailureFixture) { f.pod.Status.ContainerStatuses[0].Ready = true },
		"sidecar":         func(f *oracleFailureFixture) { f.pod.Status.ContainerStatuses[0].Name = "sidecar" },
		"oversized message": func(f *oracleFailureFixture) {
			f.pod.Status.ContainerStatuses[0].State.Terminated.Message += strings.Repeat(" ", 4097)
		},
		"different component": func(f *oracleFailureFixture) { f.its.OwnerReferences[0].UID = "foreign" },
		"replaced workload":   func(f *oracleFailureFixture) { f.pod.OwnerReferences[0].UID = "old-workload" },
		"noncontroller owner": func(f *oracleFailureFixture) { no := false; f.pod.OwnerReferences[0].Controller = &no },
		"wrong owner group":   func(f *oracleFailureFixture) { f.pod.OwnerReferences[0].APIVersion = "other.io/v1" },
		"old component":       func(f *oracleFailureFixture) { f.comp.UID = "replacement" },
		"old cluster":         func(f *oracleFailureFixture) { f.cluster.UID = "replacement" },
		"different active request": func(f *oracleFailureFixture) {
			f.cluster.Annotations = map[string]string{constant.RestoreFromBackupAnnotationKey: `{"oracle":{"name":"other","restoreTime":"2026-10-25T12:34:56Z"}}`}
		},
		"restore already done":       func(f *oracleFailureFixture) { f.comp.Annotations[constant.RestoreDoneAnnotationKey] = "true" },
		"prepare not complete":       func(f *oracleFailureFixture) { f.prepare.Status.Phase = dp.RestorePhaseRunning },
		"prepare missing time":       func(f *oracleFailureFixture) { f.prepare.Status.CompletionTimestamp = nil },
		"different backup":           func(f *oracleFailureFixture) { f.prepare.Spec.Backup.Name = "other" },
		"different backup namespace": func(f *oracleFailureFixture) { f.prepare.Spec.Backup.Namespace = "other" },
		"different source target":    func(f *oracleFailureFixture) { f.prepare.Spec.Backup.SourceTargetName = "other" },
		"different prepare time":     func(f *oracleFailureFixture) { f.prepare.Spec.RestoreTime = "2026-10-25T12:34:55Z" },
		"foreign preparation":        func(f *oracleFailureFixture) { f.prepare.OwnerReferences[0].UID = "other" },
		"foreign namespace":          func(f *oracleFailureFixture) { f.pod.Namespace = "other" },
		"deleted pod": func(f *oracleFailureFixture) {
			now := metav1.Now()
			f.pod.DeletionTimestamp = &now
			f.pod.Finalizers = []string{"keep"}
		},
		"deleted workload": func(f *oracleFailureFixture) {
			now := metav1.Now()
			f.its.DeletionTimestamp = &now
			f.its.Finalizers = []string{"keep"}
		},
		"deleted preparation": func(f *oracleFailureFixture) {
			now := metav1.Now()
			f.prepare.DeletionTimestamp = &now
			f.prepare.Finalizers = []string{"keep"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := oracleFailureCase()
			mutate(f)
			cli := f.client(f.post)
			beforePrepare := &dp.Restore{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.prepare), beforePrepare); err != nil {
				t.Fatal(err)
			}
			r := &RestoreReconciler{Client: cli, Scheme: oracleFailureScheme(), Recorder: record.NewFakeRecorder(20)}
			res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.post)})
			if err != nil {
				t.Fatal(err)
			}
			got := &dp.Restore{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.post), got); err != nil {
				t.Fatal(err)
			}
			if got.Status.Phase != "" || res.RequeueAfter == 0 {
				t.Fatalf("unrelated signal changed status or did not retry: %#v, %#v", got.Status, res)
			}
			jobs := &batchv1.JobList{}
			if err := cli.List(context.Background(), jobs); err != nil {
				t.Fatal(err)
			}
			if len(jobs.Items) != 0 {
				t.Fatal("failure-only record started ordinary actions")
			}
		})
	}
}
func TestOracleFailureLastExitAndIdempotence(t *testing.T) {
	for _, code := range []string{"RMAN-06025", "RMAN-06054"} {
		t.Run(code, func(t *testing.T) {
			f := oracleFailureCase()
			c := &f.pod.Status.ContainerStatuses[0]
			c.State.Terminated.Message = strings.ReplaceAll(c.State.Terminated.Message, "RMAN-06054", code)
			c.LastTerminationState = c.State
			c.State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}
			f.post.Status.Phase = dp.RestorePhaseRunning
			cli := f.client(f.post)
			beforePrepare := &dp.Restore{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.prepare), beforePrepare); err != nil {
				t.Fatal(err)
			}
			r := &RestoreReconciler{Client: cli, Scheme: oracleFailureScheme(), Recorder: record.NewFakeRecorder(20)}
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.post)}
			if _, err := r.Reconcile(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			first := &dp.Restore{}
			if err := cli.Get(context.Background(), req.NamespacedName, first); err != nil {
				t.Fatal(err)
			}
			if first.Status.Phase != dp.RestorePhaseFailed || first.Status.CompletionTimestamp == nil {
				t.Fatalf("not failed: %#v", first.Status)
			}
			if _, err := r.Reconcile(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			second := &dp.Restore{}
			if err := cli.Get(context.Background(), req.NamespacedName, second); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first.Status, second.Status) {
				t.Fatal("repeat changed terminal status")
			}
			prep := &dp.Restore{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.prepare), prep); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(prep.Status, beforePrepare.Status) {
				t.Fatal("prepareData completion was rewritten")
			}
			// The plan must propagate the terminal record even after the Pod disappears.
			if err := cli.Delete(context.Background(), f.pod); err != nil {
				t.Fatal(err)
			}
			mgr := plan.NewRestoreManager(context.Background(), cli, f.cluster, oracleFailureScheme(), nil, 1, 0)
			if err := mgr.DoRestore(f.synth, f.comp, false); err == nil || !strings.Contains(err.Error(), "status is Failed") {
				t.Fatalf("lost terminal failure: %v", err)
			}
		})
	}
}
func TestOracleFailureDoesNotCreateEarlyRecordForRetryableError(t *testing.T) {
	f := oracleFailureCase()
	f.pod.Status.ContainerStatuses[0].State.Terminated.Message = "ORA-00308: transient I/O"
	cli := f.client()
	mgr := plan.NewRestoreManager(context.Background(), cli, f.cluster, oracleFailureScheme(), nil, 1, 0)
	if err := mgr.DoRestore(f.synth, f.comp, false); err != nil {
		t.Fatal(err)
	}
	err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.post), &dp.Restore{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("created early record: %v", err)
	}
}

type oracleReadErrorClient struct{ client.Client }

func (c oracleReadErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("read failed")
}

type oracleConflictClient struct{ client.Client }

func (c oracleConflictClient) Status() client.SubResourceWriter {
	return oracleConflictStatus{c.Client.Status()}
}

type oracleConflictStatus struct{ client.SubResourceWriter }

func (c oracleConflictStatus) Patch(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
	return apierrors.NewConflict(schema.GroupResource{Group: dp.GroupVersion.Group, Resource: "restores"}, "postready", errors.New("newer status"))
}
func TestOracleFailureReadAndConflictErrorsDoNotWrite(t *testing.T) {
	for _, mode := range []string{"read", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			f := oracleFailureCase()
			base := f.client(f.post)
			var cli client.Client = oracleReadErrorClient{base}
			if mode == "conflict" {
				cli = oracleConflictClient{base}
			}
			r := &RestoreReconciler{Client: cli, Scheme: oracleFailureScheme(), Recorder: record.NewFakeRecorder(20)}
			if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.post)}); err == nil {
				t.Fatal("expected error")
			}
			got := &dp.Restore{}
			if err := base.Get(context.Background(), client.ObjectKeyFromObject(f.post), got); err != nil {
				t.Fatal(err)
			}
			if got.Status.Phase != "" {
				t.Fatal("failed request wrote status")
			}
		})
	}
}
func TestOracleFailureWaitsForEveryPreparation(t *testing.T) {
	f := oracleFailureCase()
	other := f.prepare.DeepCopy()
	other.Name += "-other"
	other.UID = "other"
	other.Status.Phase = dp.RestorePhaseRunning
	cli := f.client(other)
	message, err := dprestore.OracleEngineFailure(context.Background(), cli, f.post)
	if err != nil || message != "" {
		t.Fatalf("accepted partially completed preparation: %q %v", message, err)
	}
}

func TestOracleFailureExistingOrdinaryPostReady(t *testing.T) {
	f := oracleFailureCase()
	f.post.Annotations = nil
	f.post.Status.Phase = dp.RestorePhaseRunning
	cli := f.client(f.post)
	r := &RestoreReconciler{Client: cli, Scheme: oracleFailureScheme(), Recorder: record.NewFakeRecorder(20)}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.post)}); err != nil {
		t.Fatal(err)
	}
	got := &dp.Restore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(f.post), got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != dp.RestorePhaseFailed {
		t.Fatalf("ordinary postReady missed fatal engine exit: %s", got.Status.Phase)
	}
}

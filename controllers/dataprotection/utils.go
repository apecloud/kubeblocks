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

package dataprotection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dperrors "github.com/apecloud/kubeblocks/pkg/dataprotection/errors"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

var (
	errNoDefaultBackupRepo = fmt.Errorf("no default BackupRepo found")
)

// getBackupRepo returns the backup repo specified by the backup object or the policy.
// if no backup repo specified, it will return the default one.
func getBackupRepo(ctx context.Context,
	cli client.Client,
	backup *dpv1alpha1.Backup,
	backupPolicy *dpv1alpha1.BackupPolicy) (*dpv1alpha1.BackupRepo, error) {
	// use the specified backup repo
	var repoName string
	if val := backup.Labels[dataProtectionBackupRepoKey]; val != "" {
		repoName = val
	} else if backupPolicy.Spec.BackupRepoName != nil && *backupPolicy.Spec.BackupRepoName != "" {
		repoName = *backupPolicy.Spec.BackupRepoName
	}
	if repoName != "" {
		repo := &dpv1alpha1.BackupRepo{}
		if err := cli.Get(ctx, client.ObjectKey{Name: repoName}, repo); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, intctrlutil.NewNotFound("backup repo %s not found", repoName)
			}
			return nil, err
		}
		return repo, nil
	}
	// fallback to use the default repo
	return getDefaultBackupRepo(ctx, cli)
}

func HandleBackupRepo(request *dpbackup.Request) error {
	repo, err := getBackupRepo(request.Ctx, request.Client, request.Backup, request.BackupPolicy)
	if err != nil {
		return err
	}
	request.BackupRepo = repo

	if repo.Status.Phase != dpv1alpha1.BackupRepoReady {
		return dperrors.NewBackupRepoIsNotReady(repo.Name)
	}

	switch {
	case repo.AccessByMount():
		pvcName := repo.Status.BackupPVCName
		if pvcName == "" {
			return dperrors.NewBackupPVCNameIsEmpty(repo.Name, request.Spec.BackupPolicyName)
		}
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := client.ObjectKey{Namespace: request.Req.Namespace, Name: pvcName}
		if err = request.Client.Get(request.Ctx, pvcKey, pvc); err != nil {
			// will wait for the backuprepo controller to create the PVC,
			// so ignore the NotFound error
			return client.IgnoreNotFound(err)
		}
		// backupRepo PVC exists, record the PVC name
		if err == nil {
			request.BackupRepoPVC = pvc
		}
	case repo.AccessByTool():
		toolConfigSecretName := repo.Status.ToolConfigSecretName
		if toolConfigSecretName == "" {
			return dperrors.NewToolConfigSecretNameIsEmpty(repo.Name)
		}
		secret := &corev1.Secret{}
		secretKey := client.ObjectKey{Namespace: request.Req.Namespace, Name: toolConfigSecretName}
		if err = request.Client.Get(request.Ctx, secretKey, secret); err != nil {
			// will wait for the backuprepo controller to create the secret,
			// so ignore the NotFound error
			return client.IgnoreNotFound(err)
		}
		if err == nil {
			request.ToolConfigSecret = secret
		}
	}
	return nil
}

// GetTargetPods gets the target pods by BackupPolicy. If podName is not empty,
// it will return the pod which name is podName. Otherwise, it will return the
// pods which are selected by BackupPolicy selector and strategy.
func GetTargetPods(reqCtx intctrlutil.RequestCtx,
	cli client.Client, podName string,
	backupMethod *dpv1alpha1.BackupMethod,
	backupPolicy *dpv1alpha1.BackupPolicy,
) ([]*corev1.Pod, error) {
	if backupMethod == nil {
		return nil, nil
	}
	existPodSelector := func(selector *dpv1alpha1.PodSelector) bool {
		return selector != nil && selector.LabelSelector != nil
	}
	var selector *dpv1alpha1.PodSelector
	if backupMethod.Target != nil && existPodSelector(backupMethod.Target.PodSelector) {
		selector = backupMethod.Target.PodSelector
	} else {
		// using global target policy.
		selector = backupPolicy.Spec.Target.PodSelector
		if !existPodSelector(selector) {
			return nil, nil
		}
	}
	labelSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
	if err != nil {
		return nil, err
	}
	pods := &corev1.PodList{}
	if err = cli.List(reqCtx.Ctx, pods,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("failed to find target pods by backup policy %s/%s",
			backupPolicy.Namespace, backupPolicy.Name)
	}

	var targetPods []*corev1.Pod
	if podName != "" && selector.Strategy == dpv1alpha1.PodSelectionStrategyAny {
		for _, pod := range pods.Items {
			if pod.Name == podName {
				targetPods = append(targetPods, &pod)
				break
			}
		}
		if len(targetPods) > 0 {
			return targetPods, nil
		}
	}
	sort.Sort(intctrlutil.ByPodName(pods.Items))
	// if pod selection strategy is Any, always return first pod
	switch selector.Strategy {
	case dpv1alpha1.PodSelectionStrategyAny:
		pod := dputils.GetFirstIndexRunningPod(pods)
		if pod != nil {
			targetPods = append(targetPods, pod)
		}
	case dpv1alpha1.PodSelectionStrategyAll:
		for i := range pods.Items {
			targetPods = append(targetPods, &pods.Items[i])
		}
	}

	return targetPods, nil
}

// getCluster gets the cluster and will ignore the error.
func getCluster(ctx context.Context,
	cli client.Client,
	targetPod *corev1.Pod) *appsv1alpha1.Cluster {
	clusterName := targetPod.Labels[constant.AppInstanceLabelKey]
	if len(clusterName) == 0 {
		return nil
	}
	cluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: targetPod.Namespace,
		Name:      clusterName,
	}, cluster); err != nil {
		// should not affect the backup status
		return nil
	}
	return cluster
}

func getClusterLabelKeys() []string {
	return []string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey}
}

// sendWarningEventForError sends warning event for backup controller error
func sendWarningEventForError(recorder record.EventRecorder, obj client.Object, err error) {
	controllerErr := intctrlutil.UnwrapControllerError(err)
	if controllerErr != nil {
		recorder.Eventf(obj, corev1.EventTypeWarning, string(controllerErr.Type), err.Error())
	} else {
		recorder.Eventf(obj, corev1.EventTypeWarning, "FailedCreatedBackup",
			"Creating backup failed, error: %s", err.Error())
	}
}

func getDefaultBackupRepo(ctx context.Context, cli client.Client) (*dpv1alpha1.BackupRepo, error) {
	backupRepoList := &dpv1alpha1.BackupRepoList{}
	if err := cli.List(ctx, backupRepoList); err != nil {
		return nil, err
	}
	var defaultRepo *dpv1alpha1.BackupRepo
	for idx := range backupRepoList.Items {
		repo := &backupRepoList.Items[idx]
		// skip non-default repo
		if !(repo.Annotations[dptypes.DefaultBackupRepoAnnotationKey] == trueVal &&
			repo.Status.Phase == dpv1alpha1.BackupRepoReady) {
			continue
		}
		if defaultRepo != nil {
			return nil, fmt.Errorf("multiple default BackupRepo found, both %s and %s are default",
				defaultRepo.Name, repo.Name)
		}
		defaultRepo = repo
	}
	if defaultRepo == nil {
		return nil, errNoDefaultBackupRepo
	}
	return defaultRepo, nil
}

func deleteRelatedJobs(reqCtx intctrlutil.RequestCtx, cli client.Client, namespace string, labels map[string]string) error {
	if labels == nil || namespace == "" {
		return nil
	}
	jobs := &batchv1.JobList{}
	if err := cli.List(reqCtx.Ctx, jobs,
		client.MatchingLabels(labels)); err != nil {
		return client.IgnoreNotFound(err)
	}
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if err := dputils.RemoveDataProtectionFinalizer(reqCtx.Ctx, cli, job); err != nil {
			return err
		}
		if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, job); err != nil {
			return err
		}
	}
	return nil
}

func RecorderEventAndRequeue(reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder,
	obj client.Object, err error) (reconcile.Result, error) {
	sendWarningEventForError(recorder, obj, err)
	return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
}

// ============================================================================
// refObjectMapper
// ============================================================================

// refObjectMapper is a helper struct that maintains the mapping between referent objects and referenced objects.
// A referent object is an object that has a reference to another object in its spec.
// A referenced object is an object that is referred by one or more referent objects.
// It is mainly used in the controller Watcher() to trigger the reconciliation of the
// objects that have references to other objects when those objects change.
// For example, if object A has a reference to object B, and object B changes,
// the refObjectMapper can generate a request for object A to be reconciled.
type refObjectMapper struct {
	mu     sync.Mutex
	once   sync.Once
	ref    map[string]string   // key is the referent, value is the referenced object.
	invert map[string][]string // invert map, key is the referenced object, value is the list of referent.
}

// init initializes the ref and invert maps lazily if they are nil.
func (r *refObjectMapper) init() {
	r.once.Do(func() {
		r.ref = make(map[string]string)
		r.invert = make(map[string][]string)
	})
}

// setRef sets or updates the mapping between a referent object and a referenced object.
func (r *refObjectMapper) setRef(referent client.Object, referencedKey types.NamespacedName) {
	r.init()
	r.mu.Lock()
	defer r.mu.Unlock()
	left := toFlattenName(client.ObjectKeyFromObject(referent))
	right := toFlattenName(referencedKey)
	if oldRight, ok := r.ref[left]; ok {
		r.removeInvertLocked(left, oldRight)
	}
	r.addInvertLocked(left, right)
	r.ref[left] = right
}

// removeRef removes the mapping for a given referent object.
func (r *refObjectMapper) removeRef(referent client.Object) {
	r.init()
	r.mu.Lock()
	defer r.mu.Unlock()
	left := toFlattenName(client.ObjectKeyFromObject(referent))
	if right, ok := r.ref[left]; ok {
		r.removeInvertLocked(left, right)
		delete(r.ref, left)
	}
}

// mapToRequests returns a list of requests for the referent objects that have a reference to a given referenced object.
func (r *refObjectMapper) mapToRequests(referenced client.Object) []ctrl.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	right := toFlattenName(client.ObjectKeyFromObject(referenced))
	l := r.invert[right]
	var ret []ctrl.Request
	for _, v := range l {
		name, namespace := fromFlattenName(v)
		ret = append(ret, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}})
	}
	return ret
}

// addInvertLocked adds a pair of referent and referenced objects to the invert map.
// It assumes the lock is already held by the caller.
func (r *refObjectMapper) addInvertLocked(left string, right string) {
	// no duplicated item in the list
	l := r.invert[right]
	r.invert[right] = append(l, left)
}

// removeInvertLocked removes a pair of referent and referenced objects from the invert map.
// It assumes the lock is already held by the caller.
func (r *refObjectMapper) removeInvertLocked(left string, right string) {
	l := r.invert[right]
	for i, v := range l {
		if v == left {
			l[i] = l[len(l)-1]
			r.invert[right] = l[:len(l)-1]
			return
		}
	}
}

func toFlattenName(key types.NamespacedName) string {
	return key.Namespace + "/" + key.Name
}

func fromFlattenName(flatten string) (name string, namespace string) {
	parts := strings.SplitN(flatten, "/", 2)
	if len(parts) == 2 {
		namespace = parts[0]
		name = parts[1]
	} else {
		name = flatten
	}
	return
}

// restore functions

func getPopulatePVCName(pvcUID types.UID) string {
	return fmt.Sprintf("%s-%s", PopulatePodPrefix, pvcUID)
}

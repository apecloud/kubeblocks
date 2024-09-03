/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dperrors "github.com/apecloud/kubeblocks/pkg/dataprotection/errors"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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
			return dperrors.NewBackupPVCNameIsEmpty(repo.Name)
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
	cli client.Client,
	selectedPodNames []string,
	backupPolicy *dpv1alpha1.BackupPolicy,
	target *dpv1alpha1.BackupTarget,
	backupType dpv1alpha1.BackupType) ([]*corev1.Pod, error) {
	if target == nil {
		return nil, nil
	}
	existPodSelector := func(selector *dpv1alpha1.PodSelector) bool {
		return selector != nil && selector.LabelSelector != nil
	}
	// using global target policy.
	selector := target.PodSelector
	if !existPodSelector(selector) {
		return nil, nil
	}

	filterTargetPods := func(strategy dpv1alpha1.PodSelectionStrategy,
		labelSelector *metav1.LabelSelector) ([]*corev1.Pod, error) {
		var targetPods []*corev1.Pod
		pods, err := dputils.GetPodListByLabelSelector(reqCtx, cli, labelSelector)
		if err != nil {
			return nil, err
		}
		switch strategy {
		case dpv1alpha1.PodSelectionStrategyAny:
			var pod *corev1.Pod
			if len(selectedPodNames) == 0 || backupType == dpv1alpha1.BackupTypeContinuous {
				pod = dputils.GetFirstIndexRunningPod(pods)
			} else {
				// if already selected target pods and backupType is not Continuous, we should re-use them.
				pod = dputils.GetPodByName(pods, selectedPodNames[0])
			}
			if pod != nil {
				targetPods = append(targetPods, pod)
			}
		case dpv1alpha1.PodSelectionStrategyAll:
			if len(selectedPodNames) == 0 || backupType == dpv1alpha1.BackupTypeContinuous {
				for i := range pods.Items {
					targetPods = append(targetPods, &pods.Items[i])
				}
				return targetPods, nil
			}
			// if already selected target pods and backupType is not Continuous, we should re-use them.
			if len(pods.Items) == 0 {
				return nil, fmt.Errorf("failed to find target pods by backup policy %s/%s",
					backupPolicy.Namespace, backupPolicy.Name)
			}
			podMap := map[string]*corev1.Pod{}
			for i := range pods.Items {
				podMap[pods.Items[i].Name] = &pods.Items[i]
			}
			for _, podName := range selectedPodNames {
				pod, ok := podMap[podName]
				if !ok {
					return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not found the target pod "%s"`, podName))
				}
				targetPods = append(targetPods, pod)
			}
		}
		return targetPods, nil
	}

	targetPods, err := filterTargetPods(selector.Strategy, selector.LabelSelector)
	if err != nil {
		return nil, err
	}
	// if selector.LabelSelector fails to filter a available target pod or the selected target pod,
	// use selector.FallbackLabelSelector to filter, and selector.FallbackLabelSelector only takes effect
	// when selector.Strategy equals to dpv1alpha1.PodSelectionStrategyAny.
	if selector.Strategy == dpv1alpha1.PodSelectionStrategyAll || len(targetPods) > 0 ||
		selector.FallbackLabelSelector == nil {
		return targetPods, nil
	}
	if targetPods, err = filterTargetPods(selector.Strategy, selector.FallbackLabelSelector); err != nil {
		return nil, err
	}
	return targetPods, nil
}

// getCluster gets the cluster and will ignore the error.
func getCluster(ctx context.Context,
	cli client.Client,
	targetPod *corev1.Pod) *appsv1.Cluster {
	clusterName := targetPod.Labels[constant.AppInstanceLabelKey]
	if len(clusterName) == 0 {
		return nil
	}
	cluster := &appsv1.Cluster{}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: targetPod.Namespace,
		Name:      clusterName,
	}, cluster); err != nil {
		// should not affect the backup status
		return nil
	}
	return cluster
}

// listObjectsOfCluster list the objects of the cluster by labels.
func listObjectsOfCluster(ctx context.Context,
	cli client.Client,
	cluster *appsv1.Cluster,
	object client.ObjectList) (client.ObjectList, error) {
	labels := constant.GetClusterWellKnownLabels(cluster.Name)
	if err := cli.List(ctx, object, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return object, nil
}

// getObjectString convert object to string
func getObjectString(object any) (*string, error) {
	if object == nil {
		return nil, nil
	}
	objectBytes, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	objectString := string(objectBytes)
	return &objectString, nil
}

func getClusterLabelKeys() []string {
	return []string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey, constant.KBAppShardingNameLabelKey}
}

// sendWarningEventForError sends warning event for controller error
func sendWarningEventForError(recorder record.EventRecorder, obj client.Object, err error) {
	controllerErr := intctrlutil.UnwrapControllerError(err)
	if controllerErr != nil {
		recorder.Eventf(obj, corev1.EventTypeWarning, string(controllerErr.Type), err.Error())
	} else {
		recorder.Eventf(obj, corev1.EventTypeWarning, "ReconcileFailed", "Reconciling failed, error: %s", err.Error())
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

func UniversalContext(ctx context.Context, mcMgr multicluster.Manager) context.Context {
	if mcMgr == nil {
		return ctx
	}
	return multicluster.IntoContext(ctx, strings.Join(mcMgr.GetContexts(), ","))
}

func checkResourceUniversallyAvailable(ctx context.Context, cli client.Client, objKey client.ObjectKey, obj client.Object, mcMgr multicluster.Manager) error {
	if mcMgr != nil {
		for _, dataCluster := range mcMgr.GetContexts() {
			getCtx := multicluster.IntoContext(ctx, dataCluster)
			err := cli.Get(getCtx, objKey, obj, multicluster.Oneshot())
			if err != nil {
				return fmt.Errorf("get %s from the %s data cluster error: %w", objKey.String(), dataCluster, err)
			}
		}
	}
	if err := cli.Get(ctx, objKey, obj, multicluster.InControlContext()); err != nil {
		return fmt.Errorf("get %s from the control cluster error: %w", objKey.String(), err)
	}
	return nil
}

func EnsureWorkerServiceAccount(reqCtx intctrlutil.RequestCtx, cli client.Client, namespace string, mcMgr multicluster.Manager) (string, error) {
	if namespace == "" {
		return "", fmt.Errorf("namespace is empty")
	}
	saName := viper.GetString(dptypes.CfgKeyWorkerServiceAccountName)
	if saName == "" {
		return "", fmt.Errorf("worker service account name is empty")
	}
	sa := &corev1.ServiceAccount{}
	saKey := client.ObjectKey{Namespace: namespace, Name: saName}
	err := checkResourceUniversallyAvailable(reqCtx.Ctx, cli, saKey, sa, mcMgr)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}
	saExists := err == nil

	clusterRoleName := viper.GetString(dptypes.CfgKeyWorkerClusterRoleName)
	if clusterRoleName == "" {
		return "", fmt.Errorf("worker cluster role name is empty")
	}

	var extraAnnotations map[string]string
	annotationsJSON := viper.GetString(dptypes.CfgKeyWorkerServiceAccountAnnotations)
	if annotationsJSON != "" {
		extraAnnotations = make(map[string]string)
		err := json.Unmarshal([]byte(annotationsJSON), &extraAnnotations)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal worker service account annotations: %s, json: %q",
				err.Error(), annotationsJSON)
		}
	}

	ctx := UniversalContext(reqCtx.Ctx, mcMgr)

	if saExists {
		// SA exists, check if annotations are consistent
		saCopy := sa.DeepCopy()
		if len(extraAnnotations) > 0 && sa.Annotations == nil {
			sa.Annotations = extraAnnotations
		} else {
			for k, v := range extraAnnotations {
				sa.Annotations[k] = v
			}
		}
		sa.ImagePullSecrets = intctrlutil.BuildImagePullSecrets()
		if !reflect.DeepEqual(sa, saCopy) {
			err := cli.Patch(ctx, sa, client.MergeFrom(saCopy), multicluster.InUniversalContext())
			if err != nil {
				return "", fmt.Errorf("failed to patch worker service account: %w", err)
			}
		}
		// fast path
		return saName, nil
	}

	createRoleBinding := func() error {
		rb := &rbacv1.RoleBinding{}
		rb.Name = fmt.Sprintf("%s-rolebinding", saName)
		rb.Namespace = namespace
		rb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      saName,
			Namespace: namespace,
		}}
		rb.RoleRef = rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		}
		if err := cli.Create(ctx, rb, multicluster.InUniversalContext()); err != nil {
			return client.IgnoreAlreadyExists(err)
		}
		return nil
	}

	createServiceAccount := func() error {
		sa := &corev1.ServiceAccount{}
		sa.Name = saName
		sa.Namespace = namespace
		sa.Annotations = extraAnnotations
		sa.ImagePullSecrets = intctrlutil.BuildImagePullSecrets()
		if err := cli.Create(ctx, sa, multicluster.InUniversalContext()); err != nil {
			return client.IgnoreAlreadyExists(err)
		}
		return nil
	}

	// this function returns earlier if the service account already exists,
	// so we create the role binding first for idempotent.
	if err := createRoleBinding(); err != nil {
		return "", fmt.Errorf("failed to create rolebinding: %w", err)
	}
	if err := createServiceAccount(); err != nil {
		return "", fmt.Errorf("failed to create service account: %w", err)
	}
	return saName, nil
}

func checkSecretKeyRef(reqCtx intctrlutil.RequestCtx, cli client.Client,
	namespace string, ref *corev1.SecretKeySelector) error {
	if ref == nil {
		return fmt.Errorf("ref is nil")
	}
	secret := &corev1.Secret{}
	err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      ref.Name,
	}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("secret (%s/%s) is not found", ref.Name, namespace)
		}
		return err
	}
	if _, has := secret.Data[ref.Key]; !has {
		return fmt.Errorf("secret (%s/%s) doesn't contain key %s",
			ref.Name, namespace, ref.Key)
	}
	return nil
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

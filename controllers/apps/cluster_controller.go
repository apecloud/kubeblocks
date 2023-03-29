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

package apps

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update

// owned K8s core API resources controller-gen RBAC marker
// full access on core API resources
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=resourcequotas/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=resourcequotas/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=replicasets/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update

// read + update access
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// read only + watch access
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// dataprotection get list and delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;delete;deletecollection
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;delete;deletecollection

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// ClusterStatusEventHandler is the event handler for the cluster status event
type ClusterStatusEventHandler struct{}

var _ k8score.EventHandler = &ClusterStatusEventHandler{}

func init() {
	k8score.EventHandlerMap["cluster-status-handler"] = &ClusterStatusEventHandler{}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "cluster", req.NamespacedName)
	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	clusterConditionMgr := clusterConditionManager{
		Client:   r.Client,
		Recorder: r.Recorder,
		ctx:      ctx,
		cluster:  cluster,
	}

	// deletion check stage
	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, cluster, dbClusterFinalizerName, func() (*ctrl.Result, error) {
		return r.deleteExternalResources(reqCtx, cluster)
	}); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}

	// metadata update stage
	// should patch the label first to prevent the label from being modified by the user.
	if res, err := r.patchClusterLabelsIfNotExist(reqCtx, cluster); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}

	doDependencyCRsCheck := func(kind string, fetcher func() error, checker func() (*ctrl.Result, error)) (*ctrl.Result, error) {
		// validate required objects
		reqCtx.Log.V(1).Info("fetch", "kind", kind)
		err := fetcher()
		if err != nil {
			if apierrors.IsNotFound(err) {
				if setErr := clusterConditionMgr.setPreCheckErrorCondition(err); componentutil.IgnoreNoOps(setErr) != nil {
					return nil, setErr
				}
			}
			return intctrlutil.ResultToP(intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log))
		}
		return checker()
	}

	doStatusPatch := func(doPatchPredicate func() bool, modifier func() error, postHandler func() error) (*ctrl.Result, error) {
		if !doPatchPredicate() {
			return nil, nil
		}
		patch := client.MergeFrom(cluster.DeepCopy())
		if err := modifier(); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}
		if err := r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
			return nil, err
		}
		if postHandler != nil {
			if err := postHandler(); err != nil {
				return nil, err
			}
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}

	// fetch required dependency objects stage
	// validate ClusterDefinition
	var clusterDefinition = &appsv1alpha1.ClusterDefinition{}
	if res, err := doDependencyCRsCheck("ClusterDefinition", func() error {
		return r.Client.Get(reqCtx.Ctx, types.NamespacedName{
			Name: cluster.Spec.ClusterDefRef,
		}, clusterDefinition)
	}, func() (*ctrl.Result, error) {
		return r.checkReferencedCRStatus(reqCtx, clusterConditionMgr, clusterDefinition.Status.Phase,
			appsv1alpha1.ClusterDefinitionKind, clusterDefinition.Name)
	}); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}

	// observedGeneration and terminal phase check state
	if cluster.Status.ObservedGeneration == cluster.Generation &&
		slices.Contains(appsv1alpha1.GetClusterTerminalPhases(), cluster.Status.Phase) {
		// reconcile the phase and conditions of the Cluster.status
		if res, err := r.reconcileClusterStatus(reqCtx, cluster, clusterDefinition); componentutil.IgnoreNoOps(err) != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		} else if res != nil {
			return *res, nil
		}

		if err := r.cleanupAnnotationsAfterRunning(reqCtx, cluster); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	// Cluster CR generation update stage
	if res, err := doStatusPatch(func() bool {
		return cluster.Status.ObservedGeneration != cluster.Generation
	}, func() error {
		cluster.Status.ObservedGeneration = cluster.Generation
		if cluster.Status.Phase == "" {
			// REVIEW: may need to start with "validating" phase
			cluster.Status.Phase = appsv1alpha1.StartingClusterPhase
			return nil
		}
		if cluster.Status.Phase != appsv1alpha1.StartingClusterPhase {
			cluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
		}
		return nil
	}, nil); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}

	// validation stage
	// validate ClusterVersion
	var clusterVersion = &appsv1alpha1.ClusterVersion{}
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		if res, err := doDependencyCRsCheck("ClusterVersion", func() error {
			return r.Client.Get(reqCtx.Ctx, types.NamespacedName{
				Name: cluster.Spec.ClusterVersionRef,
			}, clusterVersion)
		}, func() (*ctrl.Result, error) {
			return r.checkReferencedCRStatus(reqCtx, clusterConditionMgr, clusterVersion.Status.Phase,
				appsv1alpha1.ClusterVersionKind, clusterVersion.Name)
		}); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		} else if res != nil {
			return *res, nil
		}
	}

	// validate config and send warning event log necessarily
	if err := cluster.ValidateEnabledLogs(clusterDefinition); err != nil {
		if setErr := clusterConditionMgr.setPreCheckErrorCondition(err); componentutil.IgnoreNoOps(setErr) != nil {
			return intctrlutil.RequeueWithError(setErr, reqCtx.Log, "")
		}
		return intctrlutil.RequeueAfter(requeueDuration, reqCtx.Log, "")
	}

	// ClusterDefinition CR generation update stage
	if res, err := doStatusPatch(func() bool {
		return cluster.Status.ClusterDefGeneration != clusterDefinition.Generation
	}, func() error {
		cluster.Status.ClusterDefGeneration = clusterDefinition.Generation
		return nil
	}, nil); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}

	// progressing process stage
	// cluster status progressing phase update
	reqCtx.Log.V(1).Info("update cluster status")
	if res, err := r.updateClusterPhaseWithOperations(reqCtx, cluster); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}

	// preCheck succeed, starting the cluster provisioning
	if err := clusterConditionMgr.setProvisioningStartedCondition(); componentutil.IgnoreNoOps(err) != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	clusterDeepCopy := cluster.DeepCopy()
	shouldRequeue, err := reconcileClusterWorkloads(reqCtx, r.Client, clusterDefinition, clusterVersion, cluster)
	if err != nil {
		if patchErr := r.patchClusterStatus(reqCtx.Ctx, cluster, clusterDeepCopy); patchErr != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		// REVIEW: got following error:
		// - "PersistentVolumeClaim \"data-test-clusterljdhar-mysql1-0\" not found"
		// - "secrets \"test-clusternjcfvi-mysql-tls-certs\" already exists" // "clusterdefinitions.apps.kubeblocks.io \"test-clusterdef-tls\" not found" // "clusterdefinitions.apps.kubeblocks.io \"clusterdef-for-status-fgm25k\" not found"
		// - "cluster test-clusterkqajfv h-scale failed, backup error: BackupPolicy.dataprotection.kubeblocks.io \"test-backup-policy\" not found"
		// - "PersistentVolumeClaim \"data-mysql-5can4v-consensus-2\" is invalid: spec.resources.requests.storage: Forbidden: field can not be less than previous value"
		// - "PersistentVolumeClaim \"data-test-clustershmgtv-redis-rsts-0\" is invalid: spec: Forbidden: spec is immutable after creation except resources.requests for bound claims\n  core.PersistentVolumeClaimSpec{\n  \tAccessModes: {\"ReadWriteOnce\"},\n  \tSelector:    nil,\n  \tResources: core.ResourceRequirements{\n  \t\tLimits: nil,\n- \t\tRequests: core.ResourceList{\n- \t\t\ts\"storage\": {i: resource.int64Amount{value: 1073741824}, s: \"1Gi\", Format: \"BinarySI\"},\n- \t\t},\n+ \t\tRequests: core.ResourceList{\n+ \t\t\ts\"storage\": {i: resource.int64Amount{value: 3221225472}, s: \"3Gi\", Format: \"BinarySI\"},\n+ \t\t},\n  \t},\n  \tVolumeName:       \"\",\n  \tStorageClassName: &\"standard\",\n  \t... // 3 identical fields\n  }\n"

		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		if setErr := clusterConditionMgr.setApplyResourcesFailedCondition(err.Error()); componentutil.IgnoreNoOps(setErr) != nil {
			return intctrlutil.RequeueWithError(setErr, reqCtx.Log, "")
		}
		return intctrlutil.RequeueAfter(requeueDuration, reqCtx.Log, "")
	}
	if shouldRequeue {
		if err = r.patchClusterStatus(reqCtx.Ctx, cluster, clusterDeepCopy); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.RequeueAfter(requeueDuration, reqCtx.Log, "")
	}

	// patchClusterCustomLabels if cluster has custom labels.
	if err = r.patchClusterResourceCustomLabels(reqCtx.Ctx, cluster, clusterDefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err = r.handleClusterStatusAfterApplySucceed(ctx, cluster, clusterDeepCopy); componentutil.IgnoreNoOps(err) != nil {
		// REVIEW:
		// caught err being - "clusters.apps.kubeblocks.io \"test-clusterucasrw\" not found",
		// how could this possible
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	// checks if the controller is handling the garbage of restore.
	if err := r.handleGarbageOfRestoreBeforeRunning(ctx, cluster); err == nil {
		return intctrlutil.Reconciled()
	} else if componentutil.IgnoreNoOps(err) != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	// reconcile phase update stage
	// reconcile the phase and conditions of the Cluster.status
	if res, err := r.reconcileClusterStatus(reqCtx, cluster, clusterDefinition); componentutil.IgnoreNoOps(err) != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	} else if res != nil {
		return *res, nil
	}
	return intctrlutil.Reconciled() // caught Starting
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	requeueDuration = time.Millisecond * time.Duration(viper.GetInt(constant.CfgKeyCtrlrReconcileRetryDurationMS))
	// TODO: add filter predicate for core API objects
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Cluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Complete(r)
}

// patchClusterStatus patches the cluster status.
func (r *ClusterReconciler) patchClusterStatus(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterDeepCopy *appsv1alpha1.Cluster) error {
	if reflect.DeepEqual(cluster.Status, clusterDeepCopy.Status) {
		return nil
	}
	patch := client.MergeFrom(clusterDeepCopy)
	return r.Client.Status().Patch(ctx, cluster, patch)
}

// handleClusterStatusAfterApplySucceed when cluster apply resources successful, handle the status
func (r *ClusterReconciler) handleClusterStatusAfterApplySucceed(
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterDeepCopy *appsv1alpha1.Cluster) error {
	applyResourcesCondition := newApplyResourcesCondition()
	oldApplyCondition := meta.FindStatusCondition(cluster.Status.Conditions, applyResourcesCondition.Type)
	meta.SetStatusCondition(&cluster.Status.Conditions, applyResourcesCondition)
	if err := r.patchClusterStatus(ctx, cluster, clusterDeepCopy); err != nil {
		return err
	}
	if oldApplyCondition == nil || oldApplyCondition.Status != applyResourcesCondition.Status {
		r.Recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
	}
	return nil
}

func (r *ClusterReconciler) patchClusterLabelsIfNotExist(
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster) (*ctrl.Result, error) {
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	cdLabelName := cluster.Labels[clusterDefLabelKey]
	cvLabelName := cluster.Labels[clusterVersionLabelKey]
	cdName, cvName := cluster.Spec.ClusterDefRef, cluster.Spec.ClusterVersionRef
	if cdLabelName == cdName && cvLabelName == cvName {
		return nil, nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Labels[clusterDefLabelKey] = cdName
	cluster.Labels[clusterVersionLabelKey] = cvName
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

func (r *ClusterReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) (*ctrl.Result, error) {
	//
	// delete any external resources
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		// if cluster.Status.Phase != appsv1alpha1.DeletingClusterPhase {
		// 	patch := client.MergeFrom(cluster.DeepCopy())
		// 	cluster.Status.ObservedGeneration = cluster.Generation
		// 	// cluster.Status.Message = fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		// 	if err := r.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		// 		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		// 	}
		// }
		// TODO: add warning event
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	case appsv1alpha1.Delete, appsv1alpha1.WipeOut:
		if err := r.deletePVCs(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
		// The backup policy must be cleaned up when the cluster is deleted.
		// Automatic backup scheduling needs to be stopped at this point.
		if err := r.deleteBackupPolicies(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
		if cluster.Spec.TerminationPolicy == appsv1alpha1.WipeOut {
			// TODO check whether delete backups together with cluster is allowed
			// wipe out all backups
			if err := r.deleteBackups(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
				return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
			}
		}
	}

	// it's possible at time of external resource deletion, cluster definition has already been deleted.
	ml := client.MatchingLabels{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppInstanceLabelKey:  cluster.GetName(),
	}
	inNS := client.InNamespace(cluster.Namespace)

	removeFinalizers := func() (*ctrl.Result, error) {
		if ret, err := removeFinalizer(r, reqCtx, generics.StatefulSetSignature, inNS, ml); err != nil {
			return ret, err
		}

		if ret, err := removeFinalizer(r, reqCtx, generics.DeploymentSignature, inNS, ml); err != nil {
			return ret, err
		}

		if ret, err := removeFinalizer(r, reqCtx, generics.ServiceSignature, inNS, ml); err != nil {
			return ret, err
		}

		if ret, err := removeFinalizer(r, reqCtx, generics.SecretSignature, inNS, ml); err != nil {
			return ret, err
		}

		if ret, err := removeFinalizer(r, reqCtx, generics.ConfigMapSignature, inNS, ml); err != nil {
			return ret, err
		}

		if ret, err := removeFinalizer(r, reqCtx, generics.PodDisruptionBudgetSignature, inNS, ml); err != nil {
			return ret, err
		}
		return nil, nil
	}

	deleteResources := func() error {
		if err := r.Client.DeleteAllOf(reqCtx.Ctx, &appsv1.StatefulSet{}, inNS, ml); err != nil {
			return err
		}
		if err := r.Client.DeleteAllOf(reqCtx.Ctx, &appsv1.Deployment{}, inNS, ml); err != nil {
			return err
		}
		if err := r.Client.DeleteAllOf(reqCtx.Ctx, &corev1.Service{}, inNS, ml); err != nil {
			return err
		}
		if err := r.Client.DeleteAllOf(reqCtx.Ctx, &corev1.Secret{}, inNS, ml); err != nil {
			return err
		}
		if err := r.Client.DeleteAllOf(reqCtx.Ctx, &corev1.ConfigMap{}, inNS, ml); err != nil {
			return err
		}
		if err := r.Client.DeleteAllOf(reqCtx.Ctx, &policyv1.PodDisruptionBudget{}, inNS, ml); err != nil {
			return err
		}
		return nil
	}

	// all resources created in reconcileClusterWorkloads should be handled properly
	if ret, err := removeFinalizers(); err != nil {
		return ret, err
	}
	if err := deleteResources(); err != nil {
		return nil, err
	}

	return nil, nil
}

func removeFinalizer[T generics.Object, PT generics.PObject[T],
	L generics.ObjList[T], PL generics.PObjList[T, L]](
	r *ClusterReconciler, reqCtx intctrlutil.RequestCtx, _ func(T, L), opts ...client.ListOption) (*ctrl.Result, error) {
	var (
		objList L
	)
	if err := r.List(reqCtx.Ctx, PL(&objList), opts...); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	for _, obj := range reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T) {
		pobj := PT(&obj)
		if !controllerutil.ContainsFinalizer(pobj, dbClusterFinalizerName) {
			continue
		}
		patch := client.MergeFrom(PT(pobj.DeepCopy()))
		controllerutil.RemoveFinalizer(pobj, dbClusterFinalizerName)
		if err := r.Patch(reqCtx.Ctx, pobj, patch); err != nil {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
	}
	return nil, nil
}

func (r *ClusterReconciler) deletePVCs(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	// it's possible at time of external resource deletion, cluster definition has already been deleted.
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	inNS := client.InNamespace(cluster.Namespace)

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(reqCtx.Ctx, pvcList, inNS, ml); err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if err := r.Delete(reqCtx.Ctx, &pvc); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterReconciler) deleteBackupPolicies(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backupPolicies
	return r.Client.DeleteAllOf(reqCtx.Ctx, &dataprotectionv1alpha1.BackupPolicy{}, inNS, ml)
}

func (r *ClusterReconciler) deleteBackups(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backups
	backups := &dataprotectionv1alpha1.BackupList{}
	if err := r.List(reqCtx.Ctx, backups, inNS, ml); err != nil {
		return err
	}
	for _, backup := range backups.Items {
		// check backup delete protection label
		deleteProtection, exists := backup.GetLabels()[constant.BackupProtectionLabelKey]
		// not found backup-protection or value is Delete, delete it.
		if !exists || deleteProtection == constant.BackupDelete {
			if err := r.Delete(reqCtx.Ctx, &backup); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkReferencingCRStatus checks if cluster referenced CR is available
func (r *ClusterReconciler) checkReferencedCRStatus(
	reqCtx intctrlutil.RequestCtx,
	conMgr clusterConditionManager,
	referencedCRPhase appsv1alpha1.Phase,
	crKind, crName string) (*ctrl.Result, error) {
	if referencedCRPhase == appsv1alpha1.AvailablePhase {
		return nil, nil
	}
	message := fmt.Sprintf("%s: %s is unavailable, this problem needs to be solved first.", crKind, crName)
	if err := conMgr.setReferenceCRUnavailableCondition(message); componentutil.IgnoreNoOps(err) != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.RequeueAfter(requeueDuration, reqCtx.Log, ""))
}

// updateClusterPhaseWithOperations updates cluster.status.phase according to operations
// REVIEW: need to refactor out this function
// updateClusterPhase updates cluster.status.phase
// Deprecated:
func (r *ClusterReconciler) updateClusterPhaseWithOperations(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) (*reconcile.Result, error) {
	oldClusterPhase := cluster.Status.Phase
	patch := client.MergeFrom(cluster.DeepCopy())
	if oldClusterPhase == cluster.Status.Phase {
		return nil, nil
	}
	if err := r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return nil, err
	}
	// send an event when cluster perform operations
	r.Recorder.Eventf(cluster, corev1.EventTypeNormal, string(cluster.Status.Phase),
		"Start %s in Cluster: %s", cluster.Status.Phase, cluster.Name)
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// REVIEW: this handling rather monolithic
// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
// @return ErrNoOps if no operation
// Deprecated:
func (r *ClusterReconciler) reconcileClusterStatus(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition) (*reconcile.Result, error) {
	if len(cluster.Status.Components) == 0 {
		return nil, nil
	}

	var (
		currentClusterPhase       appsv1alpha1.ClusterPhase
		existsAbnormalOrFailed    bool
		notReadyCompNames         = map[string]struct{}{}
		replicasNotReadyCompNames = map[string]struct{}{}
	)

	// analysis the status of components and calculate the cluster phase .
	analysisComponentsStatus := func(cluster *appsv1alpha1.Cluster) {
		var (
			runningCompCount int
			stoppedCompCount int
		)
		for k, v := range cluster.Status.Components {
			if v.PodsReady == nil || !*v.PodsReady {
				replicasNotReadyCompNames[k] = struct{}{}
				notReadyCompNames[k] = struct{}{}
			}
			switch v.Phase {
			case appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.FailedClusterCompPhase:
				existsAbnormalOrFailed = true
				notReadyCompNames[k] = struct{}{}
			case appsv1alpha1.RunningClusterCompPhase:
				runningCompCount += 1
			case appsv1alpha1.StoppedClusterCompPhase:
				stoppedCompCount += 1
			}
		}
		compLen := len(cluster.Status.Components)
		notReadyLen := len(notReadyCompNames)
		if existsAbnormalOrFailed && notReadyLen > 0 {
			if compLen == notReadyLen {
				currentClusterPhase = appsv1alpha1.FailedClusterPhase
			} else {
				currentClusterPhase = appsv1alpha1.AbnormalClusterPhase
			}
			return
		}
		switch len(cluster.Status.Components) {
		case 0:
			// if no components, return, and how could this possible?
			return
		case runningCompCount:
			currentClusterPhase = appsv1alpha1.RunningClusterPhase
		case stoppedCompCount:
			// cluster is Stopped when cluster is not Running and all components are Stopped or Running
			currentClusterPhase = appsv1alpha1.StoppedClusterPhase
		}
	}

	// remove the invalid component in status.components when spec.components changed and analysis the status of components.
	removeInvalidComponentsAndAnalysis := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		tmpCompsStatus := map[string]appsv1alpha1.ClusterComponentStatus{}
		compsStatus := cluster.Status.Components
		for _, v := range cluster.Spec.ComponentSpecs {
			if compStatus, ok := compsStatus[v.Name]; ok {
				tmpCompsStatus[v.Name] = compStatus
			}
		}
		if len(tmpCompsStatus) != len(compsStatus) {
			// keep valid components' status
			cluster.Status.Components = tmpCompsStatus
			return nil, nil
		}
		analysisComponentsStatus(cluster)
		return nil, componentutil.ErrNoOps
	}

	// handle the cluster conditions with ClusterReady and ReplicasReady type.
	handleClusterReadyCondition := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		return handleNotReadyConditionForCluster(cluster, r.Recorder, replicasNotReadyCompNames, notReadyCompNames)
	}

	// processes cluster phase changes.
	processClusterPhaseChanges := func(cluster *appsv1alpha1.Cluster,
		oldPhase,
		currPhase appsv1alpha1.ClusterPhase,
		eventType string,
		eventMessage string,
		doAction func(cluster *appsv1alpha1.Cluster)) (postHandler, error) {
		if oldPhase == currPhase {
			return nil, componentutil.ErrNoOps
		}
		cluster.Status.Phase = currPhase
		if doAction != nil {
			doAction(cluster)
		}
		postFuncAfterPatch := func(currCluster *appsv1alpha1.Cluster) error {
			r.Recorder.Event(currCluster, eventType, string(currPhase), eventMessage)
			return opsutil.MarkRunningOpsRequestAnnotation(reqCtx.Ctx, r.Client, currCluster)
		}
		return postFuncAfterPatch, nil
	}
	// handle the Cluster.status when some components of cluster are Abnormal or Failed.
	handleExistAbnormalOrFailed := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		if !existsAbnormalOrFailed {
			return nil, componentutil.ErrNoOps
		}
		oldPhase := cluster.Status.Phase
		componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster,
			clusterDef, "")
		// handle the cluster status when some components are not ready.
		handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
		currPhase := cluster.Status.Phase
		if !slices.Contains(appsv1alpha1.GetClusterFailedPhases(), currPhase) {
			return nil, componentutil.ErrNoOps
		}
		message := fmt.Sprintf("Cluster: %s is %s, check according to the components message",
			cluster.Name, currPhase)
		return processClusterPhaseChanges(cluster, oldPhase, currPhase,
			corev1.EventTypeWarning, message, nil)
	}

	// handle the Cluster.status when cluster is Stopped.
	handleClusterIsStopped := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		if currentClusterPhase != appsv1alpha1.StoppedClusterPhase {
			return nil, componentutil.ErrNoOps
		}
		message := fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase,
			corev1.EventTypeNormal, message, nil)
	}

	// handle the Cluster.status when cluster is Running.
	handleClusterIsRunning := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		if currentClusterPhase != appsv1alpha1.RunningClusterPhase {
			return nil, componentutil.ErrNoOps
		}
		message := fmt.Sprintf("Cluster: %s is ready, current phase is Running.", cluster.Name)
		action := func(currCluster *appsv1alpha1.Cluster) {
			meta.SetStatusCondition(&currCluster.Status.Conditions,
				newClusterReadyCondition(currCluster.Name))
		}
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase,
			corev1.EventTypeNormal, message, action)
	}
	if err := doChainClusterStatusHandler(reqCtx.Ctx, r.Client, cluster,
		removeInvalidComponentsAndAnalysis,
		handleClusterReadyCondition,
		handleExistAbnormalOrFailed,
		handleClusterIsStopped,
		handleClusterIsRunning); err != nil {
		return nil, err
	}
	return nil, nil
}

// cleanupAnnotationsAfterRunning cleans up the cluster annotations after cluster is Running.
func (r *ClusterReconciler) cleanupAnnotationsAfterRunning(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	if _, ok := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]; !ok {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
	return r.Client.Patch(reqCtx.Ctx, cluster, patch)
}

// REVIEW: this handling is rather hackish, call for refactor.
// handleRestoreGarbageBeforeRunning handles the garbage for restore before cluster phase changes to Running.
// @return ErrNoOps if no operation
// Deprecated: to be removed by PITR feature.
func (r *ClusterReconciler) handleGarbageOfRestoreBeforeRunning(ctx context.Context, cluster *appsv1alpha1.Cluster) error {
	clusterBackupResourceMap, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return err
	}
	if clusterBackupResourceMap == nil {
		return componentutil.ErrNoOps
	}
	// check if all components are running.
	for _, v := range cluster.Status.Components {
		if v.Phase != appsv1alpha1.RunningClusterCompPhase {
			return componentutil.ErrNoOps
		}
	}
	// remove the garbage for restore if the cluster restores from backup.
	return r.removeGarbageWithRestore(ctx, cluster, clusterBackupResourceMap)
}

// REVIEW: this handling is rather hackish, call for refactor.
// removeGarbageWithRestore removes the garbage for restore when all components are Running.
// @return ErrNoOps if no operation
// Deprecated:
func (r *ClusterReconciler) removeGarbageWithRestore(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterBackupResourceMap map[string]string) error {
	var (
		doRemoveInitContainers bool
		err                    error
	)
	clusterPatch := client.MergeFrom(cluster.DeepCopy())
	for k, v := range clusterBackupResourceMap {
		// remove the init container for restore
		if doRemoveInitContainers, err = r.removeStsInitContainerForRestore(ctx, cluster, k, v); err != nil {
			return err
		}
	}
	if doRemoveInitContainers {
		// reset the component phase to Creating during removing the init containers of statefulSet.
		return r.Client.Status().Patch(ctx, cluster, clusterPatch)
	}
	return componentutil.ErrNoOps
}

// removeStsInitContainerForRestore removes the statefulSet's init container which restores data from backup.
func (r *ClusterReconciler) removeStsInitContainerForRestore(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	componentName,
	backupName string) (bool, error) {
	// get the sts list of component
	stsList := &appsv1.StatefulSetList{}
	if err := componentutil.GetObjectListByComponentName(ctx, r.Client, *cluster, stsList, componentName); err != nil {
		return false, err
	}
	var doRemoveInitContainers bool
	for _, sts := range stsList.Items {
		initContainers := sts.Spec.Template.Spec.InitContainers
		restoreInitContainerName := component.GetRestoredInitContainerName(backupName)
		restoreInitContainerIndex, _ := intctrlutil.GetContainerByName(initContainers, restoreInitContainerName)
		if restoreInitContainerIndex == -1 {
			continue
		}
		doRemoveInitContainers = true
		initContainers = append(initContainers[:restoreInitContainerIndex], initContainers[restoreInitContainerIndex+1:]...)
		sts.Spec.Template.Spec.InitContainers = initContainers
		if err := r.Client.Update(ctx, &sts); err != nil {
			return false, err
		}
	}
	if doRemoveInitContainers {
		// if need to remove init container, reset component to Creating.
		compStatus := cluster.Status.Components[componentName]
		compStatus.Phase = appsv1alpha1.StartingClusterCompPhase
		cluster.Status.SetComponentStatus(componentName, compStatus)
	}
	return doRemoveInitContainers, nil
}

// patchClusterResourceCustomLabels patches the custom labels to GVR(Group/Version/Resource) defined in the cluster spec.
func (r *ClusterReconciler) patchClusterResourceCustomLabels(ctx context.Context, cluster *appsv1alpha1.Cluster, clusterDef *appsv1alpha1.ClusterDefinition) error {
	if cluster == nil || clusterDef == nil {
		return nil
	}
	// patch the custom label defined in clusterDefinition.spec.componentDefs[x].customLabelSpecs to the component resource.
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compDef := clusterDef.GetComponentDefByName(compSpec.ComponentDefRef)
		for _, customLabelSpec := range compDef.CustomLabelSpecs {
			// TODO if the customLabelSpec.Resources is empty, we should add the label to all the resources under the component.
			for _, resource := range customLabelSpec.Resources {
				if err := componentutil.PatchGVRCustomLabels(ctx, r.Client, cluster, resource, compSpec.Name, customLabelSpec.Key, customLabelSpec.Value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Handle is the event handler for the cluster status event.
func (r *ClusterStatusEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != constant.ProbeCheckRolePath {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}

	// parse probe event message when field path is probe-role-changed-check
	message := k8score.ParseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return nil
	}

	// if probe message event is checkRoleFailed, it means the cluster is abnormal, need to handle the cluster status
	if message.Event == k8score.ProbeEventCheckRoleFailed {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}
	return nil
}

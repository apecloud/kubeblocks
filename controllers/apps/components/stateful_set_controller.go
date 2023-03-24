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

package components

import (
	"context"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	uitl "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	componentutil "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// StatefulSetReconciler reconciles a statefulset object
type StatefulSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		sts = &appsv1.StatefulSet{}
		err error
	)

	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("statefulSet", req.NamespacedName),
	}

	if err = r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, sts); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, sts, apps.DBClusterFinalizerName, func() (*ctrl.Result, error) {
		return r.deleteExternalResources(reqCtx, sts)
	})
	if res != nil {
		return *res, err
	}

	return workloadCompClusterReconcile(reqCtx, r.Client, sts,
		func(cluster *appsv1alpha1.Cluster, componentSpec *appsv1alpha1.ClusterComponentSpec, component types.Component) (ctrl.Result, error) {
			compCtx := newComponentContext(reqCtx, r.Client, r.Recorder, component, sts, componentSpec)
			reqCtx.Log.V(1).Info("before updateComponentStatusInClusterStatus",
				"generation", sts.Generation, "observed generation", sts.Status.ObservedGeneration,
				"replicas", sts.Status.Replicas)
			if requeueAfter, err := updateComponentStatusInClusterStatus(compCtx, cluster); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			} else if requeueAfter != 0 {
				// if the reconcileAction need requeue, do it
				return intctrlutil.RequeueAfter(requeueAfter, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		})
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.WorkloadFilterPredicate)).
		Complete(r)
}

func (r *StatefulSetReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, sts *appsv1.StatefulSet) (*ctrl.Result, error) {
	// TODO: hack delete postgres patroni configmap here, should be refactored
	return deletePostgresPatroniConfigMap(reqCtx, r.Client, sts)
}

func deletePostgresPatroniConfigMap(reqCtx intctrlutil.RequestCtx, cli client.Client, sts *appsv1.StatefulSet) (*ctrl.Result, error) {
	stsLabels := sts.GetLabels()
	clusterName := stsLabels[constant.AppInstanceLabelKey]
	componentName := stsLabels[constant.KBAppComponentLabelKey]

	var cluster *appsv1alpha1.Cluster
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: sts.Namespace, Name: clusterName}, cluster); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	// if the cluster is not being deleted, skip deleting postgres patroni configmap
	if cluster != nil {
		if cluster.DeletionTimestamp != nil {
			res, err := intctrlutil.CheckedRequeueWithError(errors.New("cluster is not being deleted, skip deleting postgres patroni configmap"), reqCtx.Log, "")
			return &res, err
		}
	}

	// we should check there is no pod under the component before deleting the configmap, otherwise the patroni configMap will be recreated by postgres Pod.
	podList := &corev1.PodList{}
	if err := cli.List(reqCtx.Ctx, podList, client.InNamespace(sts.Namespace), uitl.GetComponentMatchLabels(clusterName, componentName)); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	if len(podList.Items) > 0 {
		res, err := intctrlutil.CheckedRequeueWithError(errors.New("the component has pods, skip deleting postgres patroni configmap"), reqCtx.Log, "")
		return &res, err
	}

	var patroniConfigMapNameList []string
	builtInEnvMap := componentutil.GetReplacementMapForBuiltInEnv(clusterName, componentName)
	for _, patroniConfigMap := range []string{
		PgPatroniConfigMapLeaderPlaceHolder,
		PgPatroniConfigMapConfigPlaceHolder,
		PgPatroniConfigMapFailoverPlaceHolder,
	} {
		patroniConfigMap = componentutil.ReplaceNamedVars(builtInEnvMap, patroniConfigMap, -1, true)
		patroniConfigMapNameList = append(patroniConfigMapNameList, patroniConfigMap)
	}

	for _, patroniConfigMapName := range patroniConfigMapNameList {
		patroniConfigMap := &corev1.ConfigMap{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: sts.Namespace, Name: patroniConfigMapName}, patroniConfigMap); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
		if patroniConfigMap == nil {
			continue
		}
		reqCtx.Recorder.Eventf(sts, corev1.EventTypeNormal, "Deleting", "Deleting Patroni ConfigMap %s", patroniConfigMap.Name)
		if err := cli.Delete(reqCtx.Ctx, patroniConfigMap); err != nil || !apierrors.IsNotFound(err) {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}
	return nil, nil
}

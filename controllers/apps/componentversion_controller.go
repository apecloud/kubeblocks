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

package apps

import (
	"context"
	"fmt"
	"slices"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ComponentVersionReconciler reconciles a ComponentVersion object
type ComponentVersionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentversions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ComponentVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("componentVersion", req.NamespacedName),
		Recorder: r.Recorder,
	}

	rctx.Log.V(1).Info("reconcile", "componentVersion", req.NamespacedName)

	compVersion := &appsv1alpha1.ComponentVersion{}
	if err := r.Client.Get(rctx.Ctx, rctx.Req.NamespacedName, compVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	return r.reconcile(rctx, compVersion)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ComponentVersion{}).
		Complete(r)
}

func (r *ComponentVersionReconciler) reconcile(rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, compVersion, componentVersionFinalizerName, r.deletionHandler(rctx, compVersion))
	if res != nil {
		return *res, err
	}

	if compVersion.Status.ObservedGeneration == compVersion.Generation &&
		slices.Contains([]appsv1alpha1.Phase{appsv1alpha1.AvailablePhase}, compVersion.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	rules, err := r.buildCompatibilityRules(r.Client, rctx, compVersion)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	if err = r.validate(compVersion, rules); err != nil {
		if err1 := r.unavailable(r.Client, rctx, compVersion, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	err = r.updateLabels(r.Client, rctx, compVersion, rules)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	err = r.available(r.Client, rctx, compVersion)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, compVersion)

	return intctrlutil.Reconciled()
}

func (r *ComponentVersionReconciler) buildCompatibilityRules(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion) (map[*appsv1alpha1.ComponentDefinition][]string, error) {
	compDefs := make(map[*appsv1alpha1.ComponentDefinition][]string)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		for _, compDef := range rule.CompDefs {
			equal := func(cmpd *appsv1alpha1.ComponentDefinition) bool {
				return cmpd.Name == compDef
			}
			if slices.IndexFunc(maps.Keys(compDefs), equal) >= 0 {
				continue
			}
			cmpd := &appsv1alpha1.ComponentDefinition{}
			key := types.NamespacedName{
				Name: compDef,
			}
			if err := cli.Get(rctx.Ctx, key, cmpd); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return nil, err
			}
			compDefs[cmpd] = rule.Versions
		}
	}
	return compDefs, nil
}

func (r *ComponentVersionReconciler) deletionHandler(rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(compVersion, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, compVersion, constant.ComponentVersionLabelKey,
			recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ComponentVersionReconciler) available(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion) error {
	return r.status(cli, rctx, compVersion, appsv1alpha1.AvailablePhase, "")
}

func (r *ComponentVersionReconciler) unavailable(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion, err error) error {
	return r.status(cli, rctx, compVersion, appsv1alpha1.UnavailablePhase, err.Error())
}

func (r *ComponentVersionReconciler) status(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion, phase appsv1alpha1.Phase, message string) error {
	patch := client.MergeFrom(compVersion.DeepCopy())
	compVersion.Status.ObservedGeneration = compVersion.Generation
	compVersion.Status.Phase = phase
	compVersion.Status.Message = message
	return cli.Status().Patch(rctx.Ctx, compVersion, patch)
}

func (r *ComponentVersionReconciler) updateLabels(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion, rules map[*appsv1alpha1.ComponentDefinition][]string) error {
	updated := false
	if compVersion.Labels == nil {
		compVersion.Labels = make(map[string]string)
	}
	for cmpd, _ := range rules {
		if _, ok := compVersion.Labels[cmpd.Name]; ok {
			continue
		}
		compVersion.Labels[cmpd.Name] = cmpd.Name
		updated = true
	}
	if updated {
		return cli.Update(rctx.Ctx, compVersion)
	}
	return nil
}

func (r *ComponentVersionReconciler) validate(compVersion *appsv1alpha1.ComponentVersion,
	rules map[*appsv1alpha1.ComponentDefinition][]string) error {
	for _, release := range compVersion.Spec.Releases {
		if err := r.validateRelease(release, rules); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentVersionReconciler) validateRelease(release appsv1alpha1.ComponentVersionRelease,
	rules map[*appsv1alpha1.ComponentDefinition][]string) error {
	for cmpd, versions := range rules {
		if !slices.Contains(versions, release.Version) {
			continue
		}
		for _, app := range release.Apps {
			if err := r.validateApp(*cmpd, app); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ComponentVersionReconciler) validateApp(cmpd appsv1alpha1.ComponentDefinition,
	app appsv1alpha1.ComponentAppVersion) error {
	cmp := func(c corev1.Container) bool {
		return c.Name == app.Name
	}
	if slices.IndexFunc(cmpd.Spec.Runtime.InitContainers, cmp) != -1 {
		return nil
	}
	if slices.IndexFunc(cmpd.Spec.Runtime.Containers, cmp) != -1 {
		return nil
	}
	return fmt.Errorf("app/container %s is not found in ComponentDefinition %s", app.Name, cmpd.Name)
}

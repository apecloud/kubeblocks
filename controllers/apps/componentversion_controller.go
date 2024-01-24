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
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
		Watches(&appsv1alpha1.ComponentDefinition{}, handler.EnqueueRequestsFromMapFunc(r.compatibleCompVersion)).
		Complete(r)
}

func (r *ComponentVersionReconciler) compatibleCompVersion(ctx context.Context, obj client.Object) []reconcile.Request {
	compDef, ok := obj.(*appsv1alpha1.ComponentDefinition)
	if !ok {
		return nil
	}
	versions := &appsv1alpha1.ComponentVersionList{}
	if err := r.Client.List(ctx, versions); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for _, compVersion := range versions.Items {
		if r.isCompatibleWith(*compDef, compVersion) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: compVersion.Name,
				},
			})
		}
	}
	return requests
}

func (r *ComponentVersionReconciler) isCompatibleWith(compDef appsv1alpha1.ComponentDefinition, compVer appsv1alpha1.ComponentVersion) bool {
	for _, rule := range compVer.Spec.CompatibilityRules {
		for _, name := range rule.CompDefs {
			if strings.HasPrefix(compDef.Name, name) {
				return true
			}
		}
	}
	return false
}

func (r *ComponentVersionReconciler) reconcile(rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, compVersion, componentVersionFinalizerName, r.deletionHandler(rctx, compVersion))
	if res != nil {
		return *res, err
	}

	// if compVersion.Status.ObservedGeneration == compVersion.Generation &&
	//	slices.Contains([]appsv1alpha1.Phase{appsv1alpha1.AvailablePhase}, compVersion.Status.Phase) {
	//	return intctrlutil.Reconciled()
	// }

	releaseToCompDefinitions, err := r.buildReleaseToCompDefinitionMapping(r.Client, rctx, compVersion)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	if err = r.validate(compVersion, releaseToCompDefinitions); err != nil {
		if err1 := r.unavailable(r.Client, rctx, compVersion, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	// patch the supported component definitions as labels to the object.
	err = r.updateSupportedCompDefLabels(r.Client, rctx, compVersion, releaseToCompDefinitions)
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

func (r *ComponentVersionReconciler) buildReleaseToCompDefinitionMapping(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion) (map[string]map[string]*appsv1alpha1.ComponentDefinition, error) {
	compDefs := make(map[string]*appsv1alpha1.ComponentDefinition)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		for _, compDef := range rule.CompDefs {
			if _, ok := compDefs[compDef]; ok {
				continue
			}
			cmpd := &appsv1alpha1.ComponentDefinition{}
			key := types.NamespacedName{
				Name: compDef, // TODO: wildcard
			}
			err := cli.Get(rctx.Ctx, key, cmpd)
			switch {
			case err != nil && !apierrors.IsNotFound(err):
				return nil, err
			case err != nil:
				compDefs[compDef] = nil
			default:
				compDefs[compDef] = cmpd
			}
		}
	}
	releaseToCompDefinitions := make(map[string]map[string]*appsv1alpha1.ComponentDefinition)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		for _, release := range rule.Releases {
			if _, ok := releaseToCompDefinitions[release]; !ok {
				releaseToCompDefinitions[release] = map[string]*appsv1alpha1.ComponentDefinition{}
			}
			for _, compDef := range rule.CompDefs {
				if _, ok := releaseToCompDefinitions[release][compDef]; ok {
					continue
				}
				releaseToCompDefinitions[release][compDef] = compDefs[compDef]
			}
		}
	}
	return releaseToCompDefinitions, nil
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
	compVersion.Status.ServiceVersions = r.supportedServiceVersions(compVersion)
	return cli.Status().Patch(rctx.Ctx, compVersion, patch)
}

func (r *ComponentVersionReconciler) supportedServiceVersions(compVersion *appsv1alpha1.ComponentVersion) string {
	versions := map[string]bool{}
	for _, release := range compVersion.Spec.Releases {
		if len(release.ServiceVersion) > 0 {
			versions[release.ServiceVersion] = true
		}
	}
	keys := maps.Keys(versions)
	slices.Sort(keys)
	return strings.Join(keys, ",") // TODO
}

func (r *ComponentVersionReconciler) updateSupportedCompDefLabels(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1alpha1.ComponentVersion, releaseToCompDefinitions map[string]map[string]*appsv1alpha1.ComponentDefinition) error {
	updated := false
	if compVersion.Labels == nil {
		compVersion.Labels = make(map[string]string)
	}
	for _, compDefs := range releaseToCompDefinitions {
		for name := range compDefs {
			if _, ok := compVersion.Labels[name]; ok {
				continue
			}
			compVersion.Labels[name] = name
			updated = true
		}
	}
	if updated {
		return cli.Update(rctx.Ctx, compVersion)
	}
	return nil
}

func (r *ComponentVersionReconciler) validate(compVersion *appsv1alpha1.ComponentVersion,
	releaseToCompDefinitions map[string]map[string]*appsv1alpha1.ComponentDefinition) error {
	for _, release := range compVersion.Spec.Releases {
		if err := r.validateRelease(release, releaseToCompDefinitions); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentVersionReconciler) validateRelease(release appsv1alpha1.ComponentVersionRelease,
	releaseToCompDefinitions map[string]map[string]*appsv1alpha1.ComponentDefinition) error {
	cmpds, ok := releaseToCompDefinitions[release.Name]
	if !ok {
		return fmt.Errorf("release %s has no any supported ComponentDefinition", release.Name)
	}
	for name := range release.Images {
		for _, cmpd := range cmpds {
			if err := r.validateContainer(*cmpd, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ComponentVersionReconciler) validateContainer(cmpd appsv1alpha1.ComponentDefinition, name string) error {
	cmp := func(c corev1.Container) bool {
		return c.Name == name
	}
	if slices.IndexFunc(cmpd.Spec.Runtime.InitContainers, cmp) != -1 {
		return nil
	}
	if slices.IndexFunc(cmpd.Spec.Runtime.Containers, cmp) != -1 {
		return nil
	}
	return fmt.Errorf("container %s is not found in ComponentDefinition %s", name, cmpd.Name)
}

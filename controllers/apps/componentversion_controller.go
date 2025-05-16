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

package apps

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

const (
	componentVersionFinalizerName = "componentversion.kubeblocks.io/finalizer"
	compatibleDefinitionsKey      = "componentversion.kubeblocks.io/compatible-definitions"
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

	compVersion := &appsv1.ComponentVersion{}
	if err := r.Client.Get(rctx.Ctx, rctx.Req.NamespacedName, compVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	if !intctrlutil.ObjectAPIVersionSupported(compVersion) {
		return intctrlutil.Reconciled()
	}

	return r.reconcile(rctx, compVersion)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.ComponentVersion{}).
		Watches(&appsv1.ComponentDefinition{}, handler.EnqueueRequestsFromMapFunc(r.compatibleCompVersion)).
		Complete(r)
}

func (r *ComponentVersionReconciler) compatibleCompVersion(ctx context.Context, obj client.Object) []reconcile.Request {
	compDef, ok := obj.(*appsv1.ComponentDefinition)
	if !ok {
		return nil
	}
	versions := &appsv1.ComponentVersionList{}
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

func (r *ComponentVersionReconciler) isCompatibleWith(compDef appsv1.ComponentDefinition, compVer appsv1.ComponentVersion) bool {
	for _, rule := range compVer.Spec.CompatibilityRules {
		for _, name := range rule.CompDefs {
			if component.PrefixOrRegexMatched(compDef.Name, name) {
				return true
			}
		}
	}
	return false
}

func (r *ComponentVersionReconciler) reconcile(rctx intctrlutil.RequestCtx,
	compVersion *appsv1.ComponentVersion) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, compVersion, componentVersionFinalizerName, r.deletionHandler(rctx, compVersion))
	if res != nil {
		return *res, err
	}

	// if compVersion.Status.ObservedGeneration == compVersion.Generation &&
	//	slices.Contains([]appsv1.Phase{appsv1.AvailablePhase}, compVersion.Status.Phase) {
	//	return intctrlutil.Reconciled()
	// }

	if err = validateCompatibilityRulesCompDef(compVersion); err != nil {
		if err1 := r.unavailable(r.Client, rctx, compVersion, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

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
	compVersion *appsv1.ComponentVersion) (map[string]map[string]*appsv1.ComponentDefinition, error) {
	compDefs := make(map[string][]*appsv1.ComponentDefinition)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		for _, compDef := range rule.CompDefs {
			if _, ok := compDefs[compDef]; ok {
				continue
			}
			var err error
			compDefs[compDef], err = listCompDefinitionsWithPattern(rctx.Ctx, cli, compDef)
			if err != nil {
				return nil, err
			}
		}
	}
	releaseToCompDefinitions := make(map[string]map[string]*appsv1.ComponentDefinition)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		for _, release := range rule.Releases {
			if _, ok := releaseToCompDefinitions[release]; !ok {
				releaseToCompDefinitions[release] = map[string]*appsv1.ComponentDefinition{}
			}
			for _, compDefName := range rule.CompDefs {
				for i, compDef := range compDefs[compDefName] {
					if _, ok := releaseToCompDefinitions[release][compDef.Name]; ok {
						continue
					}
					releaseToCompDefinitions[release][compDef.Name] = compDefs[compDefName][i]
				}
			}
		}
	}
	return releaseToCompDefinitions, nil
}

func (r *ComponentVersionReconciler) deletionHandler(rctx intctrlutil.RequestCtx,
	compVersion *appsv1.ComponentVersion) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(compVersion, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, compVersion, constant.ComponentVersionLabelKey,
			recordEvent, &appsv1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ComponentVersionReconciler) available(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1.ComponentVersion) error {
	return r.status(cli, rctx, compVersion, appsv1.AvailablePhase, "")
}

func (r *ComponentVersionReconciler) unavailable(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1.ComponentVersion, err error) error {
	return r.status(cli, rctx, compVersion, appsv1.UnavailablePhase, err.Error())
}

func (r *ComponentVersionReconciler) status(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1.ComponentVersion, phase appsv1.Phase, message string) error {
	patch := client.MergeFrom(compVersion.DeepCopy())
	compVersion.Status.ObservedGeneration = compVersion.Generation
	compVersion.Status.Phase = phase
	compVersion.Status.Message = message
	compVersion.Status.ServiceVersions = r.supportedServiceVersions(compVersion)
	return cli.Status().Patch(rctx.Ctx, compVersion, patch)
}

func (r *ComponentVersionReconciler) supportedServiceVersions(compVersion *appsv1.ComponentVersion) string {
	versionSet := sets.New[string]()
	for _, release := range compVersion.Spec.Releases {
		if err := validateServiceVersion(release.ServiceVersion); err == nil {
			versionSet.Insert(release.ServiceVersion)
		}
	}
	versions := versionSet.UnsortedList()
	slices.SortFunc(versions, func(a, b string) int {
		return serviceVersionComparator(a, b) * -1
	})
	return strings.Join(versions, ",") // TODO(API): service versions length
}

func (r *ComponentVersionReconciler) updateSupportedCompDefLabels(cli client.Client, rctx intctrlutil.RequestCtx,
	compVersion *appsv1.ComponentVersion, releaseToCompDefinitions map[string]map[string]*appsv1.ComponentDefinition) error {
	if compVersion.Annotations == nil {
		compVersion.Annotations = make(map[string]string)
	}
	compatibleDefs := strings.Split(compVersion.Annotations[compatibleDefinitionsKey], ",")

	labels := make(map[string]string)
	for _, compDefs := range releaseToCompDefinitions {
		for name := range compDefs {
			labels[name] = name
		}
	}
	labelKeys := maps.Keys(labels)
	slices.Sort(labelKeys)

	// nothing changed
	if reflect.DeepEqual(compatibleDefs, labelKeys) {
		return nil
	}

	compatibleDefsSet := sets.New(compatibleDefs...)
	for k, v := range compVersion.Labels {
		if !compatibleDefsSet.Has(k) {
			labels[k] = v // add non-definition labels back
		}
	}
	compVersion.Labels = labels
	compVersion.Annotations[compatibleDefinitionsKey] = strings.Join(labelKeys, ",")
	return cli.Update(rctx.Ctx, compVersion)
}

func (r *ComponentVersionReconciler) validate(compVersion *appsv1.ComponentVersion,
	releaseToCompDefinitions map[string]map[string]*appsv1.ComponentDefinition) error {
	for _, release := range compVersion.Spec.Releases {
		if err := r.validateRelease(release, releaseToCompDefinitions); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentVersionReconciler) validateRelease(release appsv1.ComponentVersionRelease,
	releaseToCompDefinitions map[string]map[string]*appsv1.ComponentDefinition) error {
	cmpds, ok := releaseToCompDefinitions[release.Name]
	notNil := func(cmpd *appsv1.ComponentDefinition) bool {
		return cmpd != nil
	}
	if !ok || generics.CountFunc(maps.Values(cmpds), notNil) == 0 {
		return fmt.Errorf("release %s has no any supported ComponentDefinition", release.Name)
	}
	if err := r.validateServiceVersion(release); err != nil {
		return err
	}
	if err := r.validateImages(release, cmpds); err != nil {
		return err
	}
	return nil
}

func (r *ComponentVersionReconciler) validateServiceVersion(release appsv1.ComponentVersionRelease) error {
	return validateServiceVersion(release.ServiceVersion)
}

func (r *ComponentVersionReconciler) validateImages(release appsv1.ComponentVersionRelease, cmpds map[string]*appsv1.ComponentDefinition) error {
	for name := range release.Images {
		for _, cmpd := range cmpds {
			if cmpd == nil {
				continue
			}
			if err := r.validateImageContainer(*cmpd, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ComponentVersionReconciler) validateImageContainer(cmpd appsv1.ComponentDefinition, name string) error {
	if r.imageDefinedInContainers(cmpd, name) {
		return nil
	}
	if r.imageDefinedInActions(cmpd, name) {
		return nil
	}
	// user-managed images, leave it to the user to handle
	return nil
}

func (r *ComponentVersionReconciler) imageDefinedInContainers(cmpd appsv1.ComponentDefinition, name string) bool {
	cmp := func(c corev1.Container) bool {
		return c.Name == name
	}
	if slices.IndexFunc(cmpd.Spec.Runtime.InitContainers, cmp) != -1 {
		return true
	}
	if slices.IndexFunc(cmpd.Spec.Runtime.Containers, cmp) != -1 {
		return true
	}
	return false
}

func (r *ComponentVersionReconciler) imageDefinedInActions(_ appsv1.ComponentDefinition, name string) bool {
	match := func(action string) bool {
		// case insensitive
		return strings.EqualFold(action, name)
	}

	tp := reflect.TypeOf(appsv1.ComponentLifecycleActions{})
	for i := 0; i < tp.NumField(); i++ {
		if match(tp.Field(i).Name) {
			return true
		}
	}
	return false
}

// validateCompatibilityRulesCompDef validates the reference component definition name pattern defined in compatibility rules.
func validateCompatibilityRulesCompDef(compVersion *appsv1.ComponentVersion) error {
	for _, rule := range compVersion.Spec.CompatibilityRules {
		for _, compDefName := range rule.CompDefs {
			if err := component.ValidateDefNameRegexp(compDefName); err != nil {
				return errors.Wrapf(err, "invalid reference to component definition name pattern: %s in compatibility rules", compDefName)
			}
		}
	}
	return nil
}

func serviceVersionComparator(a, b string) int {
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	v, _ := version.ParseSemantic(a)
	ret, _ := v.Compare(b)
	return ret
}

// validateServiceVersion checks whether the service version is a valid semantic version.
func validateServiceVersion(v string) error {
	if len(v) == 0 {
		return nil
	}
	_, err := version.ParseSemantic(v)
	return err
}

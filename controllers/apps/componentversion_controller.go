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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
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
	notNil := func(cmpd *appsv1alpha1.ComponentDefinition) bool {
		return cmpd != nil
	}
	if !ok || generics.CountFunc(maps.Values(cmpds), notNil) == 0 {
		return fmt.Errorf("release %s has no any supported ComponentDefinition", release.Name)
	}
	for name := range release.Images {
		for _, cmpd := range cmpds {
			if cmpd == nil {
				continue
			}
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

func resolveCompDefinitionNServiceVersion(ctx context.Context, cli client.Reader, compDefName, serviceVersion string) (*appsv1alpha1.ComponentDefinition, string, error) {
	var (
		compDef *appsv1alpha1.ComponentDefinition
	)
	compDefs, err := listCompDefinitionsWithPrefix(ctx, cli, compDefName)
	if err != nil {
		return compDef, serviceVersion, err
	}

	serviceVersionToCompDefs, err := buildServiceVersionToCompDefsMapping(ctx, cli, compDefs, serviceVersion)
	if err != nil {
		return compDef, serviceVersion, err
	}

	// use specified service version or the latest.
	if len(serviceVersion) == 0 {
		serviceVersions := maps.Keys(serviceVersionToCompDefs)
		slices.Sort(serviceVersions)
		serviceVersion = serviceVersions[len(serviceVersions)-1]
	}

	compatibleCompDefs := serviceVersionToCompDefs[serviceVersion]
	if len(compatibleCompDefs) == 0 {
		return compDef, serviceVersion, fmt.Errorf("no matched component definition found: %s", compDefName)
	}

	compatibleCompDefNames := maps.Keys(compatibleCompDefs)
	slices.Sort(compatibleCompDefNames)
	compatibleCompDefName := compatibleCompDefNames[len(compatibleCompDefNames)-1]

	return compatibleCompDefs[compatibleCompDefName], serviceVersion, nil
}

func buildServiceVersionToCompDefsMapping(ctx context.Context, cli client.Reader,
	compDefs []*appsv1alpha1.ComponentDefinition, serviceVersion string) (map[string]map[string]*appsv1alpha1.ComponentDefinition, error) {
	result := make(map[string]map[string]*appsv1alpha1.ComponentDefinition)

	insert := func(version string, compDef *appsv1alpha1.ComponentDefinition) {
		if _, ok := result[version]; !ok {
			result[version] = make(map[string]*appsv1alpha1.ComponentDefinition)
		}
		result[version][compDef.Name] = compDef
	}

	checkedInsert := func(version string, compDef *appsv1alpha1.ComponentDefinition) {
		if len(serviceVersion) == 0 {
			insert(version, compDef)
		} else if compareServiceVersion(serviceVersion, version) {
			insert(version, compDef)
		}
	}

	for _, compDef := range compDefs {
		compVersions, err := compatibleCompVersions(ctx, cli, compDef)
		if err != nil {
			return nil, err
		}

		serviceVersions := sets.New[string]()
		for _, compVersion := range compVersions {
			serviceVersions = serviceVersions.Union(compatibleServiceVersions(compDef, compVersion))
		}

		for version := range serviceVersions {
			checkedInsert(version, compDef)
		}
	}
	return result, nil
}

// compatibleCompVersions returns all component versions that are compatible with specified component definition.
func compatibleCompVersions(ctx context.Context, cli client.Reader, compDef *appsv1alpha1.ComponentDefinition) ([]*appsv1alpha1.ComponentVersion, error) {
	compVersionList := &appsv1alpha1.ComponentVersionList{}
	labels := client.MatchingLabels{
		compDef.Name: compDef.Name,
	}
	if err := cli.List(ctx, compVersionList, labels); err != nil {
		return nil, err
	}

	if len(compVersionList.Items) == 0 {
		return nil, nil
	}

	compVersions := make([]*appsv1alpha1.ComponentVersion, 0)
	for i, compVersion := range compVersionList.Items {
		if compVersion.Status.Phase != appsv1alpha1.AvailablePhase {
			return nil, fmt.Errorf("matched ComponentVersion %s is not available", compVersion.Name)
		}
		compVersions = append(compVersions, &compVersionList.Items[i])
	}
	return compVersions, nil
}

// compatibleServiceVersions returns service versions that are compatible with specified component definition.
func compatibleServiceVersions(compDef *appsv1alpha1.ComponentDefinition, compVersion *appsv1alpha1.ComponentVersion) sets.Set[string] {
	prefixMatch := func(prefix string) bool {
		return strings.HasPrefix(compDef.Name, prefix)
	}
	releases := make(map[string]bool, 0)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		if slices.IndexFunc(rule.CompDefs, prefixMatch) >= 0 {
			for _, release := range rule.Releases {
				releases[release] = true
			}
		}
	}
	serviceVersions := sets.New[string]()
	for _, release := range compVersion.Spec.Releases {
		if releases[release.Name] {
			serviceVersions = serviceVersions.Insert(release.ServiceVersion)
		}
	}
	return serviceVersions
}

// TODO
func compareServiceVersion(required, provide string) bool {
	ret, err := version.MustParseSemantic(required).Compare(provide)
	return err == nil && ret == 0
}

func resolveComponentVersion(ctx context.Context, cli client.Reader, compDef *appsv1alpha1.ComponentDefinition, serviceVersion string) error {
	compVersions, err := compatibleCompVersions(ctx, cli, compDef)
	if err != nil {
		return err
	}
	if len(compVersions) == 0 {
		return nil
	}
	return resolveImagesWithCompVersions(compDef, compVersions, serviceVersion)
}

func resolveImagesWithCompVersions(compDef *appsv1alpha1.ComponentDefinition,
	compVersions []*appsv1alpha1.ComponentVersion, serviceVersion string) error {
	appsInDef := covertImagesFromCompDefinition(compDef)
	appsByUser := findMatchedImagesFromCompVersions(compVersions, serviceVersion)

	apps := checkNMergeImages(serviceVersion, appsInDef, appsByUser)

	checkNUpdateImage := func(c *corev1.Container) error {
		var err error
		app, ok := apps[c.Name]
		switch {
		case ok && app.err == nil:
			c.Image = app.image
		case ok:
			err = app.err
		default:
			err = fmt.Errorf("no matched image found for container %s", c.Name)
		}
		return err
	}

	for i := range compDef.Spec.Runtime.InitContainers {
		if err := checkNUpdateImage(&compDef.Spec.Runtime.InitContainers[i]); err != nil {
			return err
		}
	}
	for i := range compDef.Spec.Runtime.Containers {
		if err := checkNUpdateImage(&compDef.Spec.Runtime.Containers[i]); err != nil {
			return err
		}
	}
	return nil
}

func covertImagesFromCompDefinition(compDef *appsv1alpha1.ComponentDefinition) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	checkNAdd := func(c *corev1.Container) {
		if len(c.Image) > 0 {
			apps[c.Name] = appNameVersionImage{
				name:    c.Name,
				version: compDef.Spec.ServiceVersion,
				image:   c.Image,
			}
		}
	}
	for i := range compDef.Spec.Runtime.InitContainers {
		checkNAdd(&compDef.Spec.Runtime.InitContainers[i])
	}
	for i := range compDef.Spec.Runtime.Containers {
		checkNAdd(&compDef.Spec.Runtime.Containers[i])
	}
	return apps
}

func findMatchedImagesFromCompVersions(compVersions []*appsv1alpha1.ComponentVersion, serviceVersion string) map[string]appNameVersionImage {
	appsWithReleases := make(map[string]map[string]appNameVersionImage)
	for _, compVersion := range compVersions {
		for _, release := range compVersion.Spec.Releases {
			if compareServiceVersion(serviceVersion, release.ServiceVersion) {
				for name, image := range release.Images {
					if _, ok := appsWithReleases[name]; !ok {
						appsWithReleases[name] = make(map[string]appNameVersionImage)
					}
					appsWithReleases[name][release.Name] = appNameVersionImage{
						name:    name,
						version: release.ServiceVersion,
						image:   image,
					}
				}
			}
		}
	}
	apps := make(map[string]appNameVersionImage)
	for name, releases := range appsWithReleases {
		names := maps.Keys(releases)
		slices.Sort(names)
		// use the latest release
		apps[name] = releases[names[len(names)-1]]
	}
	return apps
}

func checkNMergeImages(serviceVersion string, appsInDef, appsByUser map[string]appNameVersionImage) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	merge := func(name string, def, user appNameVersionImage) appNameVersionImage {
		if len(user.name) == 0 {
			if !compareServiceVersion(serviceVersion, def.version) {
				def.err = fmt.Errorf("no matched image found for container %s with required version %s", name, serviceVersion)
			}
			return def
		}
		return user
	}
	for _, name := range append(maps.Keys(appsInDef), maps.Keys(appsByUser)...) {
		apps[name] = merge(name, appsInDef[name], appsByUser[name])
	}
	return apps
}

type appNameVersionImage struct {
	name    string
	version string
	image   string
	err     error
}

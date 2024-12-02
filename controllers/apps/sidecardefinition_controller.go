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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsconfig "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// SidecarDefinitionReconciler reconciles a SidecarDefinition object
type SidecarDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=sidecardefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=sidecardefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=sidecardefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *SidecarDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("sidecarDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	sidecarDef := &appsv1.SidecarDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, sidecarDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, sidecarDef,
		sidecarDefinitionFinalizerName, r.deletionHandler(reqCtx, sidecarDef)); res != nil {
		return *res, err
	}

	// if sidecarDef.Status.ObservedGeneration == sidecarDef.Generation &&
	//	sidecarDef.Status.Phase == appsv1.AvailablePhase {
	//	return intctrlutil.Reconciled()
	// }

	sidecarDefCopy := sidecarDef.DeepCopy()
	if err := r.validateNResolve(r.Client, reqCtx, sidecarDef); err != nil {
		if err1 := r.unavailable(reqCtx, sidecarDefCopy, sidecarDef, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, reqCtx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.immutableHash(r.Client, reqCtx, sidecarDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.available(reqCtx, sidecarDefCopy, sidecarDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, sidecarDef)

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *SidecarDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.SidecarDefinition{}).
		Watches(&appsv1.ComponentDefinition{}, handler.EnqueueRequestsFromMapFunc(r.matchedCompDefinition)).
		Complete(r)
}

func (r *SidecarDefinitionReconciler) matchedCompDefinition(ctx context.Context, obj client.Object) []reconcile.Request {
	compDef, ok := obj.(*appsv1.ComponentDefinition)
	if !ok {
		return nil
	}
	sidecarDefs := &appsv1.SidecarDefinitionList{}
	if err := r.Client.List(ctx, sidecarDefs); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for _, sidecarDef := range sidecarDefs.Items {
		names := append([]string{sidecarDef.Spec.Owner}, sidecarDef.Spec.Selectors...)
		if slices.ContainsFunc(names, func(name string) bool {
			return component.DefNameMatched(compDef.Name, name)
		}) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: sidecarDef.Name,
				},
			})
		}
	}
	return requests
}

func (r *SidecarDefinitionReconciler) deletionHandler(rctx intctrlutil.RequestCtx, sidecarDef *appsv1.SidecarDefinition) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(sidecarDef, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing Cluster")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, sidecarDef, constant.SidecarDefLabelKey,
			recordEvent, &appsv1.ComponentList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *SidecarDefinitionReconciler) available(rctx intctrlutil.RequestCtx,
	sidecarDefCopy, sidecarDef *appsv1.SidecarDefinition) error {
	return r.status(rctx, sidecarDefCopy, sidecarDef, appsv1.AvailablePhase, "")
}

func (r *SidecarDefinitionReconciler) unavailable(rctx intctrlutil.RequestCtx,
	sidecarDefCopy, sidecarDef *appsv1.SidecarDefinition, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return r.status(rctx, sidecarDefCopy, sidecarDef, appsv1.UnavailablePhase, message)
}

func (r *SidecarDefinitionReconciler) status(rctx intctrlutil.RequestCtx,
	sidecarDefCopy, sidecarDef *appsv1.SidecarDefinition, phase appsv1.Phase, message string) error {
	patch := client.MergeFrom(sidecarDefCopy)
	sidecarDef.Status.ObservedGeneration = sidecarDef.Generation
	sidecarDef.Status.Phase = phase
	sidecarDef.Status.Message = message
	return r.Client.Status().Patch(rctx.Ctx, sidecarDef, patch)
}

func (r *SidecarDefinitionReconciler) validateNResolve(cli client.Client, rctx intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition) error {
	compDefList := &appsv1.ComponentDefinitionList{}
	if err := cli.List(rctx.Ctx, compDefList); err != nil {
		return err
	}

	for _, f := range []func(client.Client, intctrlutil.RequestCtx, *appsv1.SidecarDefinition, []appsv1.ComponentDefinition) error{
		r.validateContainer,
		r.validateVars,
		r.validateConfigNScript,
		r.validateNResolveOwner,
		r.validateNResolveSelectors,
		r.validateOwnerNSelectors,
	} {
		if err := f(cli, rctx, sidecarDef, compDefList.Items); err != nil {
			return err
		}
	}
	return r.immutableCheck(sidecarDef)
}

func (r *SidecarDefinitionReconciler) validateContainer(_ client.Client, _ intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition, _ []appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(sidecarDef.Spec.Containers, "Name", nil) {
		return fmt.Errorf("duplicate names of containers are not allowed")
	}
	return nil
}

func (r *SidecarDefinitionReconciler) validateVars(_ client.Client, _ intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition, _ []appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(sidecarDef.Spec.Vars, "Name", nil) {
		return fmt.Errorf("duplicate names of vars are not allowed")
	}
	return nil
}

func (r *SidecarDefinitionReconciler) validateConfigNScript(cli client.Client, rctx intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition, _ []appsv1.ComponentDefinition) error {
	templates := sidecarDef.Spec.Configs
	if len(sidecarDef.Spec.Scripts) > 0 {
		if templates == nil {
			templates = sidecarDef.Spec.Scripts
		} else {
			templates = append(templates, sidecarDef.Spec.Scripts...)
		}
	}
	if !checkUniqueItemWithValue(templates, "Name", nil) {
		return fmt.Errorf("duplicate names of configs/scripts are not allowed")
	}

	configs := func() []appsv1.ComponentConfigSpec {
		if len(sidecarDef.Spec.Configs) == 0 {
			return nil
		}
		configs := make([]appsv1.ComponentConfigSpec, 0)
		for i := range sidecarDef.Spec.Configs {
			configs = append(configs, appsv1.ComponentConfigSpec{
				ComponentTemplateSpec: sidecarDef.Spec.Configs[i],
			})
		}
		return configs
	}
	compDef := &appsv1.ComponentDefinition{
		Spec: appsv1.ComponentDefinitionSpec{
			Configs: configs(),
			Scripts: sidecarDef.Spec.Scripts,
		},
	}
	return appsconfig.ReconcileConfigSpecsForReferencedCR(cli, rctx, compDef)
}

func (r *SidecarDefinitionReconciler) validateNResolveOwner(_ client.Client, _ intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition, compDefs []appsv1.ComponentDefinition) error {
	owner := sidecarDef.Spec.Owner
	if len(owner) == 0 {
		return fmt.Errorf("owner is required")
	}
	if err := component.ValidateDefNameRegexp(owner); err != nil {
		return err
	}

	matched := make([]string, 0)
	for _, compDef := range compDefs {
		if component.DefNameMatched(compDef.Name, owner) {
			matched = append(matched, compDef.Name)
		}
	}
	// a valid owner is required
	if len(matched) == 0 {
		return fmt.Errorf("no matched owner found: %s", owner)
	}

	// set the matched owners
	slices.Sort(matched)
	sidecarDef.Status.Owners = strings.Join(matched, ",")

	return nil
}

func (r *SidecarDefinitionReconciler) validateNResolveSelectors(_ client.Client, _ intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition, compDefs []appsv1.ComponentDefinition) error {
	selectors := sidecarDef.Spec.Selectors
	for _, selector := range selectors {
		if err := component.ValidateDefNameRegexp(selector); err != nil {
			return err
		}
	}
	matched := make([]string, 0)
	for _, compDef := range compDefs {
		if slices.ContainsFunc(selectors, func(selector string) bool {
			return component.DefNameMatched(compDef.Name, selector)
		}) {
			if err := r.validateMatchedCompDef(sidecarDef, &compDef); err != nil {
				return err
			}
			matched = append(matched, compDef.Name)
		}
	}

	// set the matched selectors
	slices.Sort(matched)
	sidecarDef.Status.Selectors = strings.Join(matched, ",")

	return nil
}

func (r *SidecarDefinitionReconciler) validateMatchedCompDef(sidecarDef *appsv1.SidecarDefinition,
	compDef *appsv1.ComponentDefinition) error {
	vars := func() error {
		if len(sidecarDef.Spec.Vars) == 0 || len(compDef.Spec.Vars) == 0 {
			return nil
		}
		names := sets.New[string]()
		for _, v := range compDef.Spec.Vars {
			names.Insert(v.Name)
		}
		for _, v := range sidecarDef.Spec.Vars {
			if names.Has(v.Name) {
				return fmt.Errorf("vars %s is conflicted with the component definition %s", v.Name, compDef.Name)
			}
		}
		return nil
	}

	templates := func() error {
		validate := func(key string, sidecar, comp []appsv1.ComponentTemplateSpec) error {
			if len(sidecar) == 0 || len(comp) == 0 {
				return nil
			}
			names := sets.New[string]()
			for _, v := range comp {
				names.Insert(v.Name)
			}
			for _, v := range sidecar {
				if names.Has(v.Name) {
					return fmt.Errorf("%s template %s is conflicted with the component definition %s", key, v.Name, compDef.Name)
				}
			}
			return nil
		}

		templateOfConfig := func(configs []appsv1.ComponentConfigSpec) []appsv1.ComponentTemplateSpec {
			if len(configs) == 0 {
				return nil
			}
			l := make([]appsv1.ComponentTemplateSpec, 0)
			for i := range configs {
				l = append(l, configs[i].ComponentTemplateSpec)
			}
			return l
		}

		if err := validate("config", sidecarDef.Spec.Configs, templateOfConfig(compDef.Spec.Configs)); err != nil {
			return err
		}
		return validate("script", sidecarDef.Spec.Scripts, compDef.Spec.Scripts)
	}
	for _, f := range []func() error{vars, templates} {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

func (r *SidecarDefinitionReconciler) validateOwnerNSelectors(_ client.Client, _ intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition, _ []appsv1.ComponentDefinition) error {
	status := sidecarDef.Status
	if len(status.Owners) == 0 || len(status.Selectors) == 0 {
		return nil
	}

	owners := sets.New(strings.Split(status.Owners, ",")...)
	selectors := sets.New(strings.Split(status.Selectors, ",")...)
	intersected := owners.Intersection(selectors)
	if intersected.Len() > 0 {
		return fmt.Errorf("owner and selectors should not be overlapped: %s", strings.Join(sets.List(intersected), ","))
	}
	return nil
}

func (r *SidecarDefinitionReconciler) immutableCheck(sidecarDef *appsv1.SidecarDefinition) error {
	if r.skipImmutableCheck(sidecarDef) {
		return nil
	}

	newHashValue, err := r.specHash(sidecarDef)
	if err != nil {
		return err
	}

	hashValue, ok := sidecarDef.Annotations[immutableHashAnnotationKey]
	if ok && hashValue != newHashValue {
		// TODO: fields been updated
		return fmt.Errorf("immutable fields can't be updated")
	}
	return nil
}

func (r *SidecarDefinitionReconciler) skipImmutableCheck(sidecarDef *appsv1.SidecarDefinition) bool {
	if sidecarDef.Annotations == nil {
		return false
	}
	skip, ok := sidecarDef.Annotations[constant.SkipImmutableCheckAnnotationKey]
	return ok && strings.ToLower(skip) == "true"
}

func (r *SidecarDefinitionReconciler) specHash(sidecarDef *appsv1.SidecarDefinition) (string, error) {
	data, err := json.Marshal(sidecarDef.Spec)
	if err != nil {
		return "", err
	}
	hash := fnv.New32a()
	hash.Write(data)
	return rand.SafeEncodeString(fmt.Sprintf("%d", hash.Sum32())), nil
}

func (r *SidecarDefinitionReconciler) immutableHash(cli client.Client, rctx intctrlutil.RequestCtx,
	sidecarDef *appsv1.SidecarDefinition) error {
	if r.skipImmutableCheck(sidecarDef) {
		return nil
	}

	if sidecarDef.Annotations != nil {
		_, ok := sidecarDef.Annotations[immutableHashAnnotationKey]
		if ok {
			return nil
		}
	}

	patch := client.MergeFrom(sidecarDef.DeepCopy())
	if sidecarDef.Annotations == nil {
		sidecarDef.Annotations = map[string]string{}
	}
	sidecarDef.Annotations[immutableHashAnnotationKey], _ = r.specHash(sidecarDef)
	return cli.Patch(rctx.Ctx, sidecarDef, patch)
}

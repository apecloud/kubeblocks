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
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"slices"
	"strings"

	"github.com/pkg/errors"
	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	componentDefinitionFinalizerName = "componentdefinition.kubeblocks.io/finalizer"
	immutableHashAnnotationKey       = "apps.kubeblocks.io/immutable-hash"
)

// ComponentDefinitionReconciler reconciles a ComponentDefinition object
type ComponentDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("component", req.NamespacedName),
		Recorder: r.Recorder,
	}

	rctx.Log.V(1).Info("reconcile", "component", req.NamespacedName)

	cmpd := &appsv1.ComponentDefinition{}
	if err := r.Client.Get(rctx.Ctx, rctx.Req.NamespacedName, cmpd); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	if !intctrlutil.ObjectAPIVersionSupported(cmpd) {
		return intctrlutil.Reconciled()
	}

	return r.reconcile(rctx, cmpd)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.ComponentDefinition{}).
		Complete(r)
}

func (r *ComponentDefinitionReconciler) reconcile(rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, cmpd, componentDefinitionFinalizerName, r.deletionHandler(rctx, cmpd))
	if res != nil {
		return *res, err
	}

	if cmpd.Status.ObservedGeneration == cmpd.Generation &&
		slices.Contains([]appsv1.Phase{appsv1.AvailablePhase}, cmpd.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err = r.validate(r.Client, rctx, cmpd); err != nil {
		if err1 := r.unavailable(r.Client, rctx, cmpd, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	if err = r.immutableHash(r.Client, rctx, cmpd); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	if err = r.available(r.Client, rctx, cmpd); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, cmpd)

	return intctrlutil.Reconciled()
}

func (r *ComponentDefinitionReconciler) deletionHandler(rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(cmpd, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Component.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, cmpd, constant.ComponentDefinitionLabelKey,
			recordEvent, &appsv1.ComponentList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ComponentDefinitionReconciler) available(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	return r.status(cli, rctx, cmpd, appsv1.AvailablePhase, "")
}

func (r *ComponentDefinitionReconciler) unavailable(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition, err error) error {
	return r.status(cli, rctx, cmpd, appsv1.UnavailablePhase, err.Error())
}

func (r *ComponentDefinitionReconciler) status(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition, phase appsv1.Phase, message string) error {
	patch := client.MergeFrom(cmpd.DeepCopy())
	cmpd.Status.ObservedGeneration = cmpd.Generation
	cmpd.Status.Phase = phase
	cmpd.Status.Message = message
	return cli.Status().Patch(rctx.Ctx, cmpd, patch)
}

func (r *ComponentDefinitionReconciler) immutableHash(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if r.skipImmutableCheck(cmpd) {
		return nil
	}

	if cmpd.Annotations != nil {
		_, ok := cmpd.Annotations[immutableHashAnnotationKey]
		if ok {
			return nil
		}
	}

	patch := client.MergeFrom(cmpd.DeepCopy())
	if cmpd.Annotations == nil {
		cmpd.Annotations = map[string]string{}
	}
	cmpd.Annotations[immutableHashAnnotationKey], _ = r.cmpdHash(cmpd)
	return cli.Patch(rctx.Ctx, cmpd, patch)
}

func (r *ComponentDefinitionReconciler) validate(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	for _, validator := range []func(client.Client, intctrlutil.RequestCtx, *appsv1.ComponentDefinition) error{
		r.validateServiceVersion,
		r.validateLabels,
		r.validateRuntime,
		r.validateVars,
		r.validateVolumes,
		r.validateHostNetwork,
		r.validateServices,
		r.validateFileTemplate,
		r.validateSystemAccounts,
		r.validateReplicasLimit,
		r.validateAvailable,
		r.validateReplicaRoles,
		r.validatePodManagementPolicy,
		r.validatePolicyRules,
		r.validateLifecycleActions,
	} {
		if err := validator(cli, rctx, cmpd); err != nil {
			return err
		}
	}
	return r.immutableCheck(cmpd)
}

func (r *ComponentDefinitionReconciler) validateServiceVersion(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	return validateServiceVersion(cmpd.Spec.ServiceVersion)
}

func (r *ComponentDefinitionReconciler) validateLabels(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateRuntime(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateVars(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.Vars, "Name", nil) {
		return fmt.Errorf("duplicate names of component vars are not allowed")
	}

	// validate the reference to component definition name pattern
	var compDef string
	for _, cVar := range cmpd.Spec.Vars {
		if len(cVar.Value) > 0 && cVar.ValueFrom != nil {
			return fmt.Errorf("both value and valueFrom are specified for component var: %s", cVar.Name)
		}
		if cVar.ValueFrom == nil {
			continue
		}
		switch {
		case cVar.ValueFrom.HostNetworkVarRef != nil:
			compDef = cVar.ValueFrom.HostNetworkVarRef.CompDef
		case cVar.ValueFrom.ServiceVarRef != nil:
			compDef = cVar.ValueFrom.ServiceVarRef.CompDef
		case cVar.ValueFrom.ServiceRefVarRef != nil:
			compDef = cVar.ValueFrom.ServiceRefVarRef.CompDef
		case cVar.ValueFrom.ResourceVarRef != nil:
			compDef = cVar.ValueFrom.ResourceVarRef.CompDef
		case cVar.ValueFrom.ComponentVarRef != nil:
			compDef = cVar.ValueFrom.ComponentVarRef.CompDef
		case cVar.ValueFrom.CredentialVarRef != nil:
			compDef = cVar.ValueFrom.CredentialVarRef.CompDef
		default:
			continue
		}

		if len(compDef) == 0 {
			continue
		}
		if err := component.ValidateDefNameRegexp(compDef); err != nil {
			return errors.Wrapf(err, "invalid reference to component definition name pattern: %s", compDef)
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateVolumes(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.Volumes, "Name", nil) {
		return fmt.Errorf("duplicate volume names are not allowed")
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateHostNetwork(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if cmpd.Spec.HostNetwork == nil {
		return nil
	}
	if !checkUniqueItemWithValue(cmpd.Spec.HostNetwork.ContainerPorts, "Container", nil) {
		return fmt.Errorf("duplicate container of host-network are not allowed")
	}

	containerPorts := make(map[string]map[string]bool)
	for _, cc := range [][]corev1.Container{cmpd.Spec.Runtime.InitContainers, cmpd.Spec.Runtime.Containers} {
		for _, c := range cc {
			ports := make(map[string]bool)
			for _, p := range c.Ports {
				ports[p.Name] = true
			}
			containerPorts[c.Name] = ports
		}
	}

	for _, c := range cmpd.Spec.HostNetwork.ContainerPorts {
		ports, ok := containerPorts[c.Container]
		if !ok {
			return fmt.Errorf("the container that host-network referenced is not defined: %s", c.Container)
		}
		for _, p := range c.Ports {
			if _, ok = ports[p]; !ok {
				return fmt.Errorf("the container port that host-network referenced is not defined: %s.%s", c.Container, p)
			}
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateServices(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.Services, "Name", nil) {
		return fmt.Errorf("duplicate names of component service are not allowed")
	}

	if !checkUniqueItemWithValue(cmpd.Spec.Services, "ServiceName", nil) {
		return fmt.Errorf("duplicate service names are not allowed")
	}

	for _, svc := range cmpd.Spec.Services {
		if len(svc.Spec.Ports) == 0 {
			return fmt.Errorf("there is no port defined for service: %s", svc.Name)
		}
	}

	roleNames := make(map[string]bool, 0)
	for _, role := range cmpd.Spec.Roles {
		roleNames[strings.ToLower(role.Name)] = true
	}
	for _, svc := range cmpd.Spec.Services {
		if len(svc.RoleSelector) > 0 && !roleNames[strings.ToLower(svc.RoleSelector)] {
			return fmt.Errorf("the role that service selector used is not defined: %s", svc.RoleSelector)
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateFileTemplate(cli client.Client, rctx intctrlutil.RequestCtx,
	compDef *appsv1.ComponentDefinition) error {
	return validateComponentFileTemplate(cli, rctx, compDef)
}

func validateComponentFileTemplate(cli client.Client, rctx intctrlutil.RequestCtx, cmpd *appsv1.ComponentDefinition) error {
	validateObject := func(objectKey client.ObjectKey) error {
		configObj := &corev1.ConfigMap{}
		return cli.Get(rctx.Ctx, objectKey, configObj)
	}
	validateTemplate := func(tpl appsv1.ComponentFileTemplate) error {
		if tpl.Template != "" {
			return validateObject(client.ObjectKey{Namespace: tpl.Namespace, Name: tpl.Template})
		}
		return nil
	}
	for _, tpls := range [][]appsv1.ComponentFileTemplate{cmpd.Spec.Configs, cmpd.Spec.Scripts} {
		for _, tpl := range tpls {
			if err := validateTemplate(tpl); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateSystemAccounts(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.SystemAccounts, "Name", nil) {
		return fmt.Errorf("duplicate system accounts are not allowed")
	}

	hasNonInitAccount := false
	for _, account := range cmpd.Spec.SystemAccounts {
		if account.InitAccount {
			continue
		}
		hasNonInitAccount = true
		if account.Statement == nil || len(account.Statement.Create) == 0 {
			return fmt.Errorf("the create statement must be provided to provision system account: %s", account.Name)
		}
	}
	if hasNonInitAccount && (cmpd.Spec.LifecycleActions == nil || cmpd.Spec.LifecycleActions.AccountProvision == nil) {
		return fmt.Errorf("the AccountProvision action is needed to provision system accounts")
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateReplicasLimit(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateAvailable(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if cmpd.Spec.Available == nil {
		return nil
	}
	if cmpd.Spec.Available.WithProbe != nil {
		return r.validateAvailableWithProbe(cmpd)
	}
	if cmpd.Spec.Available.WithPhases != nil {
		if err := r.validateAvailableWithPhases(cmpd); err != nil {
			return err
		}
	}
	if cmpd.Spec.Available.WithRole != nil {
		if err := r.validateAvailableWithRole(cmpd); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateAvailableWithPhases(cmpd *appsv1.ComponentDefinition) error {
	phases := sets.New[string](strings.Split(strings.ToLower(*cmpd.Spec.Available.WithPhases), ",")...)
	supported := sets.New[string](
		strings.ToLower(string(appsv1.CreatingComponentPhase)),
		strings.ToLower(string(appsv1.RunningComponentPhase)),
		strings.ToLower(string(appsv1.UpdatingComponentPhase)),
		strings.ToLower(string(appsv1.StartingComponentPhase)),
		strings.ToLower(string(appsv1.StoppingComponentPhase)),
		strings.ToLower(string(appsv1.StoppedComponentPhase)),
		strings.ToLower(string(appsv1.DeletingComponentPhase)),
		strings.ToLower(string(appsv1.FailedComponentPhase)),
	)
	result := phases.Difference(supported)
	if result.Len() > 0 {
		return fmt.Errorf("unsupported phases are specified: %s", strings.Join(sets.List(result), ","))
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateAvailableWithRole(cmpd *appsv1.ComponentDefinition) error {
	role := strings.ToLower(*cmpd.Spec.Available.WithRole)
	for _, r := range cmpd.Spec.Roles {
		if strings.ToLower(r.Name) == role {
			return nil
		}
	}
	return fmt.Errorf("the role that available with role used is not defined: %s", role)
}

func (r *ComponentDefinitionReconciler) validateAvailableWithProbe(cmpd *appsv1.ComponentDefinition) error {
	withProbe := cmpd.Spec.Available.WithProbe
	if withProbe == nil {
		return nil
	}
	if cmpd.Spec.LifecycleActions == nil || cmpd.Spec.LifecycleActions.AvailableProbe == nil {
		return fmt.Errorf("the available probe is required to be defined when withProbe of available is specified")
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateReplicaRoles(cli client.Client, reqCtx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.Roles, "Name", nil) {
		return fmt.Errorf("duplicate replica roles are not allowed")
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validatePodManagementPolicy(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	policy := cmpd.Spec.PodManagementPolicy
	if policy != nil {
		policies := []k8sappsv1.PodManagementPolicyType{k8sappsv1.OrderedReadyPodManagement, k8sappsv1.ParallelPodManagement}
		if !slices.Contains(policies, *policy) {
			return fmt.Errorf("invalid podManagementPolicy: %s", *policy)
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validatePolicyRules(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	// TODO: how to check the acquired rules can be granted?
	return nil
}

func (r *ComponentDefinitionReconciler) validateLifecycleActions(cli client.Client, reqCtx intctrlutil.RequestCtx,
	cmpd *appsv1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) immutableCheck(cmpd *appsv1.ComponentDefinition) error {
	if r.skipImmutableCheck(cmpd) {
		return nil
	}

	newHashValue, err := r.cmpdHash(cmpd)
	if err != nil {
		return err
	}

	hashValue, ok := cmpd.Annotations[immutableHashAnnotationKey]
	if ok && hashValue != newHashValue {
		// TODO: fields been updated
		return fmt.Errorf("immutable fields can't be updated")
	}
	return nil
}

func (r *ComponentDefinitionReconciler) skipImmutableCheck(cmpd *appsv1.ComponentDefinition) bool {
	if cmpd.Annotations == nil {
		return false
	}
	skip, ok := cmpd.Annotations[constant.SkipImmutableCheckAnnotationKey]
	return ok && strings.ToLower(skip) == "true"
}

func (r *ComponentDefinitionReconciler) cmpdHash(cmpd *appsv1.ComponentDefinition) (string, error) {
	objCopy := cmpd.DeepCopy()

	// reset all mutable fields
	objCopy.Spec.Provider = ""
	objCopy.Spec.Description = ""
	objCopy.Spec.Exporter = nil
	objCopy.Spec.PodManagementPolicy = nil
	objCopy.Spec.MinReadySeconds = 0

	// TODO: bpt

	data, err := json.Marshal(objCopy.Spec)
	if err != nil {
		return "", err
	}
	hash := fnv.New32a()
	hash.Write(data)
	return rand.SafeEncodeString(fmt.Sprintf("%d", hash.Sum32())), nil
}

// listCompDefinitionsWithPattern returns all component definitions whose names match the given pattern
func listCompDefinitionsWithPattern(ctx context.Context, cli client.Reader, name string) ([]*appsv1.ComponentDefinition, error) {
	compDefList := &appsv1.ComponentDefinitionList{}
	if err := cli.List(ctx, compDefList); err != nil {
		return nil, err
	}
	compDefsFullyMatched := make([]*appsv1.ComponentDefinition, 0)
	compDefsPatternMatched := make([]*appsv1.ComponentDefinition, 0)
	for i, item := range compDefList.Items {
		if item.Name == name {
			compDefsFullyMatched = append(compDefsFullyMatched, &compDefList.Items[i])
		}
		if component.PrefixOrRegexMatched(item.Name, name) {
			compDefsPatternMatched = append(compDefsPatternMatched, &compDefList.Items[i])
		}
	}
	if len(compDefsFullyMatched) > 0 {
		return compDefsFullyMatched, nil
	}
	return compDefsPatternMatched, nil
}

func checkUniqueItemWithValue(slice any, fieldName string, val any) bool {
	sliceValue := reflect.ValueOf(slice)
	if sliceValue.Kind() != reflect.Slice {
		panic("Not a slice")
	}

	lookupTable := make(map[any]bool)
	for i := 0; i < sliceValue.Len(); i++ {
		item := sliceValue.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		if item.Kind() != reflect.Struct {
			panic("Items in the slice are not structs or pointers to structs")
		}

		field := item.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName)
		})
		if !field.IsValid() {
			panic(fmt.Sprintf("Field '%s' not found in struct", fieldName))
		}
		fieldValue := field.Interface()

		if lookupTable[fieldValue] {
			if val == nil || val == fieldValue {
				return false
			}
		}
		lookupTable[fieldValue] = true
	}
	return true
}

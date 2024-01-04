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
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ComponentDefinitionReconciler reconciles a ComponentDefinition object
type ComponentDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions/finalizers,verbs=update

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

	cmpd := &appsv1alpha1.ComponentDefinition{}
	if err := r.Client.Get(rctx.Ctx, rctx.Req.NamespacedName, cmpd); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	return r.reconcile(rctx, cmpd)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ComponentDefinition{}).
		Complete(r)
}

func (r *ComponentDefinitionReconciler) reconcile(rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, cmpd, componentDefinitionFinalizerName, r.deletionHandler(rctx, cmpd))
	if res != nil {
		return *res, err
	}

	if cmpd.Status.ObservedGeneration == cmpd.Generation &&
		slices.Contains([]appsv1alpha1.Phase{appsv1alpha1.AvailablePhase}, cmpd.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err = r.validate(r.Client, rctx, cmpd); err != nil {
		if err1 := r.unavailable(r.Client, rctx, cmpd, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	err = r.available(r.Client, rctx, cmpd)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, cmpd)

	return intctrlutil.Reconciled()
}

func (r *ComponentDefinitionReconciler) deletionHandler(rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(cmpd, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, cmpd, constant.ComponentDefinitionLabelKey,
			recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ComponentDefinitionReconciler) available(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return r.status(cli, rctx, cmpd, appsv1alpha1.AvailablePhase, "")
}

func (r *ComponentDefinitionReconciler) unavailable(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition, err error) error {
	return r.status(cli, rctx, cmpd, appsv1alpha1.UnavailablePhase, err.Error())
}

func (r *ComponentDefinitionReconciler) status(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition, phase appsv1alpha1.Phase, message string) error {
	patch := client.MergeFrom(cmpd.DeepCopy())
	cmpd.Status.ObservedGeneration = cmpd.Generation
	cmpd.Status.Phase = phase
	cmpd.Status.Message = message
	return cli.Status().Patch(rctx.Ctx, cmpd, patch)
}

func (r *ComponentDefinitionReconciler) validate(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	for _, validator := range []func(client.Client, intctrlutil.RequestCtx, *appsv1alpha1.ComponentDefinition) error{
		r.validateRuntime,
		r.validateVars,
		r.validateVolumes,
		r.validateServices,
		r.validateConfigs,
		r.validateScripts,
		r.validatePolicyRules,
		r.validateLabels,
		r.validateReplicasLimit,
		r.validateSystemAccounts,
		r.validateReplicaRoles,
		r.validateLifecycleActions,
		r.validateComponentDefRef,
	} {
		if err := validator(cli, rctx, cmpd); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateRuntime(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateVars(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateVolumes(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.Volumes, "Name", nil) {
		return fmt.Errorf("duplicate volume names are not allowed")
	}

	hasVolumeToProtect := false
	for _, vol := range cmpd.Spec.Volumes {
		if vol.HighWatermark > 0 && vol.HighWatermark < 100 {
			hasVolumeToProtect = true
			break
		}
	}
	if hasVolumeToProtect {
		if cmpd.Spec.LifecycleActions == nil || cmpd.Spec.LifecycleActions.Readonly == nil || cmpd.Spec.LifecycleActions.Readwrite == nil {
			return fmt.Errorf("the Readonly and Readwrite actions are needed to protect volumes")
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateServices(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
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

func (r *ComponentDefinitionReconciler) validateConfigs(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	// if err := appsconfig.ReconcileConfigSpecsForReferencedCR(r.Client, rctx, dbClusterDef); err != nil {
	//	return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, err.Error())
	// }
	return nil
}

func (r *ComponentDefinitionReconciler) validateScripts(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validatePolicyRules(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	// TODO: how to check the acquired rules can be granted?
	return nil
}

func (r *ComponentDefinitionReconciler) validateLabels(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateReplicasLimit(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateSystemAccounts(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	for _, v := range cmpd.Spec.SystemAccounts {
		if v.SecretRef == nil && (cmpd.Spec.LifecycleActions == nil || cmpd.Spec.LifecycleActions.AccountProvision == nil) {
			return fmt.Errorf(`the AccountProvision action is needed to provision system account %s`, v.Name)
		}
	}
	if !checkUniqueItemWithValue(cmpd.Spec.SystemAccounts, "Name", nil) {
		return fmt.Errorf("duplicate system accounts are not allowed")
	}
	if !checkUniqueItemWithValue(cmpd.Spec.SystemAccounts, "InitAccount", true) {
		return fmt.Errorf("multiple system init accounts are not allowed")
	}

	for _, account := range cmpd.Spec.SystemAccounts {
		if !account.InitAccount && len(account.Statement) == 0 && account.SecretRef == nil {
			return fmt.Errorf("the Statement or SecretRef must be provided to create system account: %s", account.Name)
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateReplicaRoles(cli client.Client, reqCtx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	if !checkUniqueItemWithValue(cmpd.Spec.Roles, "Name", nil) {
		return fmt.Errorf("duplicate replica roles are not allowed")
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateLifecycleActions(cli client.Client, reqCtx intctrlutil.RequestCtx, cmpd *appsv1alpha1.ComponentDefinition) error {
	if err := r.validateLifecycleActionBuiltInHandlers(cmpd.Spec.LifecycleActions); err != nil {
		return err
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateLifecycleActionBuiltInHandlers(lifecycleActions *appsv1alpha1.ComponentLifecycleActions) error {
	if lifecycleActions == nil {
		return nil
	}

	builtInHandlerMap := make(map[appsv1alpha1.BuiltinActionHandlerType]bool)
	supportedBuiltInHandlers := getBuiltinActionHandlers()

	if lifecycleActions.RoleProbe != nil && lifecycleActions.RoleProbe.BuiltinHandler != nil {
		if !slices.Contains(supportedBuiltInHandlers, *lifecycleActions.RoleProbe.BuiltinHandler) {
			return fmt.Errorf("the builtin handler %s is not supported", *lifecycleActions.RoleProbe.BuiltinHandler)
		}
		builtInHandlerMap[*lifecycleActions.RoleProbe.BuiltinHandler] = true
	}

	actions := []struct {
		LifeCycleActionHandlers *appsv1alpha1.LifecycleActionHandler
	}{
		{lifecycleActions.PostProvision},
		{lifecycleActions.PreTerminate},
		{lifecycleActions.MemberJoin},
		{lifecycleActions.MemberLeave},
		{lifecycleActions.Readonly},
		{lifecycleActions.Readwrite},
		{lifecycleActions.DataPopulate},
		{lifecycleActions.DataAssemble},
		{lifecycleActions.Reconfigure},
		{lifecycleActions.AccountProvision},
	}

	for _, action := range actions {
		if action.LifeCycleActionHandlers != nil && action.LifeCycleActionHandlers.BuiltinHandler != nil {
			if !slices.Contains(supportedBuiltInHandlers, *lifecycleActions.RoleProbe.BuiltinHandler) {
				return fmt.Errorf("the builtin handler %s is not supported", *lifecycleActions.RoleProbe.BuiltinHandler)
			}
			builtInHandlerMap[*lifecycleActions.RoleProbe.BuiltinHandler] = true
		}
	}

	if len(builtInHandlerMap) > 1 {
		return fmt.Errorf("the builtin handler within the same lifecycle actions should be consistent")
	}

	return nil
}

func (r *ComponentDefinitionReconciler) validateComponentDefRef(cli client.Client, reqCtx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
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

func getBuiltinActionHandlers() []appsv1alpha1.BuiltinActionHandlerType {
	return []appsv1alpha1.BuiltinActionHandlerType{
		appsv1alpha1.MySQLBuiltinActionHandler,
		appsv1alpha1.WeSQLBuiltinActionHandler,
		appsv1alpha1.OceanbaseBuiltinActionHandler,
		appsv1alpha1.RedisBuiltinActionHandler,
		appsv1alpha1.MongoDBBuiltinActionHandler,
		appsv1alpha1.ETCDBuiltinActionHandler,
		appsv1alpha1.PostgresqlBuiltinActionHandler,
		appsv1alpha1.OfficialPostgresqlBuiltinActionHandler,
		appsv1alpha1.ApeCloudPostgresqlBuiltinActionHandler,
		appsv1alpha1.PolarDBXBuiltinActionHandler,
	}
}

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

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=shardingdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=shardingdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=shardingdefinitions/finalizers,verbs=update

// ShardingDefinitionReconciler reconciles a ShardingDefinition object
type ShardingDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	compDefs []*appsv1.ComponentDefinition
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ShardingDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("shardingDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	shardingDef := &appsv1.ShardingDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, shardingDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, shardingDef,
		shardingDefinitionFinalizerName, r.deletionHandler(reqCtx, shardingDef)); res != nil {
		return *res, err
	}

	if shardingDef.Status.ObservedGeneration == shardingDef.Generation &&
		shardingDef.Status.Phase == appsv1.AvailablePhase {
		return intctrlutil.Reconciled()
	}

	if err := r.validate(r.Client, reqCtx, shardingDef); err != nil {
		fmt.Printf("error: %v\n", err)
		if err1 := r.unavailable(reqCtx, shardingDef, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, reqCtx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.immutableHash(r.Client, reqCtx, shardingDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.available(reqCtx, shardingDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, shardingDef)

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ShardingDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&appsv1.ShardingDefinition{}).
		Complete(r)
}

func (r *ShardingDefinitionReconciler) deletionHandler(rctx intctrlutil.RequestCtx, shardingDef *appsv1.ShardingDefinition) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(shardingDef, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing Cluster")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, shardingDef, constant.ShardingDefLabelKey,
			recordEvent, &appsv1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ShardingDefinitionReconciler) available(rctx intctrlutil.RequestCtx, shardingDef *appsv1.ShardingDefinition) error {
	return r.status(rctx, shardingDef, appsv1.AvailablePhase, "")
}

func (r *ShardingDefinitionReconciler) unavailable(rctx intctrlutil.RequestCtx, shardingDef *appsv1.ShardingDefinition, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return r.status(rctx, shardingDef, appsv1.UnavailablePhase, message)
}

func (r *ShardingDefinitionReconciler) status(rctx intctrlutil.RequestCtx,
	shardingDef *appsv1.ShardingDefinition, phase appsv1.Phase, message string) error {
	patch := client.MergeFrom(shardingDef.DeepCopy())
	shardingDef.Status.ObservedGeneration = shardingDef.Generation
	shardingDef.Status.Phase = phase
	shardingDef.Status.Message = message
	return r.Client.Status().Patch(rctx.Ctx, shardingDef, patch)
}

func (r *ShardingDefinitionReconciler) validate(cli client.Client, rctx intctrlutil.RequestCtx, shardingDef *appsv1.ShardingDefinition) error {
	for _, validator := range []func(context.Context, client.Client, *appsv1.ShardingDefinition) error{
		r.validateTemplate,
		r.validateShardsLimit,
		r.validateProvisionNUpdateStrategy,
		r.validateLifecycleActions,
		r.validateSystemAccounts,
	} {
		if err := validator(rctx.Ctx, cli, shardingDef); err != nil {
			return err
		}
	}
	return r.immutableCheck(shardingDef)
}

func (r *ShardingDefinitionReconciler) validateTemplate(ctx context.Context, cli client.Client,
	shardingDef *appsv1.ShardingDefinition) error {
	template := shardingDef.Spec.Template

	if err := component.ValidateDefNameRegexp(template.CompDef); err != nil {
		return err
	}

	compDefs, err := listCompDefinitionsWithPattern(ctx, cli, template.CompDef)
	if err != nil {
		return err
	}
	if len(compDefs) == 0 {
		return fmt.Errorf("no component definition found for the specified template")
	}

	r.compDefs = compDefs // carry the component definitions for later use

	return nil
}

func (r *ShardingDefinitionReconciler) validateShardsLimit(ctx context.Context, cli client.Client,
	shardingDef *appsv1.ShardingDefinition) error {
	return nil
}

func (r *ShardingDefinitionReconciler) validateProvisionNUpdateStrategy(ctx context.Context, cli client.Client,
	shardingDef *appsv1.ShardingDefinition) error {
	supported := func(strategy *appsv1.UpdateStrategy) bool {
		if strategy == nil {
			return true
		}
		return *strategy == appsv1.SerialStrategy || *strategy == appsv1.ParallelStrategy
	}
	if !supported(shardingDef.Spec.ProvisionStrategy) {
		return fmt.Errorf("unsupported provision strategy: %s", *shardingDef.Spec.ProvisionStrategy)
	}
	if !supported(shardingDef.Spec.UpdateStrategy) {
		return fmt.Errorf("unsupported update strategy: %s", *shardingDef.Spec.UpdateStrategy)
	}
	return nil
}

func (r *ShardingDefinitionReconciler) validateLifecycleActions(ctx context.Context, cli client.Client,
	shardingDef *appsv1.ShardingDefinition) error {
	return nil
}

func (r *ShardingDefinitionReconciler) validateSystemAccounts(ctx context.Context, cli client.Client,
	shardingDef *appsv1.ShardingDefinition) error {
	if !checkUniqueItemWithValue(shardingDef.Spec.SystemAccounts, "Name", nil) {
		return fmt.Errorf("duplicate system accounts are specified")
	}

	for _, account := range shardingDef.Spec.SystemAccounts {
		if err := r.validateSystemAccountDefined(account.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *ShardingDefinitionReconciler) validateSystemAccountDefined(name string) error {
	for _, compDef := range r.compDefs {
		found := false
		for _, account := range compDef.Spec.SystemAccounts {
			if account.Name == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("system account %s is not defined in component definition %s", name, compDef.Name)
		}
	}
	return nil
}

func (r *ShardingDefinitionReconciler) immutableCheck(shardingDef *appsv1.ShardingDefinition) error {
	if r.skipImmutableCheck(shardingDef) {
		return nil
	}

	newHashValue, err := r.specHash(shardingDef)
	if err != nil {
		return err
	}

	hashValue, ok := shardingDef.Annotations[immutableHashAnnotationKey]
	if ok && hashValue != newHashValue {
		// TODO: fields been updated
		return fmt.Errorf("immutable fields can't be updated")
	}
	return nil
}

func (r *ShardingDefinitionReconciler) skipImmutableCheck(sdd *appsv1.ShardingDefinition) bool {
	if sdd.Annotations == nil {
		return false
	}
	skip, ok := sdd.Annotations[constant.SkipImmutableCheckAnnotationKey]
	return ok && strings.ToLower(skip) == "true"
}

func (r *ShardingDefinitionReconciler) specHash(shardingDef *appsv1.ShardingDefinition) (string, error) {
	data, err := json.Marshal(shardingDef.Spec)
	if err != nil {
		return "", err
	}
	hash := fnv.New32a()
	hash.Write(data)
	return rand.SafeEncodeString(fmt.Sprintf("%d", hash.Sum32())), nil
}

func (r *ShardingDefinitionReconciler) immutableHash(cli client.Client, rctx intctrlutil.RequestCtx,
	shardingDef *appsv1.ShardingDefinition) error {
	if r.skipImmutableCheck(shardingDef) {
		return nil
	}

	if shardingDef.Annotations != nil {
		_, ok := shardingDef.Annotations[immutableHashAnnotationKey]
		if ok {
			return nil
		}
	}

	patch := client.MergeFrom(shardingDef.DeepCopy())
	if shardingDef.Annotations == nil {
		shardingDef.Annotations = map[string]string{}
	}
	shardingDef.Annotations[immutableHashAnnotationKey], _ = r.specHash(shardingDef)
	return cli.Patch(rctx.Ctx, shardingDef, patch)
}

// resolveShardingDefinition resolves and returns the specific sharding definition object supported.
func resolveShardingDefinition(ctx context.Context, cli client.Reader, shardingDefName string) (*appsv1.ShardingDefinition, error) {
	shardingDefs, err := listShardingDefinitionsWithPattern(ctx, cli, shardingDefName)
	if err != nil {
		return nil, err
	}
	if len(shardingDefs) == 0 {
		return nil, fmt.Errorf("no sharding definition found for the specified name: %s", shardingDefName)
	}

	m := make(map[string]int)
	for i, def := range shardingDefs {
		m[def.Name] = i
	}
	// choose the latest one
	names := maps.Keys(m)
	slices.Sort(names)
	latestName := names[len(names)-1]

	return shardingDefs[m[latestName]], nil
}

// listShardingDefinitionsWithPattern returns all sharding definitions whose names match the given pattern
func listShardingDefinitionsWithPattern(ctx context.Context, cli client.Reader, name string) ([]*appsv1.ShardingDefinition, error) {
	shardingDefList := &appsv1.ShardingDefinitionList{}
	if err := cli.List(ctx, shardingDefList); err != nil {
		return nil, err
	}
	fullyMatched := make([]*appsv1.ShardingDefinition, 0)
	patternMatched := make([]*appsv1.ShardingDefinition, 0)
	for i, item := range shardingDefList.Items {
		if item.Name == name {
			fullyMatched = append(fullyMatched, &shardingDefList.Items[i])
		}
		if component.PrefixOrRegexMatched(item.Name, name) {
			patternMatched = append(patternMatched, &shardingDefList.Items[i])
		}
	}
	if len(fullyMatched) > 0 {
		return fullyMatched, nil
	}
	return patternMatched, nil
}

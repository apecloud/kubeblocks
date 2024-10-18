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

package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type dryRunner struct {
	ctx    context.Context
	cli    client.Client
	scheme *runtime.Scheme
}

func (r *dryRunner) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	v, _ := tree.GetRoot().(*tracev1.ReconciliationTrace)
	if isDesiredSpecChanged(v) {
		return kubebuilderx.ConditionSatisfied
	}
	return kubebuilderx.ConditionUnsatisfied
}

func (r *dryRunner) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	trace, _ := tree.GetRoot().(*tracev1.ReconciliationTrace)
	objs := tree.List(&corev1.ConfigMap{})
	var i18nResource *corev1.ConfigMap
	if len(objs) > 0 {
		i18nResource, _ = objs[0].(*corev1.ConfigMap)
	}

	root := &kbappsv1.Cluster{}
	objectKey := client.ObjectKeyFromObject(trace)
	if trace.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: trace.Spec.TargetObject.Namespace,
			Name:      trace.Spec.TargetObject.Name,
		}
	}
	if err := r.cli.Get(r.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}

	generator := newPlanGenerator(r.ctx, r.cli, r.scheme,
		cacheObjectLoader(r.ctx, r.cli, root, getKBOwnershipRules()),
		buildDescriptionFormatter(i18nResource, defaultLocale, trace.Spec.Locale))

	desiredRoot, err := applySpec(root.DeepCopy(), trace.Spec.DryRun.DesiredSpec)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	plan, err := generator.generatePlan(desiredRoot)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	plan.DesiredSpecRevision = getDesiredSpecRevision(trace.Spec.DryRun.DesiredSpec)
	trace.Status.DryRunResult = plan

	return kubebuilderx.Continue, nil
}

func applySpec(current *kbappsv1.Cluster, desiredSpecStr string) (*kbappsv1.Cluster, error) {
	// Convert the desiredSpec YAML string to a map
	specMap := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(desiredSpecStr), &specMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal desiredSpec: %w", err)
	}

	// Extract the current spec and apply the patch
	currentSpec, err := getSpecFieldAsStruct(current)
	if err != nil {
		return nil, fmt.Errorf("failed to get current spec: %w", err)
	}

	// Create a strategic merge patch
	patchData, err := strategicpatch.CreateTwoWayMergePatch(
		specMapToJSON(currentSpec),
		specMapToJSON(specMap),
		currentSpec,
	)
	if err != nil {
		return nil, err
	}

	// Apply the patch to the current spec
	modifiedSpec, err := strategicpatch.StrategicMergePatch(
		specMapToJSON(currentSpec),
		patchData,
		currentSpec,
	)
	if err != nil {
		return nil, err
	}

	modifiedSpecMap := make(map[string]interface{})
	if err = json.Unmarshal(modifiedSpec, &modifiedSpecMap); err != nil {
		return nil, err
	}

	// Convert the object to an unstructured map
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(current)
	if err != nil {
		return nil, err
	}

	// Update the spec in the object map
	if err := unstructured.SetNestedField(objMap, modifiedSpecMap, "spec"); err != nil {
		return nil, err
	}

	// Convert the modified map back to the original object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(objMap, current); err != nil {
		return nil, err
	}
	return current, nil
}

func dryRun(ctx context.Context, cli client.Client, scheme *runtime.Scheme) kubebuilderx.Reconciler {
	return &dryRunner{
		ctx:    context.WithValue(ctx, constant.DryRunContextKey, true),
		cli:    cli,
		scheme: scheme,
	}
}

func isDesiredSpecChanged(v *tracev1.ReconciliationTrace) bool {
	if v.Spec.DryRun == nil && v.Status.DryRunResult == nil {
		return false
	}
	if v.Spec.DryRun == nil || v.Status.DryRunResult == nil {
		return true
	}
	revision := getDesiredSpecRevision(v.Spec.DryRun.DesiredSpec)
	return revision != v.Status.DryRunResult.DesiredSpecRevision
}

func getDesiredSpecRevision(desiredSpec string) string {
	hf := fnv.New32()
	_, _ = hf.Write([]byte(desiredSpec))
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32()))
}

func cacheObjectLoader(ctx context.Context, cli client.Client, root *kbappsv1.Cluster, rules []OwnershipRule) objectLoader {
	return func() (map[model.GVKNObjKey]client.Object, error) {
		return getObjectsFromCache(ctx, cli, root, rules)
	}
}

var _ kubebuilderx.Reconciler = &dryRunner{}

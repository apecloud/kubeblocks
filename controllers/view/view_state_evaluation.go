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

package view

import (
	"context"
	"fmt"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"google.golang.org/protobuf/types/known/structpb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type stateEvaluation struct {
	ctx    context.Context
	reader client.Reader
	store  ObjectRevisionStore
	scheme *runtime.Scheme
}

func (s *stateEvaluation) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (s *stateEvaluation) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)

	// build new object set from cache
	root := &kbappsv1.Cluster{}
	objectKey := client.ObjectKeyFromObject(view)
	if view.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: view.Spec.TargetObject.Namespace,
			Name:      view.Spec.TargetObject.Name,
		}
	}
	if err := s.reader.Get(s.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// keep only the latest reconciliation cycle.
	// the basic idea is:
	// 1. traverse the view progress list from tail to head,
	// 2. read the corresponding version of the Cluster object from object store by objectChange.resourceVersion,
	// 3. evaluation its state,
	// if we find the first-false-then-true pattern, means a new reconciliation cycle starts.
	firstFalseStateFound := false
	clusterType := viewv1.ObjectType{
		APIVersion: kbappsv1.APIVersion,
		Kind:       kbappsv1.ClusterKind,
	}
	latestReconciliationCycleStart := 0
	for i := len(view.Status.CurrentState.Changes) - 1; i >= 0; i-- {
		change := view.Status.CurrentState.Changes[i]
		objType := objectReferenceToType(&change.ObjectReference)
		if *objType != clusterType {
			continue
		}
		objectRef := objectReferenceToRef(&change.ObjectReference)
		obj, err := s.store.Get(objectRef, change.Revision)
		if err != nil && !apierrors.IsNotFound(err) {
			return kubebuilderx.Commit, err
		}
		// handle revision lost after controller restarted
		if obj == nil {
			continue
		}
		expr := defaultStateEvaluationExpression
		if view.Spec.StateEvaluationExpression != nil {
			expr = *view.Spec.StateEvaluationExpression
		}
		state, err := doStateEvaluation(obj, expr)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		if !state && !firstFalseStateFound {
			firstFalseStateFound = true
		}
		if state && firstFalseStateFound {
			latestReconciliationCycleStart = i
			break
		}
	}
	if latestReconciliationCycleStart <= 0 {
		if view.Status.InitialObjectTree == nil {
			reference, err := getObjectReference(root, s.scheme)
			if err != nil {
				return kubebuilderx.Commit, err
			}
			view.Status.InitialObjectTree = &viewv1.ObjectTreeNode{
				Primary: *reference,
			}
		}
		return kubebuilderx.Continue, nil
	}

	// build new InitialObjectTree
	var err error
	view.Status.InitialObjectTree, err = getObjectTreeWithRevision(root, kbOwnershipRules, s.store, view.Status.CurrentState.Changes[latestReconciliationCycleStart].Revision, s.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	// delete unused object revisions
	for i := 0; i < latestReconciliationCycleStart; i++ {
		change := view.Status.CurrentState.Changes[i]
		objectRef := objectReferenceToRef(&change.ObjectReference)
		s.store.Delete(objectRef, view, change.Revision)
	}

	// truncate view
	view.Status.CurrentState.Changes = view.Status.CurrentState.Changes[latestReconciliationCycleStart:]

	return kubebuilderx.Continue, nil
}

func doStateEvaluation(object client.Object, expression viewv1.StateEvaluationExpression) (bool, error) {
	if expression.CELExpression == nil {
		return false, fmt.Errorf("CEL expression can't be empty")
	}

	// Convert the object to an unstructured map
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return false, fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	// Create a CEL environment with the object fields available
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("object", decls.NewMapType(decls.String, decls.Dyn)),
		),
	)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Compile the CEL expression
	ast, issues := env.Compile(expression.CELExpression.Expression)
	if issues.Err() != nil {
		return false, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	// Create a program
	prg, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL program: %w", err)
	}

	// Evaluate the expression with the object map
	objValue, err := structpb.NewStruct(unstructuredMap)
	if err != nil {
		return false, fmt.Errorf("failed to create structpb value: %w", err)
	}

	out, _, err := prg.Eval(map[string]any{
		"object": types.NewDynamicMap(types.DefaultTypeAdapter, objValue.AsMap()),
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	// Convert the output to a boolean
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression did not return a boolean")
	}

	return result, nil
}

func viewStateEvaluation(ctx context.Context, reader client.Reader, scheme *runtime.Scheme, store ObjectRevisionStore) kubebuilderx.Reconciler {
	return &stateEvaluation{
		ctx:    ctx,
		reader: reader,
		scheme: scheme,
		store:  store,
	}
}

var _ kubebuilderx.Reconciler = &stateEvaluation{}

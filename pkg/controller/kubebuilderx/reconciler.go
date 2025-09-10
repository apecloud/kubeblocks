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

package kubebuilderx

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type ObjectOption interface {
	ApplyToObject(*ObjectOptions)
}

type ObjectOptions struct {
	// if not empty, action should be done on the specified subresource
	SubResource string

	// if true, the object should not be reconciled
	SkipToReconcile bool

	// hooks are called before the object is manipulated
	Hooks []func(client.Object) error
}

type WithSubResource string

func (w WithSubResource) ApplyToObject(opts *ObjectOptions) {
	opts.SubResource = string(w)
}

type SkipToReconcile bool

func (o SkipToReconcile) ApplyToObject(opts *ObjectOptions) {
	opts.SkipToReconcile = bool(o)
}

type WithHook func(client.Object) error

func (o WithHook) ApplyToObject(opts *ObjectOptions) {
	opts.Hooks = append(opts.Hooks, o)
}

type ObjectTree struct {
	// TODO(free6om): should find a better place to hold these two params?
	context.Context
	client.Reader
	record.EventRecorder
	logr.Logger

	root            client.Object
	children        model.ObjectSnapshot
	childrenOptions map[model.GVKNObjKey]ObjectOptions

	// finalizer to protect all objects of this tree
	finalizer string
}

type TreeLoader interface {
	Load(context.Context, client.Reader, ctrl.Request, record.EventRecorder, logr.Logger) (*ObjectTree, error)
}

type CheckResult struct {
	Satisfied bool
	Err       error
}

var (
	// ConditionSatisfied means the corresponding Reconcile() should be invoked
	ConditionSatisfied = &CheckResult{Satisfied: true}

	// ConditionUnsatisfied means the corresponding Reconcile() should be skipped
	ConditionUnsatisfied = &CheckResult{}

	// ConditionUnsatisfiedWithError means the corresponding and all the following
	// Reconcile() should be skipped
	ConditionUnsatisfiedWithError = func(err error) *CheckResult {
		return &CheckResult{Satisfied: false, Err: err}
	}
)

type controlMethod string

const (
	cntn controlMethod = "Continue"
	cmmt controlMethod = "Commit"
	rtry controlMethod = "Retry"
)

type Result struct {
	Next       controlMethod
	RetryAfter time.Duration
}

var (
	// Continue tells the control flow to continue
	Continue = Result{Next: cntn}

	// Commit tells the control flow to stop and jump to the commit phase
	Commit = Result{Next: cmmt}

	// RetryAfter tells the control flow to stop, jump to the commit phase
	// and retry from the beginning with a delay specified by `after`.
	RetryAfter = func(after time.Duration) Result {
		return Result{Next: rtry, RetryAfter: after}
	}
)

var ErrDeepCopyFailed = errors.New("DeepCopyFailed")

type Reconciler interface {
	// PreCondition should return ConditionSatisfied if Reconcile() should be invoked,
	// otherwise return ConditionUnsatisfied.
	PreCondition(*ObjectTree) *CheckResult

	// Reconcile contains the business logic and modifies the object tree.
	Reconcile(tree *ObjectTree) (Result, error)
}

func (t *ObjectTree) DeepCopy() (*ObjectTree, error) {
	out := new(ObjectTree)
	if t.root != nil {
		root, ok := t.root.DeepCopyObject().(client.Object)
		if !ok {
			return nil, ErrDeepCopyFailed
		}
		out.root = root
	}
	children := make(model.ObjectSnapshot, len(t.children))
	for key, child := range t.children {
		childCopied, ok := child.DeepCopyObject().(client.Object)
		if !ok {
			return nil, ErrDeepCopyFailed
		}
		children[key] = childCopied
	}
	childrenOptions := make(map[model.GVKNObjKey]ObjectOptions, len(t.childrenOptions))
	for key, options := range t.childrenOptions {
		childrenOptions[key] = options
	}
	out.children = children
	out.childrenOptions = childrenOptions
	out.finalizer = t.finalizer
	out.Context = t.Context
	out.EventRecorder = t.EventRecorder
	out.Logger = t.Logger
	return out, nil
}

func (t *ObjectTree) Get(object client.Object) (client.Object, error) {
	name, err := model.GetGVKName(object)
	if err != nil {
		return nil, err
	}
	return t.children[*name], nil
}

func (t *ObjectTree) GetWithOption(object client.Object) (client.Object, ObjectOptions, error) {
	name, err := model.GetGVKName(object)
	if err != nil {
		return nil, ObjectOptions{}, err
	}
	return t.children[*name], t.childrenOptions[*name], nil
}

func (t *ObjectTree) GetRoot() client.Object {
	return t.root
}

func (t *ObjectTree) SetRoot(root client.Object) {
	t.root = root
}

func (t *ObjectTree) DeleteRoot() {
	t.root = nil
}

func (t *ObjectTree) List(obj client.Object) []client.Object {
	assignableTo := func(src, dst reflect.Type) bool {
		if dst == nil {
			return src == nil
		}
		return src.AssignableTo(dst)
	}
	objType := reflect.TypeOf(obj)
	objects := make([]client.Object, 0)
	for _, child := range t.children {
		vertexType := reflect.TypeOf(child)
		if assignableTo(vertexType, objType) {
			objects = append(objects, child)
		}
	}
	return objects
}

func (t *ObjectTree) GetSecondaryObjects() model.ObjectSnapshot {
	return t.children
}

func (t *ObjectTree) Add(objects ...client.Object) error {
	return t.replace(objects)
}

func (t *ObjectTree) AddWithOption(object client.Object, options ...ObjectOption) error {
	return t.replace([]client.Object{object}, options...)
}

func (t *ObjectTree) Update(object client.Object, options ...ObjectOption) error {
	return t.replace([]client.Object{object}, options...)
}

func (t *ObjectTree) replace(objects []client.Object, options ...ObjectOption) error {
	option := ObjectOptions{}
	for _, opt := range options {
		opt.ApplyToObject(&option)
	}

	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		t.childrenOptions[*name] = option
		t.children[*name] = object
	}
	return nil
}

func (t *ObjectTree) Delete(objects ...client.Object) error {
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		delete(t.children, *name)
	}
	return nil
}

func (t *ObjectTree) DeleteWithOption(object client.Object, options ...ObjectOption) error {
	name, err := model.GetGVKName(object)
	if err != nil {
		return err
	}
	delete(t.children, *name)
	if len(options) > 0 {
		option := ObjectOptions{}
		for _, opt := range options {
			opt.ApplyToObject(&option)
		}
		t.childrenOptions[*name] = option
	}
	return nil
}

func (t *ObjectTree) SetFinalizer(finalizer string) {
	t.finalizer = finalizer
}

func (t *ObjectTree) GetFinalizer() string {
	return t.finalizer
}

func NewObjectTree() *ObjectTree {
	return &ObjectTree{
		children:        make(model.ObjectSnapshot),
		childrenOptions: make(map[model.GVKNObjKey]ObjectOptions),
	}
}

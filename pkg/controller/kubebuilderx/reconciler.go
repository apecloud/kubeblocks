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

type ObjectTree struct {
	// TODO(free6om): should find a better place to hold these two params?
	record.EventRecorder
	logr.Logger

	root     client.Object
	children model.ObjectSnapshot

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

var ResultUnsatisfied = &CheckResult{}
var ResultSatisfied = &CheckResult{Satisfied: true}

type ControlMethod string

const (
	// Continue tells the control flow to continue
	Continue ControlMethod = "Continue"

	// Commit tells the control flow to stop and jump to the commit phase
	Commit   ControlMethod = "Commit"

	// Retry tells the control flow to stop and retry from the beginning with a delay specified by Result.RetryAfter
	Retry    ControlMethod = "Retry"
)

type Result struct {
	Next       ControlMethod
	RetryAfter time.Duration
}

var ErrDeepCopyFailed = errors.New("DeepCopyFailed")

type Reconciler interface {
	PreCondition(*ObjectTree) *CheckResult
	Reconcile(tree *ObjectTree) (Result, error)
}

func CheckResultWithError(err error) *CheckResult {
	return &CheckResult{Satisfied: false, Err: err}
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
	out.children = children
	out.finalizer = t.finalizer
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
	return t.replace(objects...)
}

func (t *ObjectTree) Update(objects ...client.Object) error {
	return t.replace(objects...)
}

func (t *ObjectTree) replace(objects ...client.Object) error {
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
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

func (t *ObjectTree) DeleteSecondaryObjects() {
	t.children = make(model.ObjectSnapshot)
}

func (t *ObjectTree) SetFinalizer(finalizer string) {
	t.finalizer = finalizer
}

func (t *ObjectTree) GetFinalizer() string {
	return t.finalizer
}

func NewObjectTree() *ObjectTree {
	return &ObjectTree{
		children: make(model.ObjectSnapshot),
	}
}

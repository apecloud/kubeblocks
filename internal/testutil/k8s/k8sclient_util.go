/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package testutil

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/mock/gomock"
	ginkgov2 "github.com/onsi/ginkgo/v2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mock_client "github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

type CallMockOptions = func(call *gomock.Call)
type CallerFunction = func() *gomock.Call
type DoReturnedFunction = any

type HandleGetReturnedObject = func(key client.ObjectKey, obj client.Object) error
type HandlePatchReturnedObject = func(obj client.Object, patch client.Patch) error
type HandleListReturnedObject = func(list client.ObjectList) error

type CallMockReturnedOptions = func(callHelper *callHelper, call *gomock.Call)
type CallMockGetReturnedOptions = func(callHelper *callHelper, call *gomock.Call, _ HandleGetReturnedObject) error
type CallMockPatchReturnedOptions = func(callHelper *callHelper, call *gomock.Call, _ HandlePatchReturnedObject) error
type CallMockListReturnedOptions = func(callHelper *callHelper, call *gomock.Call, _ HandleListReturnedObject) error

type callHelper struct {
	callerOnce   sync.Once
	callerFn     CallerFunction
	doReturnedFn DoReturnedFunction
}

type K8sClientMockHelper struct {
	ctrl      *gomock.Controller
	k8sClient *mock_client.MockClient

	getCaller    callHelper
	updateCaller callHelper
	listCaller   callHelper
	patchCaller  callHelper
	deleteCaller callHelper
}

func (h *callHelper) Caller(newCaller func() (CallerFunction, DoReturnedFunction)) CallerFunction {
	h.callerOnce.Do(func() {
		h.callerFn, h.doReturnedFn = newCaller()
	})
	return h.callerFn
}

func (helper *K8sClientMockHelper) Client() client.Client {
	return helper.k8sClient
}

func (helper *K8sClientMockHelper) Controller() *gomock.Controller {
	return helper.ctrl
}

func (helper *K8sClientMockHelper) Finish() {
	helper.ctrl.Finish()
}

func (helper *K8sClientMockHelper) mockMethod(callHelper *callHelper, options ...any) {
	for _, option := range options {
		call := callHelper.callerFn()
		switch f := option.(type) {
		case CallMockOptions:
			f(call)
		case CallMockReturnedOptions:
			f(callHelper, call)
		}
	}
}

func (helper *K8sClientMockHelper) MockGetMethod(options ...any) {
	helper.getCaller.Caller(func() (CallerFunction, DoReturnedFunction) {
		caller := func() *gomock.Call {
			return helper.k8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any())
		}
		doAndReturn := func(caller *gomock.Call, fnWrap HandleGetReturnedObject) {
			caller.DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return fnWrap(key, obj)
			})
		}
		return caller, doAndReturn
	})
	helper.mockMethod(&helper.getCaller, options...)
}

func (helper *K8sClientMockHelper) MockUpdateMethod(options ...any) {
	helper.updateCaller.Caller(func() (CallerFunction, DoReturnedFunction) {
		caller := func() *gomock.Call {
			return helper.k8sClient.EXPECT().Update(gomock.Any(), gomock.Any())
		}
		doAndReturn := func(caller *gomock.Call, fnWrap func(obj client.Object) error) {
			caller.DoAndReturn(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
				return fnWrap(obj)
			})
		}
		return caller, doAndReturn
	})
	helper.mockMethod(&helper.updateCaller, options...)
}

func (helper *K8sClientMockHelper) MockDeleteMethod(options ...any) {
	helper.deleteCaller.Caller(func() (CallerFunction, DoReturnedFunction) {
		caller := func() *gomock.Call {
			return helper.k8sClient.EXPECT().Delete(gomock.Any(), gomock.Any())
		}
		doAndReturn := func(caller *gomock.Call, fnWrap func(obj client.Object) error) {
			caller.DoAndReturn(func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				return fnWrap(obj)
			})
		}
		return caller, doAndReturn
	})
	helper.mockMethod(&helper.updateCaller, options...)
}

func (helper *K8sClientMockHelper) MockListMethod(options ...any) {
	helper.listCaller.Caller(func() (CallerFunction, DoReturnedFunction) {
		caller := func() *gomock.Call {
			return helper.k8sClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		}
		doAndReturn := func(caller *gomock.Call, fnWrap HandleListReturnedObject) {
			caller.DoAndReturn(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
				return fnWrap(list)
			})
		}
		return caller, doAndReturn
	})
	helper.mockMethod(&helper.listCaller, options...)
}

func (helper *K8sClientMockHelper) MockPatchMethod(options ...any) {
	helper.patchCaller.Caller(func() (CallerFunction, DoReturnedFunction) {
		caller := func() *gomock.Call {
			return helper.k8sClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any())
		}
		doAndReturn := func(caller *gomock.Call, fnWrap HandlePatchReturnedObject) {
			caller.DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				return fnWrap(obj, patch)
			})
		}
		return caller, doAndReturn
	})
	helper.mockMethod(&helper.patchCaller, options...)
}

func SetupK8sMock() (*gomock.Controller, *mock_client.MockClient) {
	ctrl := gomock.NewController(ginkgov2.GinkgoT())
	client := mock_client.NewMockClient(ctrl)
	return ctrl, client
}

func SetGetReturnedObject(out client.Object, expectedObj client.Object) {
	outVal := reflect.ValueOf(out)
	objVal := reflect.ValueOf(expectedObj)
	reflect.Indirect(outVal).Set(reflect.Indirect(objVal))
}

func SetListReturnedObjects(list client.ObjectList, objects []runtime.Object) error {
	return apimeta.SetList(list, objects)
}

func NewK8sMockClient() *K8sClientMockHelper {
	ctrl, client := SetupK8sMock()
	clientHelper := K8sClientMockHelper{
		ctrl:      ctrl,
		k8sClient: client,
	}

	return &clientHelper
}

func WithTimes(n int) CallMockOptions {
	return func(call *gomock.Call) {
		call.Times(n)
	}
}

func WithMinTimes(n int) CallMockOptions {
	return func(call *gomock.Call) {
		call.MinTimes(n)
	}
}

func WithMaxTimes(n int) CallMockOptions {
	return func(call *gomock.Call) {
		call.MaxTimes(n)
	}
}

func WithAnyTimes() CallMockOptions {
	return func(call *gomock.Call) {
		call.AnyTimes()
	}
}

func WithFailed(err error, times ...CallMockOptions) CallMockOptions {
	return func(call *gomock.Call) {
		call.Return(err).AnyTimes()
		handleTimes(call, times...)
	}
}

func WithSucceed(times ...CallMockOptions) CallMockOptions {
	return func(call *gomock.Call) {
		call.Return(nil).AnyTimes()
		handleTimes(call, times...)
	}
}

func WithConstructListReturnedResult(r []runtime.Object) HandleListReturnedObject {
	return func(list client.ObjectList) error {
		return SetListReturnedObjects(list, r)
	}
}

type CallbackFn = func(sequence int, r []runtime.Object)

func WithConstructListSequenceResult(mockObjsList [][]runtime.Object, fns ...CallbackFn) HandleListReturnedObject {
	sequenceAccessCounter := 0
	return func(list client.ObjectList) error {
		for _, fn := range fns {
			fn(sequenceAccessCounter, mockObjsList[sequenceAccessCounter])
		}
		if err := SetListReturnedObjects(list, mockObjsList[sequenceAccessCounter]); err != nil {
			return err
		}
		if sequenceAccessCounter < len(mockObjsList)-1 {
			sequenceAccessCounter++
		}
		return nil
	}
}

type MockGetReturned struct {
	Object client.Object
	Err    error
}

func WithConstructSequenceResult(mockObjs map[client.ObjectKey][]MockGetReturned) HandleGetReturnedObject {
	sequenceAccessCounter := make(map[client.ObjectKey]int, len(mockObjs))
	return func(key client.ObjectKey, obj client.Object) error {
		accessibleSequence, ok := mockObjs[key]
		if !ok {
			return fmt.Errorf("not exist: %v", key)
		}
		index := sequenceAccessCounter[key]
		mockReturned := accessibleSequence[index]
		if mockReturned.Err == nil {
			SetGetReturnedObject(obj, mockReturned.Object)
		}
		if index < len(accessibleSequence)-1 {
			sequenceAccessCounter[key]++
		}
		return mockReturned.Err
	}
}

func WithConstructGetResult(mockObj client.Object) HandleGetReturnedObject {
	return func(key client.ObjectKey, obj client.Object) error {
		SetGetReturnedObject(obj, mockObj)
		return nil
	}
}

func WithConstructSimpleGetResult(mockObjs []client.Object) HandleGetReturnedObject {
	mockMap := make(map[client.ObjectKey]client.Object, len(mockObjs))
	for _, obj := range mockObjs {
		mockMap[client.ObjectKeyFromObject(obj)] = obj
	}
	return func(key client.ObjectKey, obj client.Object) error {
		if mockObj, ok := mockMap[key]; ok {
			SetGetReturnedObject(obj, mockObj)
			return nil
		}
		return fmt.Errorf("failed to get object: %v", key)
	}
}

func WithListReturned(action HandleListReturnedObject, times ...CallMockOptions) CallMockReturnedOptions {
	return func(helper *callHelper, call *gomock.Call) {
		switch fn := helper.doReturnedFn.(type) {
		case func(_ *gomock.Call, _ HandleListReturnedObject):
			fn(call, func(list client.ObjectList) error {
				return action(list)
			})
			handleTimes(call, times...)
		default:
			panic("not walk here!")
		}
	}
}

func handleTimes(call *gomock.Call, times ...CallMockOptions) {
	for _, time := range times {
		time(call)
	}
}

func WithGetReturned(action HandleGetReturnedObject, times ...CallMockOptions) CallMockReturnedOptions {
	return func(helper *callHelper, call *gomock.Call) {
		switch fn := helper.doReturnedFn.(type) {
		case func(_ *gomock.Call, _ HandleGetReturnedObject):
			fn(call, func(key client.ObjectKey, obj client.Object) error {
				return action(key, obj)
			})
			handleTimes(call, times...)
		default:
			panic("not walk here!")
		}
	}
}

func WithPatchReturned(action HandlePatchReturnedObject, times ...CallMockOptions) CallMockReturnedOptions {
	return func(helper *callHelper, call *gomock.Call) {
		switch fn := helper.doReturnedFn.(type) {
		case func(_ *gomock.Call, _ HandlePatchReturnedObject):
			fn(call, func(obj client.Object, patch client.Patch) error {
				return action(obj, patch)
			})
			handleTimes(call, times...)
		default:
			panic("not walk here!")
		}
	}
}

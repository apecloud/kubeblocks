/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testutil

import (
	"reflect"

	"github.com/golang/mock/gomock"
	ginkgov2 "github.com/onsi/ginkgo/v2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mock_client "github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

//go:generate go run github.com/golang/mock/mockgen -copyright_file ../../../hack/boilerplate.go.txt -destination mocks/k8sclient_mocks.go sigs.k8s.io/controller-runtime/pkg/client Client

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

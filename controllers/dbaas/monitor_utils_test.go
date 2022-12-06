/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"testing"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func assertEqual(t *testing.T, expect string, actual string) {
	if expect != actual {
		t.Errorf("fail: expected %s, actual %s", expect, actual)
	}
}

func TestCalcCharacterType(t *testing.T) {
	{
		expectType := KMysql
		actualType := CalcCharacterType("state.mysql")
		assertEqual(t, expectType, actualType)
	}

	{
		expectType := KMysql
		actualType := CalcCharacterType("state.mysql-8")
		assertEqual(t, expectType, actualType)
	}

	{
		expectType := ""
		actualType := CalcCharacterType("other")
		assertEqual(t, expectType, actualType)
	}
}

func TestIsWellKnownCharacterType(t *testing.T) {
	var wellKnownCharacterTypeFunc = map[string]func(cluster *dbaasv1alpha1.Cluster, component *Component) error{
		"mysql": setMysqlComponent,
		"redis": nil,
	}

	if !isWellKnowCharacterType("mysql", wellKnownCharacterTypeFunc) {
		t.Error("mysql is well known characterType")
	}

	if isWellKnowCharacterType("redis", wellKnownCharacterTypeFunc) {
		t.Error("redis is not well known characterType")
	}

	if isWellKnowCharacterType("other", wellKnownCharacterTypeFunc) {
		t.Error("other is not well known characterType")
	}
}

/*
Copyright 2022 The KubeBlocks Authors

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

/*
Copyright 2022.

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

package controllerutil

import "testing"

func assertEqual(t *testing.T, expect string, actual string) {
	if expect != actual {
		t.Errorf("fail: expected %s, actual %s", expect, actual)
	}
}

func TestCalcCharacterType(t *testing.T) {
	{
		expectType := "mysql"
		actualType := CalcCharacterType("state.mysql", "wesql")
		assertEqual(t, expectType, actualType)
	}

	{
		expectType := "mysql"
		actualType := CalcCharacterType("state.mysql-8", "wesql")
		assertEqual(t, expectType, actualType)
	}

	{
		expectType := ""
		actualType := CalcCharacterType("other", "wqsql")
		assertEqual(t, expectType, actualType)
	}
}

func TestIsWellKnownCharacterType(t *testing.T) {
	var wellKnownCharacterType = map[string]bool{
		"mysql": true,
		"redis": false,
	}

	if !isWellKnowCharacterType("mysql", wellKnownCharacterType) {
		t.Error("mysql is well known characterType")
	}

	if isWellKnowCharacterType("redis", wellKnownCharacterType) {
		t.Error("redis is not well known characterType")
	}

	if isWellKnowCharacterType("other", wellKnownCharacterType) {
		t.Error("other is not well known characterType")
	}
}

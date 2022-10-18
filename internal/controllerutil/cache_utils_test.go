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

package controllerutil

import "testing"

func TestGetCacheBytesValue(t *testing.T) {
	const byteValue = "byteValue"
	{
		v, err := GetCacheBytesValue("byteKey", func() ([]byte, error) {
			return []byte(byteValue), nil
		})
		if string(v) != byteValue || err != nil {
			t.Errorf("fail: expected %s, actual %s", byteValue, string(v))
		}
	}

	{
		v, err := GetCacheBytesValue("byteKey", nil)
		if string(v) != byteValue || err != nil {
			t.Errorf("fail: expected %s, actual %s", byteValue, string(v))
		}
	}
}

func TestGetCacheCUETplValue(t *testing.T) {
	{
		_, err := GetCacheCUETplValue("tplKey", func() (*CUETpl, error) {
			return nil, nil
		})
		if err != nil {
			t.Error("fail: get cue tpl from cache fail")
		}
	}

	{
		_, err := GetCacheCUETplValue("tplKey", nil)
		if err != nil {
			t.Error("fail: get cue tpl from cache fail")
		}
	}
}

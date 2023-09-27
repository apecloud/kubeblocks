/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package template

import (
	"bufio"
	"bytes"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/generics"
)

type MatchResourceFunc func(object client.Object) bool

func CustomizedObjFromYaml[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](filePath string, signature func(T, PT, L, PL)) (PT, error) {
	objList, err := CustomizedObjectListFromYaml[T, PT, L, PL](filePath, signature)
	if err != nil {
		return nil, err
	}
	if len(objList) == 0 {
		return nil, nil
	}
	return objList[0], nil
}

func CustomizedObjectListFromYaml[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](yamlfile string, signature func(T, PT, L, PL)) ([]PT, error) {
	objBytes, err := os.ReadFile(yamlfile)
	if err != nil {
		return nil, err
	}
	objList := make([]PT, 0)
	reader := bufio.NewReader(bytes.NewReader(objBytes))
	for {
		doc, err := yaml.NewYAMLReader(reader).Read()
		if len(doc) == 0 {
			break
		}
		if err != nil {
			return nil, err
		}
		objList = append(objList, CreateTypedObjectFromYamlByte[T, PT, L, PL](doc, signature))
	}
	return objList, nil
}

func CreateTypedObjectFromYamlByte[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](yamlBytes []byte, _ func(T, PT, L, PL)) PT {
	var obj PT
	if err := yaml.Unmarshal(yamlBytes, &obj); err != nil {
		return nil
	}
	return obj
}

func GetTypedResourceObjectBySignature[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](objects []client.Object, _ func(T, PT, L, PL), matchers ...MatchResourceFunc) PT {
	for _, object := range objects {
		obj, ok := object.(PT)
		if !ok {
			continue
		}
		found := true
		for _, matcher := range matchers {
			if !matcher(obj) {
				found = false
				break
			}
		}
		if found {
			return obj
		}
	}
	return nil
}

func WithResourceName(name string) MatchResourceFunc {
	return func(object client.Object) bool {
		return name == "" || object.GetName() == name
	}
}

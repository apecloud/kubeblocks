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

package app

import (
	"bytes"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/generics"
)

func CustomizedObjFromYaml[T generics.Object, PT generics.PObject[T],
	L generics.ObjList[T]](filePath string, signature func(T, L)) (PT, error) {
	objList, err := CustomizedObjectListFromYaml[T, PT, L](filePath, signature)
	if err != nil {
		return nil, err
	}
	if len(objList) == 0 {
		return nil, nil
	}
	return objList[0], nil
}

func CustomizedObjectListFromYaml[T generics.Object, PT generics.PObject[T],
	L generics.ObjList[T]](yamlfile string, signature func(T, L)) ([]PT, error) {
	objBytes, err := os.ReadFile(yamlfile)
	if err != nil {
		return nil, err
	}
	objList := make([]PT, 0)
	for _, doc := range bytes.Split(objBytes, []byte("---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		objList = append(objList, CreateTypedObjectFromYamlByte[T, PT, L](doc, signature))
	}
	return objList, nil
}

func CreateTypedObjectFromYamlByte[T generics.Object, PT generics.PObject[T],
	L generics.ObjList[T]](yamlBytes []byte, _ func(T, L)) PT {
	var obj PT
	if err := yaml.Unmarshal(yamlBytes, &obj); err != nil {
		return nil
	}
	return obj
}

func GetTypedResourceObjectBySignature[T generics.Object, PT generics.PObject[T],
	L generics.ObjList[T]](objects []client.Object, _ func(T, L)) PT {
	for _, object := range objects {
		if cd, ok := object.(PT); ok {
			return cd
		}
	}
	return nil
}

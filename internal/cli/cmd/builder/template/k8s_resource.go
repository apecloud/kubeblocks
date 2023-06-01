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

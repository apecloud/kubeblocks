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

package app

import (
	"bytes"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/generics"
)

func scanDirectoryPath(rootPath string) ([]string, error) {
	dirs, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}
	resourceList := make([]string, 0)
	for _, d := range dirs {
		if d.IsDir() {
			subDirectory, err := scanDirectoryPath(filepath.Join(rootPath, d.Name()))
			if err != nil {
				return nil, err
			}
			resourceList = append(resourceList, subDirectory...)
			continue
		}
		if filepath.Ext(d.Name()) != ".yaml" {
			continue
		}
		resourceList = append(resourceList, filepath.Join(rootPath, d.Name()))
	}
	return resourceList, nil
}

func getResourceMeta(yamlBytes []byte) (metav1.TypeMeta, error) {
	type k8sObj struct {
		metav1.TypeMeta `json:",inline"`
	}
	var o k8sObj
	err := yaml.Unmarshal(yamlBytes, &o)
	if err != nil {
		return metav1.TypeMeta{}, err
	}
	return o.TypeMeta, nil
}

func CreateObjectsFromDirectory(rootPath string) ([]client.Object, error) {
	allObjs := make([]client.Object, 0)

	// create cr from yaml
	resourceList, err := scanDirectoryPath(rootPath)
	if err != nil {
		return nil, err
	}
	for _, resourceFile := range resourceList {
		yamlBytes, err := os.ReadFile(resourceFile)
		if err != nil {
			return nil, err
		}
		objects, err := createObjectsFromYaml(yamlBytes)
		if err != nil {
			return nil, err
		}
		allObjs = append(allObjs, objects...)
	}
	return allObjs, nil
}

func createObjectsFromYaml(yamlBytes []byte) ([]client.Object, error) {
	objects := make([]client.Object, 0)
	for _, doc := range bytes.Split(yamlBytes, []byte("---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		meta, err := getResourceMeta(doc)
		if err != nil {
			return nil, err
		}
		switch meta.Kind {
		case kindFromResource(corev1.ConfigMap{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.ConfigMapSignature))
		case kindFromResource(corev1.Secret{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.SecretSignature))
		case kindFromResource(appsv1alpha1.ConfigConstraint{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.ConfigConstraintSignature))
		case kindFromResource(appsv1alpha1.ClusterDefinition{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.ClusterDefinitionSignature))
		case kindFromResource(appsv1alpha1.ClusterVersion{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.ClusterVersionSignature))
		case kindFromResource(appsv1alpha1.BackupPolicyTemplate{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.BackupPolicyTemplateSignature))
		case kindFromResource(dataprotectionv1alpha1.BackupTool{}):
			objects = append(objects, CreateTypedObjectFromYamlByte(doc, generics.BackupToolSignature))
		}
	}
	return objects, nil
}

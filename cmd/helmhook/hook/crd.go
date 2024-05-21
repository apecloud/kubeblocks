/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package hook

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type UpdateCRD struct {
	BasedHandler
}

func (p *UpdateCRD) Handle(ctx *UpgradeContext) (err error) {
	crdList, err := parseCRDs(ctx.CRDPath)
	if err != nil {
		return err
	}

	for _, crd := range crdList {
		Log("create/update CRD: %s", crd.GetName())
		existing, err := ctx.CRDClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.GetName(), metav1.GetOptions{})
		switch {
		case err == nil:
			crd.SetResourceVersion(existing.GetResourceVersion())
			_, err = ctx.CRDClient.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, &crd, metav1.UpdateOptions{})
		case apierrors.IsNotFound(err):
			_, err = ctx.CRDClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, &crd, metav1.CreateOptions{})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func parseCRDs(path string) ([]apiextensionsv1.CustomResourceDefinition, error) {
	var (
		err      error
		info     os.FileInfo
		files    []string
		filePath = path
	)

	// Return the error if ErrorIfPathMissing exists
	if info, err = os.Stat(path); err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("require path[%s] is directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		files = append(files, e.Name())
	}

	Log("reading CRDs from path: %s", path)
	crdList, err := readCRDs(filePath, files)
	if err != nil {
		return nil, err
	}
	return crdList, nil
}

func readCRDs(basePath string, files []string) ([]apiextensionsv1.CustomResourceDefinition, error) {
	var crds []apiextensionsv1.CustomResourceDefinition

	crdExts := sets.NewString(".yaml", ".yml")
	for _, file := range files {
		if !crdExts.Has(filepath.Ext(file)) {
			continue
		}
		docs, err := readDocuments(filepath.Join(basePath, file))
		if err != nil {
			return nil, err
		}
		crds = append(crds, docs...)
		Log("read CRDs from file: %s", file)
	}
	return crds, nil
}

func readDocuments(fp string) ([]apiextensionsv1.CustomResourceDefinition, error) {
	var objs []apiextensionsv1.CustomResourceDefinition

	b, err := os.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	reader := k8syaml.NewYAMLToJSONDecoder(bufio.NewReader(bytes.NewReader(b)))
	for {
		var obj apiextensionsv1.CustomResourceDefinition
		if err = reader.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		objs = append(objs, obj)
	}

	return objs, nil
}

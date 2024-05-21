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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type UpdateCR struct {
	BasedHandler
}

func (u *UpdateCR) Handle(context *UpgradeContext) error {
	if len(context.UpdatedObjects) == 0 {
		return nil
	}

	for gvr, objects := range context.UpdatedObjects {
		Log("update GVR resource: %s", gvr.String())
		for _, cr := range objects {
			Log("update resource: %s", cr.GetName())
			item, err := apiruntime.DefaultUnstructuredConverter.ToUnstructured(cr)
			if err != nil {
				return err
			}
			if err = updateOrCreate(context, gvr, &unstructured.Unstructured{Object: item}); err != nil {
				return err
			}
		}
	}
	return nil
}

func updateOrCreate(context *UpgradeContext, gvr schema.GroupVersionResource, cr *unstructured.Unstructured) error {
	_, err := context.Resource(gvr).Update(context, cr, metav1.UpdateOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		Log("resource not found and create: %s", cr.GetName())
		_, err = context.Resource(gvr).Create(context, cr, metav1.CreateOptions{})
	}
	return err
}

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

package multiversion

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const ccResource = "configconstraints"

var configConstraintBeta1GVR = appsv1beta1.SchemeGroupVersion.WithResource(ccResource)

func init() {
	// Upgrade from version 0.7/8.x  to version 0.9.x
	hook.RegisterCRDConversion(configConstraintBeta1GVR, hook.NewNoVersion(0, 9), &ccConversion{},
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8))
}

type ccConversion struct {
}

func (c *ccConversion) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	ccList, err := cli.KBClient.AppsV1alpha1().ConfigConstraints().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var objects []client.Object
	for _, oldObj := range ccList.Items {
		// If v1alpha1 is converted from v1beta1 version
		if hasConversionVersion(&oldObj) {
			hook.Log("configconstraint[%s] v1alpha1 is converted from v1beta1 version and ignore.", client.ObjectKeyFromObject(&oldObj).String())
			continue
		}
		// If the converted version v1beta1 already exists
		if hasValidBetaVersion(ctx, &oldObj, cli.KBClient) {
			hook.Log("configconstraint[%s] v1beta1 already exist and ignore.", client.ObjectKeyFromObject(&oldObj).String())
			continue
		}
		newObj, err := convert(&oldObj)
		if err != nil {
			return nil, err
		}
		objects = append(objects, newObj)
	}
	return objects, err
}

func convert(from *appsv1alpha1.ConfigConstraint) (*appsv1beta1.ConfigConstraint, error) {
	newObj := appsv1beta1.ConfigConstraint{
		TypeMeta: metav1.TypeMeta{
			Kind:       from.Kind,
			APIVersion: configConstraintBeta1GVR.GroupVersion().String(),
		},
	}
	return &newObj, from.ConvertTo(&newObj)
}

func hasValidBetaVersion(ctx context.Context, obj *appsv1alpha1.ConfigConstraint, kbClient *versioned.Clientset) bool {
	newObj, err := kbClient.AppsV1beta1().ConfigConstraints().Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return false
	}

	return hasConversionVersion(newObj)
}

func hasConversionVersion(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 {
		return false
	}
	return annotations[constant.KubeblocksAPIConversionTypeAnnotationName] == constant.MigratedAPIVersion
}

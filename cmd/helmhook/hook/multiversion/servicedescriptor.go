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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

// covert appsv1alpha1.servicedescriptor resources to appsv1.servicedescriptor

var (
	sdResource = "servicedescriptors"
	sdGVR      = appsv1.GroupVersion.WithResource(sdResource)
)

func init() {
	hook.RegisterCRDConversion(sdGVR, hook.NewNoVersion(1, 0), sdHandler(),
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func sdHandler() hook.ConversionHandler {
	return &convertor{
		kind:       "ServiceDescriptor",
		source:     &sdConvertor{},
		target:     &sdConvertor{},
		namespaces: []string{"default"}, // TODO: namespaces
	}
}

type sdConvertor struct{}

func (c *sdConvertor) list(ctx context.Context, cli *versioned.Clientset, namespace string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().ServiceDescriptors(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *sdConvertor) used(context.Context, *versioned.Clientset, string, string) (bool, error) {
	return true, nil
}

func (c *sdConvertor) get(ctx context.Context, cli *versioned.Clientset, namespace, name string) (client.Object, error) {
	return cli.AppsV1().ServiceDescriptors(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *sdConvertor) convert(source client.Object) []client.Object {
	sd := source.(*appsv1alpha1.ServiceDescriptor)
	return []client.Object{
		&appsv1.ServiceDescriptor{
			Spec: appsv1.ServiceDescriptorSpec{
				ServiceKind:    sd.Spec.ServiceKind,
				ServiceVersion: sd.Spec.ServiceVersion,
				Endpoint:       c.credentialVar(sd.Spec.Endpoint),
				Host:           c.credentialVar(sd.Spec.Host),
				Port:           c.credentialVar(sd.Spec.Port),
				Auth:           c.credentialAuth(sd.Spec.Auth),
			},
		},
	}
}

func (c *sdConvertor) credentialVar(v *appsv1alpha1.CredentialVar) *appsv1.CredentialVar {
	if v == nil {
		return nil
	}
	return &appsv1.CredentialVar{
		Value:     v.Value,
		ValueFrom: v.ValueFrom,
	}
}

func (c *sdConvertor) credentialAuth(v *appsv1alpha1.ConnectionCredentialAuth) *appsv1.CredentialAuth {
	if v == nil {
		return nil
	}
	return &appsv1.CredentialAuth{
		Username: c.credentialVar(v.Username),
		Password: c.credentialVar(v.Password),
	}
}

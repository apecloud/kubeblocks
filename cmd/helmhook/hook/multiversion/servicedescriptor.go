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
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
)

// covert appsv1alpha1.servicedescriptor resources to appsv1.servicedescriptor

var (
	sdResource = "servicedescriptors"
	sdGVR      = appsv1.GroupVersion.WithResource(sdResource)
)

func init() {
	hook.RegisterCRDConversion(sdGVR, hook.NewNoVersion(1, 0), &sdConvertor{},
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

type sdConvertor struct {
	namespaces []string // TODO: namespaces

	sds           map[string]*appsv1alpha1.ServiceDescriptor
	errors        map[string]error
	unused        sets.Set[string]
	native        sets.Set[string]
	beenConverted sets.Set[string]
	toBeConverted sets.Set[string]
}

func (c *sdConvertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	for _, namespace := range c.namespaces {
		sdList, err := cli.KBClient.AppsV1alpha1().ServiceDescriptors(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for i, sd := range sdList.Items {
			namespacedName := fmt.Sprintf("%s/%s", sd.GetNamespace(), sd.GetName())
			c.sds[namespacedName] = &sdList.Items[i]

			sdv1, err2 := c.existed(ctx, cli, sd.GetNamespace(), sd.GetName())
			switch {
			case err2 != nil:
				c.errors[namespacedName] = err2
			case sdv1 == nil:
				c.toBeConverted.Insert(namespacedName)
			case c.converted(sdv1):
				c.beenConverted.Insert(namespacedName)
			default:
				c.native.Insert(namespacedName)
			}
		}
	}
	c.dump()

	objects := make([]client.Object, 0)
	for namespacedName := range c.toBeConverted {
		objects = append(objects, c.convert(c.sds[namespacedName]))
	}
	return objects, nil
}

func (c *sdConvertor) existed(ctx context.Context, cli hook.CRClient, namespace, name string) (*appsv1.ServiceDescriptor, error) {
	obj, err := cli.KBClient.AppsV1().ServiceDescriptors(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}

func (c *sdConvertor) converted(sd *appsv1.ServiceDescriptor) bool {
	if sd != nil && sd.GetAnnotations() != nil {
		_, ok := sd.GetAnnotations()[convertedFromAnnotationKey]
		return ok
	}
	return false
}

func (c *sdConvertor) dump() {
	hook.Log("ServiceDescriptor conversion to v1 status")
	hook.Log("\tunused ServiceDescriptors")
	hook.Log(c.doubleTableFormat(sets.List(c.unused)))
	hook.Log("\thas native ServiceDescriptors defined")
	hook.Log(c.doubleTableFormat(sets.List(c.native)))
	hook.Log("\thas been converted ServiceDescriptors")
	hook.Log(c.doubleTableFormat(sets.List(c.beenConverted)))
	hook.Log("\tto be converted ServiceDescriptors")
	hook.Log(c.doubleTableFormat(sets.List(c.toBeConverted)))
	hook.Log("\terror occurred when perform pre-check")
	hook.Log(c.doubleTableFormat(maps.Keys(c.errors), c.errors))
}

func (c *sdConvertor) doubleTableFormat(items []string, errors ...map[string]error) string {
	formattedErr := func(key string) string {
		if len(errors) == 0 {
			return ""
		}
		if err, ok := errors[0][key]; ok {
			return fmt.Sprintf(": %s", err.Error())
		}
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString("\t\t" + item + formattedErr(item) + "\n")
	}
	return sb.String()
}

func (c *sdConvertor) convert(sd *appsv1alpha1.ServiceDescriptor) client.Object {
	// TODO: filter labels & annotations
	labels := func() map[string]string {
		return sd.GetLabels()
	}
	annotations := func() map[string]string {
		m := map[string]string{}
		maps.Copy(m, sd.GetAnnotations())
		b, _ := json.Marshal(sd)
		m[convertedFromAnnotationKey] = string(b)
		return m
	}
	return &appsv1.ServiceDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   sd.GetNamespace(),
			Name:        sd.GetName(),
			Labels:      labels(),
			Annotations: annotations(),
		},
		Spec: appsv1.ServiceDescriptorSpec{
			ServiceKind:    sd.Spec.ServiceKind,
			ServiceVersion: sd.Spec.ServiceVersion,
			Endpoint:       c.credentialVar(sd.Spec.Endpoint),
			Host:           c.credentialVar(sd.Spec.Host),
			Port:           c.credentialVar(sd.Spec.Port),
			Auth:           c.credentialAuth(sd.Spec.Auth),
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

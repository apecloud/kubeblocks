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

// covert appsv1alpha1.componentversion resources to appsv1.componentversion

var (
	cmpvResource = "componentversions"
	cmpvGVR      = appsv1.GroupVersion.WithResource(cmpvResource)
)

func init() {
	hook.RegisterCRDConversion(cmpvGVR, hook.NewNoVersion(1, 0), cmpvHandler(),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func cmpvHandler() hook.ConversionHandler {
	return &convertor{
		sourceKind: &cmpvConvertor{},
		targetKind: &cmpvConvertor{},
	}
}

type cmpvConvertor struct{}

func (c *cmpvConvertor) kind() string {
	return "ComponentVersion"
}

func (c *cmpvConvertor) list(ctx context.Context, cli *versioned.Clientset, _ string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().ComponentVersions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *cmpvConvertor) get(ctx context.Context, cli *versioned.Clientset, _, name string) (client.Object, error) {
	return cli.AppsV1().ComponentVersions().Get(ctx, name, metav1.GetOptions{})
}

func (c *cmpvConvertor) convert(source client.Object) []client.Object {
	cmpv := source.(*appsv1alpha1.ComponentVersion)
	return []client.Object{
		&appsv1.ComponentVersion{
			Spec: appsv1.ComponentVersionSpec{
				CompatibilityRules: c.compatibilityRules(cmpv.Spec.CompatibilityRules),
				Releases:           c.releases(cmpv.Spec.Releases),
			},
		},
	}
}

func (c *cmpvConvertor) compatibilityRules(rules []appsv1alpha1.ComponentVersionCompatibilityRule) []appsv1.ComponentVersionCompatibilityRule {
	if len(rules) == 0 {
		return nil
	}
	newRules := make([]appsv1.ComponentVersionCompatibilityRule, 0)
	for i := range rules {
		newRules = append(newRules, appsv1.ComponentVersionCompatibilityRule{
			CompDefs: rules[i].CompDefs,
			Releases: rules[i].Releases,
		})
	}
	return newRules
}

func (c *cmpvConvertor) releases(releases []appsv1alpha1.ComponentVersionRelease) []appsv1.ComponentVersionRelease {
	if len(releases) == 0 {
		return nil
	}
	newReleases := make([]appsv1.ComponentVersionRelease, 0)
	for i := range releases {
		newReleases = append(newReleases, appsv1.ComponentVersionRelease{
			Name:           releases[i].Name,
			Changes:        releases[i].Changes,
			ServiceVersion: releases[i].ServiceVersion,
			Images:         releases[i].Images,
		})
	}
	return newReleases
}

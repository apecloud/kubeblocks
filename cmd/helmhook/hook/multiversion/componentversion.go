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
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// covert appsv1alpha1.componentversion resources to appsv1.componentversion

var (
	cmpvResource = "componentversions"
	cmpvGVR      = appsv1.GroupVersion.WithResource(cmpvResource)
)

func init() {
	hook.RegisterCRDConversion(cmpvGVR, hook.NewNoVersion(1, 0), &cmpvConvertor{},
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

type cmpvConvertor struct {
	namespaces []string // TODO: namespaces

	cmpvs         map[string]*appsv1alpha1.ComponentVersion
	errors        map[string]error
	unused        sets.Set[string]
	native        sets.Set[string]
	beenConverted sets.Set[string]
	toBeConverted sets.Set[string]
}

func (c *cmpvConvertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	cmpvList, err := cli.KBClient.AppsV1alpha1().ComponentVersions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i, cmpv := range cmpvList.Items {
		c.cmpvs[cmpv.GetName()] = &cmpvList.Items[i]

		used, err1 := c.used(ctx, cli, cmpv.GetName(), c.namespaces)
		if err1 != nil {
			c.errors[cmpv.GetName()] = err1
			continue
		}
		if !used {
			c.unused.Insert(cmpv.GetName())
			continue
		}

		cmpdv1, err2 := c.existed(ctx, cli, cmpv.GetName())
		switch {
		case err2 != nil:
			c.errors[cmpv.GetName()] = err2
		case cmpdv1 == nil:
			c.toBeConverted.Insert(cmpv.GetName())
		case c.converted(cmpdv1):
			c.beenConverted.Insert(cmpv.GetName())
		default:
			c.native.Insert(cmpv.GetName())
		}
	}
	c.dump()

	objects := make([]client.Object, 0)
	for name := range c.toBeConverted {
		objects = append(objects, c.convert(c.cmpvs[name]))
	}
	return objects, nil
}

func (c *cmpvConvertor) used(ctx context.Context, cli hook.CRClient, cmpdName string, namespaces []string) (bool, error) {
	selectors := []string{
		fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName),
		// TODO: annotation key
		fmt.Sprintf("%s=%s", constant.ComponentDefinitionLabelKey, cmpdName),
	}
	opts := metav1.ListOptions{
		LabelSelector: strings.Join(selectors, ","),
	}

	used := false
	for _, namespace := range namespaces {
		compList, err := cli.KBClient.AppsV1alpha1().Components(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}
		used = used || (len(compList.Items) > 0)
	}
	return used, nil
}

func (c *cmpvConvertor) existed(ctx context.Context, cli hook.CRClient, name string) (*appsv1.ComponentVersion, error) {
	obj, err := cli.KBClient.AppsV1().ComponentVersions().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}

func (c *cmpvConvertor) converted(cmpv *appsv1.ComponentVersion) bool {
	if cmpv != nil && cmpv.GetAnnotations() != nil {
		_, ok := cmpv.GetAnnotations()[convertedFromAnnotationKey]
		return ok
	}
	return false
}

func (c *cmpvConvertor) dump() {
	hook.Log("ComponentVersion conversion to v1 status")
	hook.Log("\tunused ComponentVersions")
	hook.Log(c.doubleTableFormat(sets.List(c.unused)))
	hook.Log("\thas native ComponentVersions defined")
	hook.Log(c.doubleTableFormat(sets.List(c.native)))
	hook.Log("\thas been converted ComponentVersions")
	hook.Log(c.doubleTableFormat(sets.List(c.beenConverted)))
	hook.Log("\tto be converted ComponentVersions")
	hook.Log(c.doubleTableFormat(sets.List(c.toBeConverted)))
	hook.Log("\terror occurred when perform pre-check")
	hook.Log(c.doubleTableFormat(maps.Keys(c.errors), c.errors))
}

func (c *cmpvConvertor) doubleTableFormat(items []string, errors ...map[string]error) string {
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

func (c *cmpvConvertor) convert(cmpv *appsv1alpha1.ComponentVersion) client.Object {
	// TODO: filter labels & annotations
	labels := func() map[string]string {
		return cmpv.GetLabels()
	}
	annotations := func() map[string]string {
		m := map[string]string{}
		maps.Copy(m, cmpv.GetAnnotations())
		b, _ := json.Marshal(cmpv)
		m[convertedFromAnnotationKey] = string(b)
		return m
	}
	return &appsv1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cmpv.GetName(),
			Labels:      labels(),
			Annotations: annotations(),
		},
		Spec: appsv1.ComponentVersionSpec{
			CompatibilityRules: c.compatibilityRules(cmpv.Spec.CompatibilityRules),
			Releases:           c.releases(cmpv.Spec.Releases),
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

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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// covert appsv1alpha1.clusterversion resources to appsv1.componentversion
//
// conversion example:
// -----
// kind: clusterVersion
// metadata:
//   name: cv1
// spec:
//   clusterDefinitionRef: cd
//   componentVersions:
//     - componentDefRef: comp1
//       versionsCtx:
// 	       initContainers:
// 	         - name: init
// 	           image: init:v1
// 	       containers:
// 	         - name: main
// 	           image: main:v1
//     - componentDefRef: comp2
//       versionsCtx:
// 	       initContainers:
// 	         - name: init
// 	           image: init:v1
// 	       containers:
// 	         - name: main
// 	           image: main:v1
// -----
// kind: clusterVersion
// metadata:
//   name: cv2
// spec:
//   clusterDefinitionRef: cd
//   componentVersions:
//     - componentDefRef: comp1
//       versionsCtx:
// 	       initContainers:
// 	         - name: init
// 	           image: init:v2
// 	       containers:
// 	         - name: main
// 	           image: main:v2
//   \/
//   \/
//   \/
// -----
// kind: componentVersion
// metadata:
//   name: cd-comp1
// spec:
//   releases:
//     - name: cv1
//       serviceVersion: v.0.0.1
// 	     images:
// 	       init: init:v1
// 	       main: main:v1
//     - name: cv2
//       serviceVersion: v.0.0.2
// 	     images:
// 	       init: init:v2
// 	       main: main:v2
//   compatibilityRules:
//     - compDefs: [cd-comp1]
//       releases: [cv1, cv2]
// -----
// kind: componentVersion
// metadata:
//   name: cd-comp2
// spec:
//   releases:
//     - name: cv1
//       serviceVersion: v.0.0.1
// 	     images:
// 	       init: init:v1
// 	       main: main:v1
//   compatibilityRules:
//     - compDefs: [cd-comp2]
//       releases: [cv1]

var (
	cvResource = "clusterversions"
	cvGVR      = appsv1.GroupVersion.WithResource(cvResource)
)

func init() {
	hook.RegisterCRDConversion(cvGVR, hook.NewNoVersion(1, 0), &cvConvertor{},
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

type cvConvertor struct {
	namespaces []string // TODO: namespaces

	cvs           map[string]*appsv1alpha1.ClusterVersion
	errors        map[string]error
	unused        sets.Set[string]
	native        sets.Set[string]
	beenConverted sets.Set[string]
	toBeConverted sets.Set[string]
}

func (c *cvConvertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	cvList, err := cli.KBClient.AppsV1alpha1().ClusterVersions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i, cv := range cvList.Items {
		c.cvs[cv.GetName()] = &cvList.Items[i]

		used, err1 := c.used(ctx, cli, cv.GetName(), c.namespaces)
		if err1 != nil {
			c.errors[cv.GetName()] = err1
			continue
		}
		if !used {
			c.unused.Insert(cv.GetName())
			continue
		}

		cmpvsv1, err2 := c.existed(ctx, cli, &cvList.Items[i])
		switch {
		case err2 != nil:
			c.errors[cv.GetName()] = err2
		case len(cmpvsv1) == 0:
			c.toBeConverted.Insert(cv.GetName())
		case c.converted(cmpvsv1):
			c.beenConverted.Insert(cv.GetName())
		default:
			c.native.Insert(cv.GetName())
		}
	}
	c.dump()

	objects := make([]client.Object, 0)
	for name := range c.toBeConverted {
		objects = append(objects, c.convert(c.cvs[name])...)
	}
	return objects, nil
}

func (c *cvConvertor) used(ctx context.Context, cli hook.CRClient, cvName string, namespaces []string) (bool, error) {
	selectors := []string{
		fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName),
		fmt.Sprintf("%s=%s", constant.AppVersionLabelKey, cvName),
	}
	opts := metav1.ListOptions{
		LabelSelector: strings.Join(selectors, ","),
	}

	used := false
	for _, namespace := range namespaces {
		compList, err := cli.KBClient.AppsV1alpha1().Clusters(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}
		used = used || (len(compList.Items) > 0)
	}
	return used, nil
}

func (c *cvConvertor) existed(ctx context.Context, cli hook.CRClient, cv *appsv1alpha1.ClusterVersion) ([]*appsv1.ComponentVersion, error) {
	cmpvs := c.convert(cv)
	for _, cmpv := range cmpvs {
		obj, err := cli.KBClient.AppsV1().ComponentVersions().Get(ctx, cmpv.GetName(), metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
	}
	return cmpvs, nil
}

func (c *cvConvertor) converted(cmpvs []*appsv1.ComponentVersion) bool {
	converted := false
	for _, cmpv := range cmpvs {
		if cmpv != nil && cmpv.GetAnnotations() != nil {
			_, ok := cmpv.GetAnnotations()[convertedFromAnnotationKey]
			converted = converted && ok
		}
	}
	return converted
}

func (c *cvConvertor) dump() {
	hook.Log("ClusterVersion conversion to v1 status")
	hook.Log("\tunused ClusterVersions")
	hook.Log(c.doubleTableFormat(sets.List(c.unused)))
	hook.Log("\thas native ClusterVersions defined")
	hook.Log(c.doubleTableFormat(sets.List(c.native)))
	hook.Log("\thas been converted ClusterVersions")
	hook.Log(c.doubleTableFormat(sets.List(c.beenConverted)))
	hook.Log("\tto be converted ClusterVersions")
	hook.Log(c.doubleTableFormat(sets.List(c.toBeConverted)))
	hook.Log("\terror occurred when perform pre-check")
	hook.Log(c.doubleTableFormat(maps.Keys(c.errors), c.errors))
}

func (c *cvConvertor) doubleTableFormat(items []string, errors ...map[string]error) string {
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

func (c *cvConvertor) convert(cv *appsv1alpha1.ClusterVersion) []client.Object {
	// TODO: filter labels & annotations
	labels := func() map[string]string {
		return cv.GetLabels()
	}
	annotations := func() map[string]string {
		m := map[string]string{}
		maps.Copy(m, cv.GetAnnotations())
		b, _ := json.Marshal(cv)
		m[convertedFromAnnotationKey] = string(b)
		return m
	}

	versions := map[string][]appsv1.ComponentVersionRelease{}
	for _, v := range cv.Spec.ComponentVersions {
		if len(v.VersionsCtx.InitContainers) == 0 && len(v.VersionsCtx.Containers) == 0 {
			continue
		}
		releases, ok := versions[v.ComponentDefRef]
		if !ok {
			releases = make([]appsv1.ComponentVersionRelease, 0)
		}
		release := appsv1.ComponentVersionRelease{
			Name:           c.releaseName(cv),
			ServiceVersion: c.serviceVersion(cv),
			Images:         map[string]string{},
		}
		for _, containers := range [][]corev1.Container{v.VersionsCtx.InitContainers, v.VersionsCtx.InitContainers} {
			for _, cc := range containers {
				release.Images[cc.Name] = cc.Image
			}
		}
		releases = append(releases, release)
		versions[v.ComponentDefRef] = releases
	}

	objects := make([]client.Object, 0)
	for compDefRef := range versions {
		obj := &appsv1.ComponentVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name:        c.cmpvName(cv, compDefRef),
				Labels:      labels(),
				Annotations: annotations(),
			},
			Spec: appsv1.ComponentVersionSpec{
				Releases: versions[compDefRef],
				CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
					{
						CompDefs: []string{
							generatedCmpdName(cv.Spec.ClusterDefinitionRef, compDefRef),
						},
						Releases: c.releaseNames(versions[compDefRef]),
					},
				},
			},
		}
		objects = append(objects, obj)
	}
	return objects
}

func (c *cvConvertor) cmpvName(cv *appsv1alpha1.ClusterVersion, compDefRef string) string {
	return fmt.Sprintf("%s-%s", cv.Spec.ClusterDefinitionRef, compDefRef)
}

func (c *cvConvertor) releaseName(cv *appsv1alpha1.ClusterVersion) string {
	return cv.GetName()
}

func (c *cvConvertor) serviceVersion(cv *appsv1alpha1.ClusterVersion) string {
	return cv.GetName()
}

func (c *cvConvertor) releaseNames(releases []appsv1.ComponentVersionRelease) []string {
	names := make([]string, 0)
	for _, release := range releases {
		names = append(names, release.Name)
	}
	return names
}

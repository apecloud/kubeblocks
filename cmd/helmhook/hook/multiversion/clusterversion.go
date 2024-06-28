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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
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
	hook.RegisterCRDConversion(cvGVR, hook.NewNoVersion(1, 0), cvHandler(),
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func cvHandler() hook.ConversionHandler {
	return &convertor{
		sourceKind: &cvConvertor{},
		targetKind: &cvConvertor{},
	}
}

type cvConvertor struct{}

func (c *cvConvertor) kind() string {
	return "ClusterVersion"
}

func (c *cvConvertor) list(ctx context.Context, cli *versioned.Clientset, _ string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().ClusterVersions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *cvConvertor) get(ctx context.Context, cli *versioned.Clientset, _, name string) (client.Object, error) {
	return nil, apierrors.NewNotFound(cvGVR.GroupResource(), name)
}

func (c *cvConvertor) convert(source client.Object) []client.Object {
	cv := source.(*appsv1alpha1.ClusterVersion)

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

// func (c *cvConvertor) cmpvName(cv *appsv1alpha1.ClusterVersion, compDefRef string) string {
//	return fmt.Sprintf("%s-%s", cv.Spec.ClusterDefinitionRef, compDefRef)
// }

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

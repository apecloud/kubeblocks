/*
Copyright ApeCloud, Inc.

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

package smoketest

import (
	"context"
	"log"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

const (
	timeout  time.Duration = time.Second * 360
	interval time.Duration = time.Second * 1
)

func SmokeTest() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks smoke test", func() {

		It("check addon auto-install", func() {
			allEnabled, err := checkAddons()
			if err != nil {
				log.Println(err)
			}
			Expect(allEnabled).Should(BeTrue())

		})
		It("run test cases", func() {
			dir, err := os.Getwd()
			if err != nil {
				log.Println(err)
			}
			folders, _ := e2eutil.GetFolders(dir + "/testdata/smoketest")
			for _, folder := range folders {
				if folder == dir+"/testdata/smoketest" {
					continue
				}
				log.Println("folder: " + folder)
				files, _ := e2eutil.GetFiles(folder)
				e2eutil.WaitTime(400000000)
				clusterVersions := e2eutil.GetClusterVersion(folder)
				if len(clusterVersions) > 1 {
					for _, clusterVersion := range clusterVersions {
						if len(files) > 0 {
							file := e2eutil.GetClusterCreateYaml(files)
							e2eutil.ReplaceClusterVersionRef(file, clusterVersion)
							runTestCases(files)
						}
					}
				} else {
					runTestCases(files)
				}
			}
		})
	})
}

func runTestCases(files []string) {
	for _, file := range files {
		By("test " + file)
		b := e2eutil.OpsYaml(file, "apply")
		Expect(b).Should(BeTrue())
		podStatusResult := e2eutil.CheckPodStatus()
		log.Println(podStatusResult)
		for _, result := range podStatusResult {
			Eventually(func(g Gomega) {
				g.Expect(result).Should(BeTrue())
			}).Should(Succeed())
		}
		clusterStatusResult := e2eutil.CheckClusterStatus()
		Eventually(func(g Gomega) {
			g.Expect(clusterStatusResult).Should(BeTrue())
		}).Should(Succeed())
	}
	if len(files) > 0 {
		file := e2eutil.GetClusterCreateYaml(files)
		e2eutil.OpsYaml(file, "delete")
	}
}

func checkAddons() (bool, error) {
	e2eutil.WaitTime(400000000)
	var Dynamic dynamic.Interface
	addons := make(map[string]bool)
	allEnabled := true
	objects, err := Dynamic.Resource(types.AddonGVR()).List(context.TODO(), metav1.ListOptions{
		LabelSelector: e2eutil.BuildAddonLabelSelector(),
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if objects == nil || len(objects.Items) == 0 {
		klog.V(1).Info("No Addons found")
		return false, nil
	}
	for _, obj := range objects.Items {
		addon := extensionsv1alpha1.Addon{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &addon); err != nil {
			return false, err
		}

		if addon.Status.ObservedGeneration == 0 {
			klog.V(1).Infof("Addon %s is not observed yet", addon.Name)
			allEnabled = false
			continue
		}

		installable := false
		if addon.Spec.InstallSpec != nil {
			installable = addon.Spec.Installable.AutoInstall
		}
		if addon.Spec.InstallSpec != nil {
			installable = addon.Spec.Installable.AutoInstall
		}

		klog.V(1).Infof("Addon: %s, enabled: %v, status: %s, auto-install: %v",
			addon.Name, addon.Spec.InstallSpec.GetEnabled(), addon.Status.Phase, installable)
		// addon is enabled, then check its status
		if addon.Spec.InstallSpec.GetEnabled() {
			addons[addon.Name] = true
			if addon.Status.Phase != extensionsv1alpha1.AddonEnabled {
				klog.V(1).Infof("Addon %s is not enabled yet", addon.Name)
				addons[addon.Name] = false
				allEnabled = false
			}
		}
	}
	return allEnabled, nil
}

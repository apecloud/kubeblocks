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
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

const (
	timeout  time.Duration = time.Second * 360
	interval time.Duration = time.Second * 1
)

type Options struct {
	Dynamic dynamic.Interface
}

func SmokeTest() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks smoke test", func() {
		It("check addon", func() {
			cfg, err := e2eutil.GetConfig()
			if err != nil {
				logrus.WithError(err).Fatal("could not get config")
			}
			dynamic, err := dynamic.NewForConfig(cfg)
			if err != nil {
				logrus.WithError(err).Fatal("could not generate dynamic client for config")
			}
			objects, err := dynamic.Resource(types.AddonGVR()).List(context.TODO(), metav1.ListOptions{
				LabelSelector: e2eutil.BuildAddonLabelSelector(),
			})
			if err != nil && !apierrors.IsNotFound(err) {
				log.Println(err)
			}
			if objects == nil || len(objects.Items) == 0 {
				log.Println("No Addons found")
			}
			if len(objects.Items) > 0 {
				for _, obj := range objects.Items {
					addon := extensionsv1alpha1.Addon{}
					if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &addon); err != nil {
						log.Println(err)
					}
					if addon.Status.ObservedGeneration == 0 {
						log.Printf("Addon %s is not observed yet", addon.Name)
					}
					log.Printf("Addon: %s, enabled: %v, status: %s",
						addon.Name, addon.Spec.InstallSpec.GetEnabled(), addon.Status.Phase)
					// addon is enabled, then check its status
					if addon.Spec.InstallSpec.GetEnabled() {
						if addon.Status.Phase != extensionsv1alpha1.AddonEnabled {
							log.Printf("Addon %s is not enabled yet", addon.Name)
						}
					}
				}
			}
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
				var clusterVersions []string
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
		Eventually(func(g Gomega) {
			podStatusResult := e2eutil.CheckPodStatus()
			for _, result := range podStatusResult {
				g.Expect(result).Should(BeTrue())
			}
		}, time.Second*180, time.Second*1).Should(Succeed())
		Eventually(func(g Gomega) {
			clusterStatusResult := e2eutil.CheckClusterStatus()
			g.Expect(clusterStatusResult).Should(BeTrue())
		}, time.Second*180, time.Second*1).Should(Succeed())

	}
	if len(files) > 0 {
		file := e2eutil.GetClusterCreateYaml(files)
		e2eutil.OpsYaml(file, "delete")
	}
}

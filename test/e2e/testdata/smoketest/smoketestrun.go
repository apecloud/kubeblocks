/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	. "github.com/apecloud/kubeblocks/test/e2e"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
	"github.com/apecloud/kubeblocks/test/testutils"
)

const (
	timeout  time.Duration = time.Second * 1500
	interval time.Duration = time.Second * 10
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
			objects, err := dynamic.Resource(testutils.AddonGVR()).List(context.TODO(), metav1.ListOptions{
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
			log.Println("====================start " + TestType + " e2e test====================")
			if len(TestType) == 0 {
				folders, _ := e2eutil.GetFolders(dir + "/testdata/smoketest")
				for _, folder := range folders {
					if folder == dir+"/testdata/smoketest" {
						continue
					}
					log.Println("folder: " + folder)
					getFiles(folder)
				}
			} else {
				getFiles(dir + "/testdata/smoketest/" + TestType)
			}
		})
	})
}

func getFiles(folder string) {
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

func runTestCases(files []string) {
	var clusterName, nameSpace string
	var testResult bool
	shouldSkip := false
	for _, file := range files {
		fileName := e2eutil.GetPrefix(file, "/")
		if len(SkipCase) > 0 {
			if strings.Contains(SkipCase, ",") {
				for _, c := range strings.Split(SkipCase, ",") {
					if strings.Contains(file, c) {
						e2eResult := NewResult(fileName, false, "")
						TestResults = append(TestResults, e2eResult)
						shouldSkip = true
						break
					}
				}
				if shouldSkip {
					shouldSkip = false
					continue
				}
			} else {
				if strings.Contains(file, SkipCase) {
					e2eResult := NewResult(fileName, false, "")
					TestResults = append(TestResults, e2eResult)
					continue
				}
			}
		}
		if strings.Contains(file, "restore") {
			backups := "kubectl get backup | awk '{print $1}' | tail -n +2"
			log.Println(backups)
			backupList := e2eutil.ExecCommandReadline(backups)
			log.Println(backupList)
			Eventually(func(g Gomega) {
				for _, backup := range backupList {
					cmd := "kubectl get backup " + backup + " -o=jsonpath='{.status.phase}'"
					log.Println(cmd)
					log.Println(e2eutil.ExecCommand(cmd))
					g.Expect(e2eutil.ExecCommand(cmd)).Should(Equal("Completed"))
				}
			}, timeout, interval).Should(Succeed())
		}
		b := e2eutil.OpsYaml(file, "create")
		if strings.Contains(file, "00") || strings.Contains(file, "restore") {
			clusterName, nameSpace = e2eutil.GetName(file)
			log.Println("clusterName is " + clusterName)
		}
		Expect(b).Should(BeTrue())
		if !strings.Contains(file, "stop") {
			testResult = false
			Eventually(func(g Gomega) {
				e2eutil.WaitTime(1000000)
				podStatusResult := e2eutil.CheckPodStatus(clusterName, nameSpace)
				for _, result := range podStatusResult {
					g.Expect(result).Should(BeTrue())
				}
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				clusterStatusResult := e2eutil.CheckClusterStatus(clusterName, nameSpace, "Running")
				g.Expect(clusterStatusResult).Should(BeTrue())
			}, timeout, interval).Should(Succeed())
			testResult = true
		} else {
			testResult = false
			Eventually(func(g Gomega) {
				clusterStatusResult := e2eutil.CheckClusterStatus(clusterName, nameSpace, "Stopped")
				g.Expect(clusterStatusResult).Should(BeTrue())
			}, timeout, interval).Should(Succeed())
			testResult = true
		}
		log.Println(testResult)
		if testResult {
			e2eResult := NewResult(fileName, testResult, "")
			TestResults = append(TestResults, e2eResult)
		} else {
			out := troubleShooting(clusterName)
			e2eResult := NewResult(fileName, testResult, out)
			TestResults = append(TestResults, e2eResult)
		}
	}
}

func troubleShooting(clusterName string) string {
	cmd := "kubectl get all -l app.kubernetes.io/instance=" + clusterName
	allResourceStatus := e2eutil.ExecCommand(cmd)
	command := "kubectl describe cluster " + clusterName
	clusterEvents := e2eutil.ExecCommand(command)
	return allResourceStatus + clusterEvents
}

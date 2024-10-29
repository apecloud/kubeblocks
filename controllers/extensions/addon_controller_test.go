/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package extensions

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Addon controller", func() {
	const (
		clusterName = "test-cluster"
	)
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// non-namespaced
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.AddonSignature, true, ml)

		inNS := client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS))
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS,
			client.HasLabels{
				constant.AddonNameLabelKey,
			})
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS,
			client.HasLabels{
				constant.AppManagedByLabelKey,
			})

		// delete rest mocked objects
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)

		// By("deleting the Namespace to perform the tests")
		// Eventually(func(g Gomega) {
		// 	namespace := testCtx.GetNamespaceObj()
		// 	err := testCtx.Cli.Delete(testCtx.Ctx, &namespace)
		// 	g.Expect(client.IgnoreNotFound(err)).To(Not(HaveOccurred()))
		// 	g.Expect(client.IgnoreNotFound(testCtx.Cli.Get(
		// 		testCtx.Ctx, testCtx.GetNamespaceKey(), &namespace))).To(Not(HaveOccurred()))
		// }).Should(Succeed())
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("Addon controller test", func() {
		var addon *extensionsv1alpha1.Addon
		var key types.NamespacedName
		var clusterKey types.NamespacedName
		BeforeEach(func() {
			cleanEnv()
			const distro = "kubeblocks"
			extensionsv1alpha1.SetKubeServerVersionWithDistro("1", "24", "0", distro)
			Expect(client.IgnoreAlreadyExists(testCtx.CreateNamespace())).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			cleanEnv()
			viper.Set(constant.CfgKeyCtrlrMgrTolerations, "")
			viper.Set(constant.CfgKeyCtrlrMgrAffinity, "")
			viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "")
		})

		doReconcile := func() (ctrl.Result, error) {
			addonReconciler := &AddonReconciler{
				Client: testCtx.Cli,
				Scheme: testCtx.Cli.Scheme(),
			}
			req := reconcile.Request{
				NamespacedName: key,
			}
			return addonReconciler.Reconcile(ctx, req)
		}

		doReconcileOnce := func(g Gomega) {
			By("Reconciling once")
			result, err := doReconcile()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result.Requeue).Should(BeFalse())
		}

		getJob := func(g Gomega, jobKey client.ObjectKey) *batchv1.Job {
			job := &batchv1.Job{}
			g.Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				g.Expect(testCtx.Cli.Get(ctx, jobKey, job)).Should(Succeed())
			}).Should(Succeed())
			return job
		}

		fakeCompletedJob := func(g Gomega, jobKey client.ObjectKey) {
			job := getJob(g, jobKey)
			job.Status.Succeeded = 1
			job.Status.Active = 0
			job.Status.Failed = 0
			g.Expect(testCtx.Cli.Status().Update(ctx, job)).Should(Succeed())
		}

		fakeFailedJob := func(g Gomega, jobKey client.ObjectKey) {
			job := getJob(g, jobKey)
			job.Status.Failed = 1
			job.Status.Active = 0
			job.Status.Succeeded = 0
			g.Expect(testCtx.Cli.Status().Update(ctx, job)).Should(Succeed())
		}

		fakeActiveJob := func(g Gomega, jobKey client.ObjectKey) {
			job := getJob(g, jobKey)
			job.Status.Active = 1
			job.Status.Succeeded = 0
			job.Status.Failed = 0
			g.Expect(testCtx.Cli.Status().Update(ctx, job)).Should(Succeed())
		}

		checkedJobDeletion := func(g Gomega, jobKey client.ObjectKey) {
			job := &batchv1.Job{}
			err := testCtx.Cli.Get(ctx, jobKey, job)
			if err == nil {
				g.Expect(job.DeletionTimestamp.IsZero()).Should(BeFalse())
				return
			}
			g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		}

		fakeInstallationCompletedJob := func(expectedObservedGeneration int) {
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getInstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				fakeCompletedJob(g, jobKey)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonEnabled))
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(expectedObservedGeneration))
				checkedJobDeletion(g, jobKey)
			}).Should(Succeed())
		}

		fakeInstallationFailedJob := func(expectedObservedGeneration int) {
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getInstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				fakeFailedJob(g, jobKey)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonFailed))
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(expectedObservedGeneration))
			}).Should(Succeed())
		}

		fakeUninstallationFailedJob := func(expectedObservedGeneration int) {
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getUninstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				fakeFailedJob(g, jobKey)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				result, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				g.Expect(result.Requeue).To(BeTrue())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				checkedJobDeletion(g, jobKey)
			}).Should(Succeed())
		}

		createAddonSpecWithRequiredAttributes := func(modifiers func(newOjb *extensionsv1alpha1.Addon)) {
			if modifiers != nil {
				addon = testapps.CreateCustomizedObj(&testCtx, "addon/addon.yaml",
					&extensionsv1alpha1.Addon{}, modifiers)
			} else {
				addon = testapps.CreateCustomizedObj(&testCtx, "addon/addon.yaml",
					&extensionsv1alpha1.Addon{})
			}
			key = types.NamespacedName{
				Name: addon.Name,
			}
			Expect(addon.Spec.DefaultInstallValues).ShouldNot(BeEmpty())
		}

		addonStatusPhaseCheck := func(genID int, expectPhase extensionsv1alpha1.AddonPhase, handler func()) {
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Generation).Should(BeEquivalentTo(genID))
				g.Expect(addon.Spec.InstallSpec).ShouldNot(BeNil())
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(genID))
				g.Expect(addon.Status.Phase).Should(Equal(expectPhase))
			}).Should(Succeed())

			if handler == nil {
				return
			}

			handler()
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Generation).Should(BeEquivalentTo(genID))
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(genID))
				g.Expect(addon.Status.Phase).Should(Equal(expectPhase))
			}).Should(Succeed())
		}

		enablingPhaseCheck := func(genID int) {
			addonStatusPhaseCheck(genID, extensionsv1alpha1.AddonEnabling, func() {
				By("By fake active install job")
				jobKey := client.ObjectKey{
					Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
					Name:      getInstallJobName(addon),
				}
				Eventually(func(g Gomega) {
					fakeActiveJob(g, jobKey)
				}).Should(Succeed())
			})
		}

		disablingPhaseCheck := func(genID int) {
			addonStatusPhaseCheck(genID, extensionsv1alpha1.AddonDisabling, nil)
		}

		checkAddonDeleted := func(g Gomega) {
			addon = &extensionsv1alpha1.Addon{}
			err := testCtx.Cli.Get(ctx, key, addon)
			g.Expect(err).To(HaveOccurred())
			g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		}

		checkAddonNotDeleted := func(g Gomega) {
			addon = &extensionsv1alpha1.Addon{}
			err := testCtx.Cli.Get(ctx, key, addon)
			g.Expect(err).To(Not(HaveOccurred()))
		}

		disableAddon := func(genID int) {
			addon = &extensionsv1alpha1.Addon{}
			Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
			addon.Spec.InstallSpec.Enabled = false
			Expect(testCtx.Cli.Update(ctx, addon)).Should(Succeed())
			disablingPhaseCheck(genID)
		}

		disableAddonFailedCheck := func(genID, obGenID int, expectPhase extensionsv1alpha1.AddonPhase) {
			addon = &extensionsv1alpha1.Addon{}
			Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
			addon.Spec.InstallSpec.Enabled = false
			Expect(testCtx.Cli.Update(ctx, addon)).Should(Succeed())

			Eventually(func(g Gomega) {
				_, err := doReconcile()
				Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Generation).Should(BeEquivalentTo(genID))
				g.Expect(addon.Spec.InstallSpec).ShouldNot(BeNil())
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(obGenID))
				g.Expect(addon.Status.Phase).Should(Equal(expectPhase))
			}).Should(Succeed())
		}

		fakeHelmRelease := func() {
			// create fake helm release
			helmRelease := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("sh.helm.release.v1.%s.v1", addon.Name),
					Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
					Labels: map[string]string{
						"owner": "helm",
						"name":  getHelmReleaseName(addon),
					},
				},
				Type: "helm.sh/release.v1",
			}
			Expect(testCtx.CreateObj(ctx, helmRelease)).Should(Succeed())
		}

		createTestNamespace := func(name string) {
			namespace := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}
			Expect(client.IgnoreAlreadyExists(testCtx.Create(ctx, &namespace))).To(Succeed())
		}

		createTestDeployment := func() {
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeblocks",
					Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
					Labels: map[string]string{
						constant.AppComponentLabelKey: "apps",
						constant.AppNameLabelKey:      "kubeblocks",
						constant.AppVersionLabelKey:   "0.9.0-beta.5",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constant.AppNameLabelKey: "kubeblocks",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								constant.AppNameLabelKey: "kubeblocks",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, deploy))).To(Succeed())
		}

		CheckKubernetesVersion := func() bool {
			var serverVersion *version.Info
			var isBeforeVersion bool
			Eventually(func(g Gomega) {
				clientset, err := kubernetes.NewForConfig(cfg)
				g.Expect(err).NotTo(HaveOccurred())
				discoveryClient := clientset.Discovery()
				serverVersion, err = discoveryClient.ServerVersion()
				g.Expect(err).NotTo(HaveOccurred())
				isBeforeVersion = (version.CompareKubeAwareVersionStrings(serverVersion.String(), "v1.23.0") > 0)
			}).Should(Succeed())
			fmt.Printf("Kubernetes server version: %s\n", serverVersion.String())
			return isBeforeVersion
		}

		It("should successfully reconcile a custom resource for Addon with spec.type=Helm", func() {
			By("By create an addon")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Type = extensionsv1alpha1.HelmType
				newOjb.Spec.Helm = &extensionsv1alpha1.HelmTypeInstallSpec{
					InstallOptions: extensionsv1alpha1.HelmInstallOptions{
						"--debug": "true",
					},
					ChartLocationURL: "file:///test-charts.tgz",
					ChartsImage:      "kubeblocks-charts",
				}
			})

			By("By checking status.observedGeneration and status.phase=disabled")
			Eventually(func(g Gomega) {
				doReconcileOnce(g)
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
			}).Should(Succeed())

			By("By enabling addon with default install")
			defaultInstall := addon.Spec.DefaultInstallValues[0].AddonInstallSpec
			addon.Spec.InstallSpec = defaultInstall.DeepCopy()
			addon.Spec.InstallSpec.Enabled = true
			Expect(testCtx.Cli.Update(ctx, addon)).Should(Succeed())
			enablingPhaseCheck(2)

			By("By enabled addon with fake completed installation job status")
			fakeInstallationCompletedJob(2)

			By("By checking init container")
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getInstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				fakeActiveJob(g, jobKey)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				job := getJob(g, jobKey)
				g.Expect(job.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			}).Should(Succeed())

			By("By disabling enabled addon")
			// create fake helm release
			fakeHelmRelease()
			disableAddon(3)

			By("By disabled an enabled addon with fake completed uninstall job status")
			uninstallJobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getUninstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				fakeCompletedJob(g, uninstallJobKey)
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(3))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
				checkedJobDeletion(g, uninstallJobKey)
			}).Should(Succeed())

			By("By delete addon with disabled status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
			}).Should(Succeed())
		})

		createAutoInstallAddon := func() {
			By("By create an addon with auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By addon autoInstall auto added")
			enablingPhaseCheck(2)
		}

		It("should successfully reconcile a custom resource for Addon with autoInstall=true", func() {
			createAutoInstallAddon()

			By("By enable addon with fake completed install job status")
			fakeInstallationCompletedJob(2)

			By("By delete addon with enabled status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonDeleted(g)
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon with autoInstall=true with failed uninstall job", func() {
			createAutoInstallAddon()

			By("By enable addon with fake completed install job status")
			fakeInstallationCompletedJob(2)

			By("By disabling enabled addon")
			fakeHelmRelease()
			disableAddon(3)

			By("By failed an uninstallation job")
			fakeUninstallationFailedJob(3)

			By("By delete addon with disabling status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonDeleted(g)
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon deletion while enabling", func() {
			By("By create an addon with auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By addon autoInstall auto added")
			enablingPhaseCheck(2)

			By("By delete addon with enabling status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getUninstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonDeleted(g)
				checkedJobDeletion(g, jobKey)
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon deletion while disabling", func() {
			createAutoInstallAddon()

			By("By enable addon with fake completed install job status")
			fakeInstallationCompletedJob(2)

			By("By disabling addon")
			disableAddon(3)

			By("By delete addon with disabling status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getUninstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonDeleted(g)
				checkedJobDeletion(g, jobKey)
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon with autoInstall=true with status.phase=Failed", func() {
			createAutoInstallAddon()

			By("By enabled addon with fake failed install job status")
			fakeInstallationFailedJob(2)

			By("By disabling addon with failed status")
			disableAddon(3)

			By("By delete addon with failed status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getUninstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonDeleted(g)
				checkedJobDeletion(g, jobKey)
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon run job with controller manager schedule settings", func() {
			viper.Set(constant.CfgKeyCtrlrMgrAffinity,
				"{\"nodeAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"preference\":{\"matchExpressions\":[{\"key\":\"kb-controller\",\"operator\":\"In\",\"values\":[\"true\"]}]},\"weight\":100}]}}")
			viper.Set(constant.CfgKeyCtrlrMgrTolerations,
				"[{\"key\":\"key1\", \"operator\": \"Exists\", \"effect\": \"NoSchedule\"}]")
			viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "{\"beta.kubernetes.io/arch\":\"amd64\"}")

			By("By create an addon with auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By addon autoInstall auto added")
			enablingPhaseCheck(2)

			By("By checking status.observedGeneration and status.phase=disabled")
			jobKey := client.ObjectKey{
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
				Name:      getInstallJobName(addon),
			}
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				job := &batchv1.Job{}
				g.Eventually(testCtx.Cli.Get(ctx, jobKey, job)).Should(Succeed())
				g.Expect(job.Spec.Template.Spec.Tolerations).ShouldNot(BeEmpty())
				g.Expect(job.Spec.Template.Spec.NodeSelector).ShouldNot(BeEmpty())
				g.Expect(job.Spec.Template.Spec.Affinity).ShouldNot(BeNil())
				g.Expect(job.Spec.Template.Spec.Affinity.NodeAffinity).ShouldNot(BeNil())
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon with no matching installable selector", func() {
			By("By create an addon with no matching installable selector")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.Selectors[0].Values = []string{"some-others"}
			})

			By("By checking status.observedGeneration and status.phase=disabled")
			Eventually(func(g Gomega) {
				doReconcileOnce(g)
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
				g.Expect(addon.Status.Conditions).ShouldNot(BeEmpty())
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(1))
			}).Should(Succeed())

			By("By addon with failed installable check")
			// "extensions.kubeblocks.io/skip-installable-check"
		})

		It("should failed reconcile a custom resource for Addon with missing spec helm of helm chartLocationURL", func() {
			By("By check Kubernetes Version to decide if check")
			// if k8s version > 1.23, it has kubebuilder X-Validation, no need to check again
			if CheckKubernetesVersion() {
				By("By create an addon with spec.helm = nil")
				createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
					newOjb.Spec.Installable.AutoInstall = true
					newOjb.Spec.Type = extensionsv1alpha1.HelmType
					newOjb.Spec.Helm = nil
				})

				By("By check addon status")
				Eventually(func(g Gomega) {
					doReconcileOnce(g)
					addon = &extensionsv1alpha1.Addon{}
					g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
					g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonFailed))
					g.Expect(addon.Spec.InstallSpec).Should(BeNil())
					g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}).Should(Succeed())
			}
		})

		It("should successfully reconcile a custom resource for Addon with CM and secret ref values", func() {
			By("By create an addon with spec.helm.installValues.configMapRefs set")
			cm := testapps.CreateCustomizedObj(&testCtx, "addon/cm-values.yaml",
				&corev1.ConfigMap{}, func(newCM *corev1.ConfigMap) {
					newCM.Namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
				})
			secret := testapps.CreateCustomizedObj(&testCtx, "addon/secret-values.yaml",
				&corev1.Secret{}, func(newSecret *corev1.Secret) {
					newSecret.Namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
				})

			By("By addon enabled via auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
				for k := range cm.Data {
					newOjb.Spec.Helm.InstallValues.ConfigMapRefs = append(newOjb.Spec.Helm.InstallValues.ConfigMapRefs,
						extensionsv1alpha1.DataObjectKeySelector{
							Name: cm.Name,
							Key:  k,
						})
				}
				for k := range secret.Data {
					newOjb.Spec.Helm.InstallValues.SecretRefs = append(newOjb.Spec.Helm.InstallValues.SecretRefs,
						extensionsv1alpha1.DataObjectKeySelector{
							Name: secret.Name,
							Key:  k,
						})
				}
			})
			enablingPhaseCheck(2)

			By("By enabled addon with fake completed install job status")
			fakeInstallationCompletedJob(2)
		})

		It("should failed reconcile a custom resource for Addon with missing CM ref values", func() {
			By("By create an addon with spec.helm.installValues.configMapRefs set")
			cm := testapps.CreateCustomizedObj(&testCtx, "addon/cm-values.yaml",
				&corev1.ConfigMap{}, func(newCM *corev1.ConfigMap) {
					newCM.Namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
				})

			By("By addon enabled via auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
				newOjb.Spec.Helm.InstallValues.ConfigMapRefs = append(newOjb.Spec.Helm.InstallValues.ConfigMapRefs,
					extensionsv1alpha1.DataObjectKeySelector{
						Name: cm.Name,
						Key:  "unknown",
					})
			})
			addonStatusPhaseCheck(2, extensionsv1alpha1.AddonFailed, nil)
		})

		It("should failed reconcile a custom resource for Addon with missing secret ref values", func() {
			By("By create an addon with spec.helm.installValues.configMapRefs set")
			secret := testapps.CreateCustomizedObj(&testCtx, "addon/secret-values.yaml",
				&corev1.Secret{}, func(newSecret *corev1.Secret) {
					newSecret.Namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
				})
			By("By addon enabled via auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
				newOjb.Spec.Helm.InstallValues.SecretRefs = append(newOjb.Spec.Helm.InstallValues.SecretRefs,
					extensionsv1alpha1.DataObjectKeySelector{
						Name: secret.Name,
						Key:  "unknown",
					})
			})
			addonStatusPhaseCheck(2, extensionsv1alpha1.AddonFailed, nil)
		})

		It("should set status to failed when install an Addon with annotations mismatching", func() {
			viper.Set(constant.CfgKeyCtrlrMgrNS, "kb-system")
			By("By create a new namespace called kb-system")
			createTestNamespace("kb-system")

			By("By create a deployment called kubeblocks with labels")
			createTestDeployment()

			By("By create an addon with annotation of KubeBlocks Version")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Installable.AutoInstall = true
				newOjb.Annotations = map[string]string{
					KBVersionValidate: ">=0.99.0",
				}
			})

			By("By checking status.observedGeneration and status.phase=failed")
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon := &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonFailed))
				g.Expect(addon.Status.Conditions).Should(HaveLen(1))
				g.Expect(addon.Status.Conditions[0].Reason).Should(BeEquivalentTo(InstallableRequirementUnmatched))
			}).Should(Succeed())
			viper.SetDefault(constant.CfgKeyCtrlrMgrNS, "default")
		})

		It("should set status to failed when enable an Addon with annotations mismatching", func() {
			viper.Set(constant.CfgKeyCtrlrMgrNS, "kb-system")
			By("By create a new namespace called kb-system")
			createTestNamespace("kb-system")

			By("By create a deployment called kubeblocks with labels")
			createTestDeployment()

			By("By create an addon")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Type = extensionsv1alpha1.HelmType
			})

			By("By checking status.observedGeneration and status.phase=disabled")
			Eventually(func(g Gomega) {
				doReconcileOnce(g)
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
			}).Should(Succeed())

			By("By set the constraint in annotations of an addon")
			defaultInstall := addon.Spec.DefaultInstallValues[0].AddonInstallSpec
			addon.Spec.InstallSpec = defaultInstall.DeepCopy()
			addon.Spec.InstallSpec.Enabled = true
			addon.Annotations = map[string]string{
				KBVersionValidate: ">=0.99.0",
			}
			Expect(testCtx.Cli.Update(ctx, addon)).Should(Succeed())

			By("By failed to enable an addon with disabled status and the constraint is not met")
			Eventually(func(g Gomega) {
				addon := &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				doReconcileOnce(g)
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonFailed))
				g.Expect(addon.Status.Conditions).Should(HaveLen(2))
				g.Expect(addon.Status.Conditions[1].Reason).Should(BeEquivalentTo(InstallableRequirementUnmatched))
			}).Should(Succeed())

			By("remove the constraint in annotations of an addon")
			addon := &extensionsv1alpha1.Addon{}
			Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
			addon.Annotations = map[string]string{}
			Expect(testCtx.Cli.Update(ctx, addon)).Should(Succeed())

			By("By enable an addon with status failed")
			Eventually(func(g Gomega) {
				addon := &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				enablingPhaseCheck(2)
			}).Should(Succeed())
			viper.SetDefault(constant.CfgKeyCtrlrMgrNS, "default")
		})

		It("should successfully reconcile a custom resource for Addon deleting with used by clusters", func() {
			createAutoInstallAddon()

			By("By enable addon with fake completed install job status")
			fakeInstallationCompletedJob(2)

			By("By creating cluster with addon")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddComponent(addon.Name, addon.Name).SetReplicas(1).
				WithRandomName().
				AddLabels(constant.ClusterDefLabelKey, addon.Name).
				Create(&testCtx).
				GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			cluster := &kbappsv1.Cluster{}
			Eventually(func(g Gomega) {
				err := testCtx.Cli.Get(ctx, clusterKey, cluster)
				g.Expect(err).To(Not(HaveOccurred()))
			}).Should(Succeed())

			By("By delete addon with disabling status")
			Expect(testCtx.Cli.Delete(ctx, addon)).To(Not(HaveOccurred()))
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonNotDeleted(g)
			}).Should(Succeed())

			By("By deleting cluster")
			Expect(testCtx.Cli.Delete(ctx, cluster)).To(Not(HaveOccurred()))

			By("By checking addon with no used cluster")
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				checkAddonDeleted(g)
			}).Should(Succeed())
		})

		It("should successfully reconcile a custom resource for Addon disabling with used by clusters", func() {
			createAutoInstallAddon()

			By("By enable addon with fake completed install job status")
			fakeInstallationCompletedJob(2)

			By("By creating cluster with addon")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddComponent(addon.Name, addon.Name).SetReplicas(1).
				WithRandomName().
				AddLabels(constant.ClusterDefLabelKey, addon.Name).
				Create(&testCtx).
				GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			cluster := &kbappsv1.Cluster{}
			Eventually(func(g Gomega) {
				err := testCtx.Cli.Get(ctx, clusterKey, cluster)
				g.Expect(err).To(Not(HaveOccurred()))
			}).Should(Succeed())

			By("By disabling enabled addon")
			fakeHelmRelease()
			disableAddonFailedCheck(3, 2, extensionsv1alpha1.AddonEnabled)

			By("By deleting cluster")
			Expect(testCtx.Cli.Delete(ctx, cluster)).To(Not(HaveOccurred()))

			By("By checking addon with no used cluster")
			disablingPhaseCheck(3)
		})

		It("should have provider and version in spec for Addon with provider and version in label initially", func() {
			By("By create an addon with auto-install and labels contains provider and version information")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Labels = map[string]string{
					AddonProvider: "apecloud",
					AddonVersion:  "0.9.0",
				}
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By setting the provider and version in spec for Addon")
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Spec.Provider).Should(Equal("apecloud"))
				g.Expect(addon.Spec.Version).Should(Equal("0.9.0"))
			}).Should(Succeed())
		})

		It("should have provider and version in spec for Addon after creating, before enabled", func() {
			By("By create an addon with auto-install and labels contains provider and version information")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Labels = map[string]string{
					AddonProvider: "apecloud",
					AddonVersion:  "0.9.0",
				}
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By Checking spec.provider and spec.version of the addon after one reconcile, before enabled")
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Spec.Provider).Should(Equal("apecloud"))
				g.Expect(addon.Spec.Version).Should(Equal("0.9.0"))
			}).Should(Succeed())
		})

		It("should have provider and version in labels for Addon with provider and version in spec after creating, before enabled", func() {
			By("By create an addon with auto-install and labels contains provider and version information")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.Addon) {
				newOjb.Spec.Provider = "apecloud"
				newOjb.Spec.Version = "0.9.0"
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By Checking spec.provider and spec.version of the addon after one reconcile, before enabled")
			Eventually(func(g Gomega) {
				_, err := doReconcile()
				g.Expect(err).To(Not(HaveOccurred()))
				addon = &extensionsv1alpha1.Addon{}
				g.Expect(testCtx.Cli.Get(ctx, key, addon)).To(Not(HaveOccurred()))
				g.Expect(addon.Labels).Should(HaveKeyWithValue(AddonProvider, "apecloud"))
				g.Expect(addon.Labels).Should(HaveKeyWithValue(AddonVersion, "0.9.0"))
				g.Expect(addon.Status.Phase).Should(Equal(extensionsv1alpha1.AddonEnabling))
			}).Should(Succeed())
		})
	})

	Context("Addon controller SetupWithManager", func() {
		It("Do controller SetupWithManager init. flow", func() {
			By("check SetupWithManager")
			k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  scheme.Scheme,
				Metrics: server.Options{BindAddress: "0"},
			})
			Expect(err).ToNot(HaveOccurred())
			reconciler := &AddonReconciler{
				Client: k8sManager.GetClient(),
				Scheme: k8sManager.GetScheme(),
			}
			err = reconciler.SetupWithManager(k8sManager)
			Expect(err).ToNot(HaveOccurred())

			testEventRecorder := func(recorder record.EventRecorder) {
				reconciler.Recorder = recorder
				addon := &extensionsv1alpha1.Addon{}
				reconciler.Event(addon, corev1.EventTypeNormal, "reason", "message")
				reconciler.Eventf(addon, corev1.EventTypeNormal, "reason", "%s", "message")
				reconciler.AnnotatedEventf(addon, map[string]string{
					"key": "value",
				}, corev1.EventTypeNormal, "reason", "message")
			}

			By("test nil event recorder")
			testEventRecorder(nil)

			By("test event recorder")
			testEventRecorder(k8sManager.GetEventRecorderFor("addon-controller"))

			By("start manager")
			go func() {
				defer GinkgoRecover()
				timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second)
				Expect(k8sManager.Start(timeoutCtx)).ToNot(HaveOccurred(), "failed to run manager")
				if cancelFunc != nil {
					cancelFunc()
				}
			}()

			createJob := func(keys ...string) *batchv1.Job {
				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "addon-test-job",
						Namespace:    testCtx.DefaultNamespace,
						Labels:       map[string]string{},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "kubeblocks",
										Image: "busybox",
									},
								},
							},
						},
					},
				}
				for _, key := range keys {
					job.ObjectMeta.Labels[key] = constant.AppName
				}
				return job
			}

			By("create watch obj type without required labels")
			job := createJob()
			Expect(k8sClient.Create(ctx, job)).ToNot(HaveOccurred())

			By("create watch obj with label=" + constant.AddonNameLabelKey)
			job = createJob(constant.AddonNameLabelKey)
			Expect(k8sClient.Create(ctx, job)).ToNot(HaveOccurred())

			By("create watch obj with label=" + constant.AppManagedByLabelKey)
			job = createJob(constant.AppManagedByLabelKey)
			Expect(k8sClient.Create(ctx, job)).ToNot(HaveOccurred())

			By("create watch obj with required labels")
			job = createJob(constant.AddonNameLabelKey, constant.AppManagedByLabelKey)
			Expect(k8sClient.Create(ctx, job)).ToNot(HaveOccurred())
		})
	})
})

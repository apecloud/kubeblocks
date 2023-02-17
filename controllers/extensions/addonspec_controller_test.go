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

package extensions

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Addon controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Second
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		inNS := client.InNamespace(viper.GetString("CM_NAMESPACE"))
		testapps.ClearResources(&testCtx, intctrlutil.JobSignature, inNS,
			client.HasLabels{
				intctrlutil.AddonNameLabelKey,
			})
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.SecretSignature, inNS, ml)

		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.AddonSpecSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("Addon controller test", func() {
		var addonSpec *extensionsv1alpha1.AddonSpec
		var key types.NamespacedName
		BeforeEach(func() {
			const distro = "kubeblocks"
			testutil.SetKubeServerVersionWithDistro("1", "24", "0", distro)
		})

		AfterEach(func() {
			cleanEnv()
		})

		fakeCompletedJob := func(g Gomega, jobKey client.ObjectKey) {
			job := &batchv1.Job{}
			g.Eventually(testCtx.Cli.Get(ctx, jobKey, job)).Should(Succeed())
			job.Status.Succeeded = 1
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

		fakeIntallationCompletedJob := func(expectedObservedGeneration int) {
			jobKey := client.ObjectKey{
				Namespace: viper.GetString("CM_NAMESPACE"),
				Name:      getInstallJobName(addonSpec),
			}
			Eventually(func(g Gomega) {
				fakeCompletedJob(g, jobKey)
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				addonSpec = &extensionsv1alpha1.AddonSpec{}
				g.Expect(testCtx.Cli.Get(ctx, key, addonSpec)).Should(Succeed())
				g.Expect(addonSpec.Status.Phase).Should(Equal(extensionsv1alpha1.AddonEnabled))
				g.Expect(addonSpec.Status.ObservedGeneration).Should(BeEquivalentTo(expectedObservedGeneration))
				checkedJobDeletion(g, jobKey)
			}, timeout, interval).Should(Succeed())
		}

		createAddonSpecWithRequiredAttributes := func(modifiers func(newOjb *extensionsv1alpha1.AddonSpec)) {
			if modifiers != nil {
				addonSpec = testapps.CreateCustomizedObj(&testCtx, "addonspec/addonspec.yaml",
					&extensionsv1alpha1.AddonSpec{}, modifiers)
			} else {
				addonSpec = testapps.CreateCustomizedObj(&testCtx, "addonspec/addonspec.yaml",
					&extensionsv1alpha1.AddonSpec{})
			}
			key = types.NamespacedName{
				Name: addonSpec.Name,
			}
			Expect(addonSpec.Spec.DefaultInstallValues).ShouldNot(BeEmpty())
		}

		enablingPhaseCheck := func(genID int) {
			Eventually(func(g Gomega) {
				addonSpec = &extensionsv1alpha1.AddonSpec{}
				g.Expect(testCtx.Cli.Get(ctx, key, addonSpec)).Should(Succeed())
				g.Expect(addonSpec.Generation).Should(BeEquivalentTo(genID))
				g.Expect(addonSpec.Spec.InstallSpec).ShouldNot(BeNil())
				g.Expect(addonSpec.Status.ObservedGeneration).Should(BeEquivalentTo(genID))
				g.Expect(addonSpec.Status.Phase).Should(Equal(extensionsv1alpha1.AddonEnabling))
			}, timeout, interval).Should(Succeed())
		}

		It("should successfully reconcile a custom resource for AddonSpec", func() {
			By("By create an addonSpec")
			createAddonSpecWithRequiredAttributes(nil)

			By("By checking status.observedGeneration and status.phase=disabled")
			Eventually(func(g Gomega) {
				addonSpec = &extensionsv1alpha1.AddonSpec{}
				g.Expect(testCtx.Cli.Get(ctx, key, addonSpec)).Should(Succeed())
				g.Expect(addonSpec.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				g.Expect(addonSpec.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
			}, timeout, interval).Should(Succeed())

			By("By enabling addon with default install")
			defaultInstall := addonSpec.Spec.DefaultInstallValues[0].AddonInstallSpec
			addonSpec.Spec.InstallSpec = &defaultInstall
			Expect(testCtx.Cli.Update(ctx, addonSpec)).Should(Succeed())
			enablingPhaseCheck(2)

			By("By enabled addon with fake completed install job status")
			fakeIntallationCompletedJob(2)

			By("By disabling enabled addon")
			// create fake helm release
			helmRelease := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("sh.helm.release.v1.%s.v1", addonSpec.Name),
					Namespace: viper.GetString("CM_NAMESPACE"),
					Labels: map[string]string{
						"owner": "helm",
						"name":  getHelmReleaseName(addonSpec),
					},
				},
				Type: "helm.sh/release.v1",
			}
			Expect(testCtx.Cli.Create(ctx, helmRelease)).Should(Succeed())
			addonSpec.Spec.InstallSpec = nil
			Expect(testCtx.Cli.Update(ctx, addonSpec)).Should(Succeed())
			Eventually(func(g Gomega) {
				addonSpec = &extensionsv1alpha1.AddonSpec{}
				g.Expect(testCtx.Cli.Get(ctx, key, addonSpec)).Should(Succeed())
				g.Expect(addonSpec.Status.ObservedGeneration).Should(BeEquivalentTo(3))
				//g.Expect(addonSpec.Status.Phase).Should(BeElementOf(
				//	[]extensionsv1alpha1.AddonPhase{
				//		extensionsv1alpha1.AddonDisabling,
				//		extensionsv1alpha1.AddonDisabled,
				//	}))
				g.Expect(addonSpec.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabling))
			}, timeout, interval).Should(Succeed())

			By("By disabled enabled addon with fake completed uninstall job status")
			jobKey := client.ObjectKey{
				Namespace: viper.GetString("CM_NAMESPACE"),
				Name:      getUninstallJobName(addonSpec),
			}
			Eventually(func(g Gomega) {
				fakeCompletedJob(g, jobKey)
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				addonSpec = &extensionsv1alpha1.AddonSpec{}
				g.Expect(testCtx.Cli.Get(ctx, key, addonSpec)).Should(Succeed())
				g.Expect(addonSpec.Status.ObservedGeneration).Should(BeEquivalentTo(3))
				g.Expect(addonSpec.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
				checkedJobDeletion(g, jobKey)
			}, timeout, interval).Should(Succeed())

		})

		It("should successfully reconcile a custom resource for AddonSpec with autoInstall=true", func() {
			By("By create an addonSpec with auto-install")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.AddonSpec) {
				newOjb.Spec.Installable.AutoInstall = true
			})

			By("By addon autoInstall auto added")
			enablingPhaseCheck(2)

			By("By enabled addon with fake completed install job status")
			fakeIntallationCompletedJob(2)
		})

		It("should successfully reconcile a custom resource for AddonSpec with no matching installable selector", func() {
			By("By create an addonSpec with no matching installable selector")
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.AddonSpec) {
				newOjb.Spec.Installable.Selectors[0].Values = []string{"some-others"}
			})

			By("By checking status.observedGeneration and status.phase=disabled")
			Eventually(func(g Gomega) {
				addonSpec = &extensionsv1alpha1.AddonSpec{}
				g.Expect(testCtx.Cli.Get(ctx, key, addonSpec)).Should(Succeed())
				g.Expect(addonSpec.Status.Phase).Should(Equal(extensionsv1alpha1.AddonDisabled))
				g.Expect(addonSpec.Status.Conditions).ShouldNot(BeEmpty())
				g.Expect(addonSpec.Status.ObservedGeneration).Should(BeEquivalentTo(1))
			}, timeout, interval).Should(Succeed())

			By("By addon with failed installable check")
			// "extensions.kubeblocks.io/skip-installable-check"
		})

		It("should successfully reconcile a custom resource for AddonSpec with CM and secret values", func() {
			By("By create an addonSpec with spec.helm.installValues.configMapRefs set")
			cm := testapps.CreateCustomizedObj(&testCtx, "addonspec/cm-values.yaml",
				&corev1.ConfigMap{}, func(newCM *corev1.ConfigMap) {
					newCM.Namespace = viper.GetString("CM_NAMESPACE")
				})
			secret := testapps.CreateCustomizedObj(&testCtx, "addonspec/secret-values.yaml",
				&corev1.Secret{}, func(newSecret *corev1.Secret) {
					newSecret.Namespace = viper.GetString("CM_NAMESPACE")
				})
			createAddonSpecWithRequiredAttributes(func(newOjb *extensionsv1alpha1.AddonSpec) {
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

			By("By addon autoInstall auto added")
			enablingPhaseCheck(2)

			By("By enabled addon with fake completed install job status")
			fakeIntallationCompletedJob(2)
		})
	})
})

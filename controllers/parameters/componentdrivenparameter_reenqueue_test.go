/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package parameters

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

// These tests target the ComponentDrivenParameter re-enqueue contract:
// when a Component is created while no matching ParametersDefinition is yet
// Available, and the PD subsequently becomes Available, the controller must
// eventually reconcile the Component and create its ComponentParameter.
//
// The forward It below is expected to fail on a build that only Watches
// `appsv1.Component` because no event re-enqueues the Component when the PD
// becomes Available. After adding a PD->Component Watch with an
// EnqueueRequestsFromMapFunc, this It should pass deterministically.
//
// The regression-guard It exercises the long-standing path where a Component
// update itself re-enqueues the Component; this path must continue to work
// after the watch is added (i.e. the watch addition must be additive only).
var _ = Describe("ComponentDrivenParameter re-enqueue contract", func() {
	const (
		probeAnnotationKey   = "kubeblocks.io/reenqueue-regression-guard"
		probeAnnotationValue = "true"
	)

	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	It("should reconcile Component after a matching ParametersDefinition becomes Available", func() {
		// Step 1: configmap + ComponentDefinition (Available) + one config slot.
		// No matching ParametersDefinition is created yet.
		configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
			AddLabels(
				constant.AppNameLabelKey, clusterName,
				constant.AppInstanceLabelKey, clusterName,
				constant.KBAppComponentLabelKey, defaultCompName,
				constant.CMConfigurationTemplateNameLabelKey, configSpecName,
				constant.CMConfigurationConstraintsNameLabelKey, cmName,
				constant.CMConfigurationSpecProviderLabelKey, configSpecName,
				constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
			).
			AddAnnotations(constant.ConfigurationRevision, "1").
			Create(&testCtx).
			GetObject()

		compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())

		// Step 2: create the Component referencing the Available CompDef.
		fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
		_ = testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetReplicas(1).
			Create(&testCtx).
			GetObject()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}

		// Step 3: setup confirmation — within a short window, ComponentParameter
		// must NOT yet exist because no matching PD is Available.
		Consistently(func() bool {
			cp := &parametersv1alpha1.ComponentParameter{}
			err := testCtx.Cli.Get(testCtx.Ctx, cfgKey, cp)
			return apierrors.IsNotFound(err)
		}, 3*time.Second, 200*time.Millisecond).Should(BeTrue(),
			"Step 3 setup confirmation: ComponentParameter should not exist before a matching PD becomes Available")

		// Step 4: create the matching ParametersDefinition and patch it to Available.
		paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
			Schema(mockSchemaData()).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Spec.ComponentDef = compDefObj.Name
			obj.Spec.TemplateName = configSpecName
			obj.Spec.FileFormatConfig = &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
				FormatterAction: parametersv1alpha1.FormatterAction{
					IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
				},
			}
		})()).Should(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		// Step 5: contract assertion — within a bounded budget after PD becomes
		// Available, ComponentParameter must be created. On a build that does
		// not Watch ParametersDefinition this Eventually fails (re-enqueue gap);
		// on the fix it must pass.
		Eventually(func() error {
			cp := &parametersv1alpha1.ComponentParameter{}
			return testCtx.Cli.Get(testCtx.Ctx, cfgKey, cp)
		}, 15*time.Second, 200*time.Millisecond).Should(Succeed(),
			"Step 5 forward contract: ComponentParameter must be created within 15s after the matching ParametersDefinition becomes Available")
	})

	It("should reconcile Components affected by both sides of a ParametersDefinition pattern change", func() {
		configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
			AddLabels(
				constant.AppNameLabelKey, clusterName,
				constant.AppInstanceLabelKey, clusterName,
				constant.KBAppComponentLabelKey, defaultCompName,
				constant.CMConfigurationTemplateNameLabelKey, configSpecName,
				constant.CMConfigurationConstraintsNameLabelKey, cmName,
				constant.CMConfigurationSpecProviderLabelKey, configSpecName,
				constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
			).
			AddAnnotations(constant.ConfigurationRevision, "1").
			Create(&testCtx).
			GetObject()

		compDefA := testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefA), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())

		compDefB := testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefB), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())

		compNameA := defaultCompName
		compNameB := defaultCompName + "-alt"
		testapps.NewComponentFactory(testCtx.DefaultNamespace, constant.GenerateClusterComponentName(clusterName, compNameA), compDefA.Name).
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetReplicas(1).
			Create(&testCtx)
		testapps.NewComponentFactory(testCtx.DefaultNamespace, constant.GenerateClusterComponentName(clusterName, compNameB), compDefB.Name).
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetReplicas(1).
			Create(&testCtx)

		cfgKeyA := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GenerateComponentConfigurationName(clusterName, compNameA),
		}
		cfgKeyB := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GenerateComponentConfigurationName(clusterName, compNameB),
		}

		paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
			Schema(mockSchemaData()).
			SetComponentDefinition(compDefA.Name).
			SetTemplateName(configSpecName).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		Eventually(func() error {
			cp := &parametersv1alpha1.ComponentParameter{}
			return testCtx.Cli.Get(testCtx.Ctx, cfgKeyA, cp)
		}, 15*time.Second, 200*time.Millisecond).Should(Succeed(),
			"setup: ComponentParameter for the original PD ComponentDef pattern should be created")
		Consistently(func() bool {
			cp := &parametersv1alpha1.ComponentParameter{}
			err := testCtx.Cli.Get(testCtx.Ctx, cfgKeyB, cp)
			return apierrors.IsNotFound(err)
		}, 3*time.Second, 200*time.Millisecond).Should(BeTrue(),
			"setup: ComponentParameter for the new PD ComponentDef pattern should not exist before the pattern changes")

		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Spec.ComponentDef = compDefB.Name
		})()).Should(Succeed())

		Eventually(func() bool {
			cp := &parametersv1alpha1.ComponentParameter{}
			err := testCtx.Cli.Get(testCtx.Ctx, cfgKeyA, cp)
			return apierrors.IsNotFound(err)
		}, 15*time.Second, 200*time.Millisecond).Should(BeTrue(),
			"the old ComponentDef pattern must be re-enqueued so stale ComponentParameter is deleted")
		Eventually(func() error {
			cp := &parametersv1alpha1.ComponentParameter{}
			return testCtx.Cli.Get(testCtx.Ctx, cfgKeyB, cp)
		}, 15*time.Second, 200*time.Millisecond).Should(Succeed(),
			"the new ComponentDef pattern must be re-enqueued so ComponentParameter is created")
	})

	It("regression guard: a Component update event still re-enqueues reconcile to create ComponentParameter", func() {
		// Step 1: configmap + ComponentDefinition (Available) + one config slot.
		// As in the forward It, no PD yet.
		configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
			AddLabels(
				constant.AppNameLabelKey, clusterName,
				constant.AppInstanceLabelKey, clusterName,
				constant.KBAppComponentLabelKey, defaultCompName,
				constant.CMConfigurationTemplateNameLabelKey, configSpecName,
				constant.CMConfigurationConstraintsNameLabelKey, cmName,
				constant.CMConfigurationSpecProviderLabelKey, configSpecName,
				constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
			).
			AddAnnotations(constant.ConfigurationRevision, "1").
			Create(&testCtx).
			GetObject()

		compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())

		fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
		compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetReplicas(1).
			Create(&testCtx).
			GetObject()

		cfgKey := client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}

		// Confirm the initial silent-nil path: no ComponentParameter while no PD.
		Consistently(func() bool {
			cp := &parametersv1alpha1.ComponentParameter{}
			err := testCtx.Cli.Get(testCtx.Ctx, cfgKey, cp)
			return apierrors.IsNotFound(err)
		}, 3*time.Second, 200*time.Millisecond).Should(BeTrue(),
			"regression guard setup: ComponentParameter should not exist before a matching PD becomes Available")

		// Create matching PD and patch Available — same as the forward It.
		paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
			Schema(mockSchemaData()).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Spec.ComponentDef = compDefObj.Name
			obj.Spec.TemplateName = configSpecName
			obj.Spec.FileFormatConfig = &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
				FormatterAction: parametersv1alpha1.FormatterAction{
					IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
				},
			}
		})()).Should(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		// Touch the Component to force a Component update event. This must still
		// re-enqueue reconcile and create ComponentParameter regardless of any
		// later changes to the PD watch contract. This guards against a fix
		// that accidentally bypasses or removes the original Component-event
		// re-enqueue path.
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compObj), func(c *appsv1.Component) {
			if c.Annotations == nil {
				c.Annotations = map[string]string{}
			}
			c.Annotations[probeAnnotationKey] = probeAnnotationValue
		})()).Should(Succeed())

		Eventually(func() error {
			cp := &parametersv1alpha1.ComponentParameter{}
			return testCtx.Cli.Get(testCtx.Ctx, cfgKey, cp)
		}, 15*time.Second, 200*time.Millisecond).Should(Succeed(),
			"regression guard: ComponentParameter must be created within 15s after a Component update event even if the PD watch is not yet in place")
	})
})

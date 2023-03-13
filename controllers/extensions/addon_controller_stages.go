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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func init() {
	viper.SetDefault(addonSANameKey, "kubeblocks-addon-installer")
}

type fetchNDeletionCheckStage struct {
	stageCtx
	deletionStage deletionStage
}

type deletionStage struct {
	stageCtx
	disablingStage disablingStage
}

type genIDProceedCheckStage struct {
	stageCtx
}

type installableCheckStage struct {
	stageCtx
}

type autoInstallCheckStage struct {
	stageCtx
}

type progressingHandler struct {
	stageCtx
	enablingStage  enablingStage
	disablingStage disablingStage
}

type helmTypeInstallStage struct {
	stageCtx
}

type helmTypeUninstallStage struct {
	stageCtx
}

type enablingStage struct {
	stageCtx
	helmTypeInstallStage helmTypeInstallStage
}

type disablingStage struct {
	stageCtx
	helmTypeUninstallStage helmTypeUninstallStage
}

type terminalStateStage struct {
	stageCtx
}

func (r *fetchNDeletionCheckStage) Handle(ctx context.Context) {
	addon := &extensionsv1alpha1.Addon{}
	if err := r.reconciler.Client.Get(ctx, r.reqCtx.Req.NamespacedName, addon); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, r.reqCtx.Log, "")
		r.updateResultNErr(&res, err)
		return
	}
	r.reqCtx.Log.V(1).Info("get addon", "generation", addon.Generation, "observedGeneration", addon.Status.ObservedGeneration)
	r.reqCtx.UpdateCtxValue(operandValueKey, addon)
	res, err := intctrlutil.HandleCRDeletion(*r.reqCtx, r.reconciler, addon, addonFinalizerName, func() (*ctrl.Result, error) {
		r.deletionStage.Handle(ctx)
		return r.deletionStage.doReturn()
	})
	if res != nil || err != nil {
		r.updateResultNErr(res, err)
		return
	}
	r.reqCtx.Log.V(1).Info("start normal reconcile")
	r.next.Handle(ctx)
}

func (r *deletionStage) Handle(ctx context.Context) {
	r.disablingStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("deletionStage", "phase", addon.Status.Phase)
		patchPhase := func(phase extensionsv1alpha1.AddonPhase, reason string) {
			r.reqCtx.Log.V(1).Info("patching status", "phase", phase)
			patch := client.MergeFrom(addon.DeepCopy())
			addon.Status.Phase = phase
			addon.Status.ObservedGeneration = addon.Generation
			if err := r.reconciler.Status().Patch(ctx, addon, patch); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reqCtx.Log.V(1).Info("progress to", "phase", phase)
			r.reconciler.Recorder.Event(addon, "Normal", reason,
				fmt.Sprintf("Progress to %s phase", phase))
			r.setReconciled()
		}
		switch addon.Status.Phase {
		case extensionsv1alpha1.AddonEnabling:
			// delete running jobs
			res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
			if err != nil {
				r.updateResultNErr(res, err)
				return
			}
			patchPhase(extensionsv1alpha1.AddonDisabling, DisablingAddon)
			return
		case extensionsv1alpha1.AddonEnabled:
			patchPhase(extensionsv1alpha1.AddonDisabling, DisablingAddon)
			return
		case extensionsv1alpha1.AddonDisabling:
			r.disablingStage.Handle(ctx)
			res, err := r.disablingStage.doReturn()

			if res != nil || err != nil {
				return
			}
			patchPhase(extensionsv1alpha1.AddonDisabled, AddonDisabled)
			return
		default:
			r.reqCtx.Log.V(1).Info("delete external resources", "phase", addon.Status.Phase)
			res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
			if res != nil || err != nil {
				r.updateResultNErr(res, err)
				return
			}
			return
		}
	})
	r.next.Handle(ctx)
}

func (r *genIDProceedCheckStage) Handle(ctx context.Context) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("genIDProceedCheckStage", "phase", addon.Status.Phase)
		switch addon.Status.Phase {
		case extensionsv1alpha1.AddonEnabled, extensionsv1alpha1.AddonDisabled:
			if addon.Generation == addon.Status.ObservedGeneration {
				res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
				if res != nil || err != nil {
					r.updateResultNErr(res, err)
					return
				}
				r.setReconciled()
			}
		case extensionsv1alpha1.AddonFailed:
			if addon.Generation == addon.Status.ObservedGeneration {
				r.setReconciled()
				return
			}
		}
	})
	r.next.Handle(ctx)
}

func (r *installableCheckStage) Handle(ctx context.Context) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("installableCheckStage", "phase", addon.Status.Phase)
		if addon.Spec.Installable == nil {
			return
		}
		// proceed if has specified addon.spec.installSpec
		if addon.Spec.InstallSpec != nil {
			return
		}
		if addon.Annotations != nil && addon.Annotations[SkipInstallableCheck] == "true" {
			r.reconciler.Recorder.Event(addon, "Warning", InstallableCheckSkipped,
				"Installable check skipped.")
			return
		}
		switch addon.Status.Phase {
		case extensionsv1alpha1.AddonEnabling, extensionsv1alpha1.AddonDisabling:
			return
		}
		for _, s := range addon.Spec.Installable.Selectors {
			if s.MatchesFromConfig() {
				continue
			}
			patch := client.MergeFrom(addon.DeepCopy())
			addon.Status.ObservedGeneration = addon.Generation
			addon.Status.Phase = extensionsv1alpha1.AddonDisabled
			meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
				Type:               extensionsv1alpha1.ConditionTypeChecked,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: addon.Generation,
				Reason:             AddonSpecInstallableReqUnmatched,
				Message:            "spec.installable.selectors has no matching requirement.",
				LastTransitionTime: metav1.Now(),
			})

			if err := r.reconciler.Status().Patch(ctx, addon, patch); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reconciler.Recorder.Event(addon, "Warning", InstallableRequirementUnmatched,
				fmt.Sprintf("Does not meet installable requirements for key %v", s))
			r.setReconciled()
			return
		}
	})
	r.next.Handle(ctx)
}

func (r *autoInstallCheckStage) Handle(ctx context.Context) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("autoInstallCheckStage", "phase", addon.Status.Phase)
		if addon.Spec.Installable == nil || !addon.Spec.Installable.AutoInstall {
			return
		}
		// proceed if has specified addon.spec.installSpec
		if addon.Spec.InstallSpec != nil {
			r.reqCtx.Log.V(1).Info("has specified addon.spec.installSpec")
			return
		}

		setInstallSpec := func(di *extensionsv1alpha1.AddonDefaultInstallSpecItem) {
			addon.Spec.InstallSpec = di.AddonInstallSpec.DeepCopy()
			addon.Spec.InstallSpec.Enabled = true
			if err := r.reconciler.Client.Update(ctx, addon); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reconciler.Recorder.Event(addon, "Normal", AddonAutoInstall,
				"Addon enabled auto-install")
			r.setReconciled()
		}

		for _, di := range addon.Spec.GetSortedDefaultInstallValues() {
			if len(di.Selectors) == 0 {
				setInstallSpec(&di)
				return
			}
			for _, s := range di.Selectors {
				if !s.MatchesFromConfig() {
					continue
				}
				setInstallSpec(&di)
				return
			}
		}
	})
	r.next.Handle(ctx)
}

func (r *progressingHandler) Handle(ctx context.Context) {
	r.enablingStage.stageCtx = r.stageCtx
	r.disablingStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("progressingHandler", "phase", addon.Status.Phase)
		patchPhase := func(phase extensionsv1alpha1.AddonPhase, reason string) {
			r.reqCtx.Log.V(1).Info("patching status", "phase", phase)
			patch := client.MergeFrom(addon.DeepCopy())
			addon.Status.Phase = phase
			addon.Status.ObservedGeneration = addon.Generation
			if err := r.reconciler.Status().Patch(ctx, addon, patch); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reconciler.Recorder.Event(addon, "Normal", reason,
				fmt.Sprintf("Progress to %s phase", phase))
			r.setReconciled()
		}

		// decision enabling or disabling
		if !addon.Spec.InstallSpec.GetEnabled() {
			r.reqCtx.Log.V(1).Info("progress to disabling stage handler")
			// if it's new simply return
			if addon.Status.Phase == "" {
				return
			}
			if addon.Status.Phase != extensionsv1alpha1.AddonDisabling {
				patchPhase(extensionsv1alpha1.AddonDisabling, DisablingAddon)
				return
			}
			r.disablingStage.Handle(ctx)
			return
		}
		// handling enabling state
		if addon.Status.Phase != extensionsv1alpha1.AddonEnabling {
			if addon.Status.Phase == extensionsv1alpha1.AddonFailed {
				// clean up existing failed installation job
				mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
				key := client.ObjectKey{
					Namespace: mgrNS,
					Name:      getInstallJobName(addon),
				}
				installJob := &batchv1.Job{}
				if err := r.reconciler.Get(ctx, key, installJob); client.IgnoreNotFound(err) != nil {
					r.setRequeueWithErr(err, "")
					return
				} else if err == nil && installJob.GetDeletionTimestamp().IsZero() {
					if err = r.reconciler.Delete(ctx, installJob); err != nil {
						r.setRequeueWithErr(err, "")
						return
					}
				}
			}
			patchPhase(extensionsv1alpha1.AddonEnabling, EnablingAddon)
			return
		}
		r.reqCtx.Log.V(1).Info("progress to enabling stage handler")
		r.enablingStage.Handle(ctx)
	})
	r.next.Handle(ctx)
}

func getInstallJobName(addon *extensionsv1alpha1.Addon) string {
	return fmt.Sprintf("install-%s-addon", addon.Name)
}

func getUninstallJobName(addon *extensionsv1alpha1.Addon) string {
	return fmt.Sprintf("uninstall-%s-addon", addon.Name)
}

func getHelmReleaseName(addon *extensionsv1alpha1.Addon) string {
	return fmt.Sprintf("kb-addon-%s", addon.Name)
}

func (r *helmTypeInstallStage) Handle(ctx context.Context) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("helmTypeInstallStage", "phase", addon.Status.Phase)
		mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)

		key := client.ObjectKey{
			Namespace: mgrNS,
			Name:      getInstallJobName(addon),
		}

		helmInstallJob := &batchv1.Job{}
		if err := r.reconciler.Get(ctx, key, helmInstallJob); client.IgnoreNotFound(err) != nil {
			r.setRequeueWithErr(err, "")
			return
		} else if err == nil {
			if helmInstallJob.Status.Succeeded > 0 {
				return
			}

			if helmInstallJob.Status.Active > 0 {
				r.setRequeueAfter(time.Second, fmt.Sprintf("running Helm install job %s", key.Name))
				return
			}
			// there are situations that job.status.[Active | Failed | Succeeded ] are all
			// 0, and len(job.status.conditions) > 0, and need to handle failed
			// info. from conditions.
			if helmInstallJob.Status.Failed > 0 {
				// job failed set terminal state phase
				patch := client.MergeFrom(addon.DeepCopy())
				addon.Status.ObservedGeneration = addon.Generation
				addon.Status.Phase = extensionsv1alpha1.AddonFailed
				meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
					Type:               extensionsv1alpha1.ConditionTypeFailed,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: addon.Generation,
					Reason:             AddonSpecInstallFailed,
					Message:            "installation failed",
					LastTransitionTime: metav1.Now(),
				})

				if err := r.reconciler.Status().Patch(ctx, addon, patch); err != nil {
					r.setRequeueWithErr(err, "")
					return
				}
				r.reconciler.Recorder.Event(addon, "Warning", InstallationFailed,
					fmt.Sprintf("Installation failed, do inspect error from jobs.batch %s", key.String()))
				r.setReconciled()
				return
			}
			r.setRequeueAfter(time.Second, "")
			return
		}
		var err error
		helmInstallJob, err = createHelmJobProto(addon)
		if err != nil {
			r.setRequeueWithErr(err, "")
			return
		}
		helmInstallJob.ObjectMeta.Name = key.Name
		helmInstallJob.ObjectMeta.Namespace = key.Namespace
		helmJobPodSpec := &helmInstallJob.Spec.Template.Spec
		helmContainer := &helmInstallJob.Spec.Template.Spec.Containers[0]
		helmContainer.Args = []string{
			"upgrade",
			"--install",
			"$(RELEASE_NAME)",
			"$(CHART)",
			"--namespace",
			"$(RELEASE_NS)",
			"--timeout",
			"10m",
			"--create-namespace",
			"--atomic",
			"--cleanup-on-fail",
			"--wait",
		}

		// add extra helm install option flags
		for k, v := range addon.Spec.Helm.InstallOptions {
			helmContainer.Args = append(helmContainer.Args, fmt.Sprintf("--%s", k))
			if v != "" {
				helmContainer.Args = append(helmContainer.Args, v)
			}
		}

		installValues := addon.Spec.Helm.BuildMergedValues(addon.Spec.InstallSpec)
		// set values from URL
		for _, urlValue := range installValues.URLs {
			helmContainer.Args = append(helmContainer.Args, "--values", urlValue)
		}

		// set values from file
		for _, cmRef := range installValues.ConfigMapRefs {
			cm := &corev1.ConfigMap{}
			if err := r.reconciler.Get(ctx, client.ObjectKey{
				Name:      cmRef.Name,
				Namespace: mgrNS}, cm); err != nil {
				if !apierrors.IsNotFound(err) {
					r.setRequeueWithErr(err, "")
					return
				}
				// TODO: handle not found error
				r.setRequeueWithErr(err, "")
				return
			}
			// TODO: validate cmRef.key exist in cm
			attachVolumeMount(helmJobPodSpec, cmRef, cm.Name, "cm",
				func() corev1.VolumeSource {
					return corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cm.Name,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  cmRef.Key,
									Path: cmRef.Key,
								},
							},
						},
					}
				})
		}

		for _, secretRef := range installValues.SecretRefs {
			secret := &corev1.Secret{}
			if err := r.reconciler.Get(ctx, client.ObjectKey{
				Name:      secretRef.Name,
				Namespace: mgrNS}, secret); err != nil {
				if !apierrors.IsNotFound(err) {
					r.setRequeueWithErr(err, "")
					return
				}
				// TODO: handle not found error
				r.setRequeueWithErr(err, "")
				return
			}
			// TODO: validate secretRef.key exist in secret

			attachVolumeMount(helmJobPodSpec, secretRef, secret.Name, "secret",
				func() corev1.VolumeSource {
					return corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
							Items: []corev1.KeyToPath{
								{
									Key:  secretRef.Key,
									Path: secretRef.Key,
								},
							},
						},
					}
				})
		}

		// set key1=val1,key2=val2 value
		if len(installValues.SetValues) > 0 {
			helmContainer.Args = append(helmContainer.Args, "--set",
				strings.Join(installValues.SetValues, ","))
		}

		// set key1=jsonval1,key2=jsonval2 JSON value, applied multiple
		for _, v := range installValues.SetJSONValues {
			helmContainer.Args = append(helmContainer.Args, "--set-json", v)
		}

		if err := r.reconciler.Create(ctx, helmInstallJob); err != nil {
			r.setRequeueWithErr(err, "")
			return
		}
		r.setRequeueAfter(time.Second, "")
	})
	r.next.Handle(ctx)
}

func (r *helmTypeUninstallStage) Handle(ctx context.Context) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("helmTypeUninstallStage", "phase", addon.Status.Phase, "next", r.next.ID())
		key := client.ObjectKey{
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			Name:      getUninstallJobName(addon),
		}
		helmUninstallJob := &batchv1.Job{}
		if err := r.reconciler.Get(ctx, key, helmUninstallJob); client.IgnoreNotFound(err) != nil {
			r.setRequeueWithErr(err, "")
			return
		} else if err == nil {
			if helmUninstallJob.Status.Succeeded > 0 {
				r.reqCtx.Log.V(1).Info("helm uninstall job succeed", "job", key)
				// TODO:
				// helm uninstall should always succeed, therefore need additional label selector to check any
				// helm managed object is not properly cleaned up
				return
			}

			// Job controller has yet handling Job or job controller is not running, i.e., testenv
			// only handle this situation when addon is at terminating state.
			if helmUninstallJob.Status.StartTime.IsZero() && !addon.GetDeletionTimestamp().IsZero() {
				return
			}

			// requeue if uninstall job is active or deleting
			if !helmUninstallJob.GetDeletionTimestamp().IsZero() || helmUninstallJob.Status.Active > 0 {
				r.setRequeueAfter(time.Second, "")
				return
			}
			// there are situations that job.status.[Active | Failed | Succeeded ] are all
			// 0, and len(job.status.conditions) > 0, and need to handle failed
			// info. from conditions.
			if helmUninstallJob.Status.Failed > 0 {
				r.reqCtx.Log.V(1).Info("helm uninstall job failed", "job", key)
				r.reconciler.Recorder.Event(addon, "Warning", UninstallationFailed,
					fmt.Sprintf("Uninstallation failed, do inspect error from jobs.batch %s",
						key.String()))

				if err := r.reconciler.Delete(ctx, helmUninstallJob); client.IgnoreNotFound(err) != nil {
					r.setRequeueWithErr(err, "")
					return
				}
				if err := r.reconciler.cleanupJobPods(*r.reqCtx); err != nil {
					r.setRequeueWithErr(err, "")
					return
				}
			}
			r.setRequeueAfter(time.Second, "")
			return
		}

		// inspect helm releases secrets
		helmSecrets := &corev1.SecretList{}
		if err := r.reconciler.List(ctx, helmSecrets, client.MatchingLabels{
			"name":  getHelmReleaseName(addon),
			"owner": "helm",
		}); err != nil {
			r.setRequeueWithErr(err, "")
			return
		}
		releaseExist := false
		for _, s := range helmSecrets.Items {
			if string(s.Type) == "helm.sh/release.v1" {
				releaseExist = true
				break
			}
		}

		// has no installed release simply return
		if !releaseExist {
			r.reqCtx.Log.V(1).Info("helmTypeUninstallStage release not exist", "job", key)
			return
		}

		r.reqCtx.Log.V(1).Info("creating helm uninstall job", "job", key)
		var err error
		// create `helm delete <release>` job
		helmUninstallJob, err = createHelmJobProto(addon)
		if err != nil {
			r.reqCtx.Log.V(1).Info("helmTypeUninstallStage", "job", key, "err", err)
			r.setRequeueWithErr(err, "")
			return
		}
		helmUninstallJob.ObjectMeta.Name = key.Name
		helmUninstallJob.ObjectMeta.Namespace = key.Namespace
		helmUninstallJob.Spec.Template.Spec.Containers[0].Args = []string{
			"delete",
			"$(RELEASE_NAME)",
			"--namespace",
			"$(RELEASE_NS)",
			"--timeout",
			"10m",
		}
		r.reqCtx.Log.V(1).Info("create helm uninstall job", "job", key)
		if err := r.reconciler.Create(ctx, helmUninstallJob); err != nil {
			r.reqCtx.Log.V(1).Info("helmTypeUninstallStage", "job", key, "err", err)
			r.setRequeueWithErr(err, "")
			return
		}
		r.setRequeueAfter(time.Second, "")
	})
	r.next.Handle(ctx)
}

func (r *enablingStage) Handle(ctx context.Context) {
	r.helmTypeInstallStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("enablingStage", "phase", addon.Status.Phase)
		switch addon.Spec.Type {
		case extensionsv1alpha1.HelmType:
			r.helmTypeInstallStage.Handle(ctx)
		default:
		}
	})
	r.next.Handle(ctx)
}

func (r *disablingStage) Handle(ctx context.Context) {
	r.helmTypeUninstallStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("disablingStage", "phase", addon.Status.Phase, "type", addon.Spec.Type)
		switch addon.Spec.Type {
		case extensionsv1alpha1.HelmType:
			r.helmTypeUninstallStage.Handle(ctx)
		default:
		}
	})
	r.next.Handle(ctx)
}

func (r *terminalStateStage) Handle(ctx context.Context) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("terminalStateStage", "phase", addon.Status.Phase)
		patchPhaseNCondition := func(phase extensionsv1alpha1.AddonPhase, reason string) {
			r.reqCtx.Log.V(1).Info("patching status", "phase", phase)
			patch := client.MergeFrom(addon.DeepCopy())
			addon.Status.Phase = phase
			addon.Status.ObservedGeneration = addon.Generation

			meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
				Type:               extensionsv1alpha1.ConditionTypeSucceed,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: addon.Generation,
				Reason:             reason,
				LastTransitionTime: metav1.Now(),
			})

			if err := r.reconciler.Status().Patch(ctx, addon, patch); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reconciler.Recorder.Event(addon, "Normal", reason,
				fmt.Sprintf("Progress to %s phase", phase))
			r.setReconciled()
		}

		// transit to enabled or disable phase
		switch addon.Status.Phase {
		case "", extensionsv1alpha1.AddonDisabling:
			patchPhaseNCondition(extensionsv1alpha1.AddonDisabled, AddonDisabled)
			return
		case extensionsv1alpha1.AddonEnabling:
			patchPhaseNCondition(extensionsv1alpha1.AddonEnabled, AddonEnabled)
			return
		}
	})
	r.next.Handle(ctx)
}

// attachVolumeMount attach a volumes to pod and added container.VolumeMounts to a ConfigMap
// or Secret referenced key as file, and add --values={volumeMountPath}/{selector.Key} to
// helm install/upgrade args
func attachVolumeMount(
	podSpec *corev1.PodSpec,
	selector extensionsv1alpha1.DataObjectKeySelector,
	objName, suff string,
	volumeSrcBuilder func() corev1.VolumeSource,
) {
	container := &podSpec.Containers[0]
	volName := fmt.Sprintf("%s-%s", objName, suff)
	mountPath := fmt.Sprintf("/vol/%s/%s", suff, objName)
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name:         volName,
		VolumeSource: volumeSrcBuilder(),
	})
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      volName,
		ReadOnly:  true,
		MountPath: mountPath,
	})
	container.Args = append(container.Args, "--values",
		fmt.Sprintf("%s/%s", mountPath, selector.Key))
}

// createHelmJobProto create a job.batch prototyped object
func createHelmJobProto(addon *extensionsv1alpha1.Addon) (*batchv1.Job, error) {
	ttl := time.Minute * 5
	if jobTTL := viper.GetString(constant.CfgKeyAddonJobTTL); jobTTL != "" {
		var err error
		if ttl, err = time.ParseDuration(jobTTL); err != nil {
			return nil, err
		}
	}
	ttlSec := int32(ttl.Seconds())
	helmInstallJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.AddonNameLabelKey:    addon.Name,
				constant.AppManagedByLabelKey: constant.AppName,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSec,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constant.AddonNameLabelKey:    addon.Name,
						constant.AppManagedByLabelKey: constant.AppName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: viper.GetString("KUBEBLOCKS_ADDON_SA_NAME"),
					Containers: []corev1.Container{
						{
							Name:            strings.ToLower(string(addon.Spec.Type)),
							Image:           viper.GetString("KUBEBLOCKS_IMAGE"),
							ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.CfgAddonJobImgPullPolicy)),
							// TODO: need have image that is capable of following settings, current settings
							// may expose potential security risk, as this pod is using cluster-admin clusterrole.
							// SecurityContext: &corev1.SecurityContext{
							//	RunAsNonRoot:             &[]bool{true}[0],
							//	RunAsUser:                &[]int64{1001}[0],
							//	AllowPrivilegeEscalation: &[]bool{false}[0],
							//	Capabilities: &corev1.Capabilities{
							//		Drop: []corev1.Capability{
							//			"ALL",
							//		},
							//	},
							// },
							Command: []string{"helm"},
							Env: []corev1.EnvVar{
								{
									Name:  "RELEASE_NAME",
									Value: getHelmReleaseName(addon),
								},
								{
									Name:  "RELEASE_NS",
									Value: viper.GetString(constant.CfgKeyCtrlrMgrNS),
								},
								{
									Name:  "CHART",
									Value: addon.Spec.Helm.ChartLocationURL,
								},
							},
							VolumeMounts: []corev1.VolumeMount{},
						},
					},
					Volumes:      []corev1.Volume{},
					Tolerations:  []corev1.Toleration{},
					Affinity:     &corev1.Affinity{},
					NodeSelector: map[string]string{},
				},
			},
		},
	}
	podSpec := &helmInstallJob.Spec.Template.Spec
	if cmTolerations := viper.GetString(constant.CfgKeyCtrlrMgrTolerations); cmTolerations != "" &&
		cmTolerations != "[]" && cmTolerations != "[{}]" {
		if err := json.Unmarshal([]byte(cmTolerations), &podSpec.Tolerations); err != nil {
			return nil, err
		}
		isAllEmptyElem := true
		for _, t := range podSpec.Tolerations {
			if t.String() != "{}" {
				isAllEmptyElem = false
				break
			}
		}
		if isAllEmptyElem {
			podSpec.Tolerations = nil
		}
	}
	if cmAffinity := viper.GetString(constant.CfgKeyCtrlrMgrAffinity); cmAffinity != "" {
		if err := json.Unmarshal([]byte(cmAffinity), &podSpec.Affinity); err != nil {
			return nil, err
		}
	}
	if cmNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector); cmNodeSelector != "" {
		if err := json.Unmarshal([]byte(cmNodeSelector), &podSpec.NodeSelector); err != nil {
			return nil, err
		}
	}
	return helmInstallJob, nil
}

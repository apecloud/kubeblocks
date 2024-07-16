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
	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"context"
	"fmt"
	"time"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"

	//"k8s.io/apimachinery/pkg/api/meta"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type progressingReconciler struct {
	stageCtx
	enablingStage  enablingStage
	disablingStage disablingStage
}

func (r *progressingReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}

	return kubebuilderx.ResultSatisfied
}

func (r *progressingReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	r.enablingStage.stageCtx = r.stageCtx
	r.disablingStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("progressingHandler", "phase", addon.Status.Phase)
		patchPhase := func(phase extensionsv1alpha1.AddonPhase, reason string) {
			r.reqCtx.Log.V(1).Info("patching status", "phase", phase)
			patch := client.MergeFrom(addon.DeepCopy())
			addon.Status.Phase = phase
			addon.Status.ObservedGeneration = addon.Generation
			if err := r.reconciler.Status().Patch(r.reqCtx.Ctx, addon, patch); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reconciler.Event(addon, corev1.EventTypeNormal, reason,
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
			r.disablingStage.Handle2(r.reqCtx.Ctx)
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
				if err := r.reconciler.Get(r.reqCtx.Ctx, key, installJob); client.IgnoreNotFound(err) != nil {
					r.setRequeueWithErr(err, "")
					return
				} else if err == nil && installJob.GetDeletionTimestamp().IsZero() {
					if err = r.reconciler.Delete(r.reqCtx.Ctx, installJob); err != nil {
						r.setRequeueWithErr(err, "")
						return
					}
				}
			}
			patchPhase(extensionsv1alpha1.AddonEnabling, EnablingAddon)
			return
		}
		r.reqCtx.Log.V(1).Info("progress to enabling stage handler")
		r.enablingStage.Handle2(r.reqCtx.Ctx)
	})
	//r.next.Handle(r.reqCtx.Ctx)
	return tree, nil
}

func (r *enablingStage) Handle2(ctx context.Context) {
	r.helmTypeInstallStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("enablingStage", "phase", addon.Status.Phase)
		switch addon.Spec.Type {
		case extensionsv1alpha1.HelmType:
			r.helmTypeInstallStage.Handle2(ctx)
		default:
		}
	})
	//r.next.Handle(ctx)
}

func (r *disablingStage) Handle2(ctx context.Context) {
	r.helmTypeUninstallStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("disablingStage", "phase", addon.Status.Phase, "type", addon.Spec.Type)
		switch addon.Spec.Type {
		case extensionsv1alpha1.HelmType:
			r.helmTypeUninstallStage.Handle2(ctx)
		default:
		}
	})
	//r.next.Handle(ctx)
}

func (r *helmTypeInstallStage) Handle2(ctx context.Context) {
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
				setAddonErrorConditions(ctx, &r.stageCtx, addon, true, true, InstallationFailed,
					fmt.Sprintf("Installation failed, do inspect error from jobs.batch %s", key.String()))
				// only allow to do pod logs if max concurrent reconciles > 1, also considered that helm
				// cmd error only has limited contents
				if viper.GetInt(maxConcurrentReconcilesKey) > 1 {
					if err := logFailedJobPodToCondError(ctx, &r.stageCtx, addon, key.Name, InstallationFailedLogs); err != nil {
						r.setRequeueWithErr(err, "")
						return
					}
				}
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

		// set addon installation job to use local charts instead of remote charts,
		// the init container will copy the local charts to the shared volume
		chartsPath, err := buildLocalChartsPath(addon)
		if err != nil {
			r.setRequeueWithErr(err, "")
			return
		}

		helmInstallJob.ObjectMeta.Name = key.Name
		helmInstallJob.ObjectMeta.Namespace = key.Namespace
		helmJobPodSpec := &helmInstallJob.Spec.Template.Spec
		helmContainer := &helmInstallJob.Spec.Template.Spec.Containers[0]
		helmContainer.Args = append([]string{
			"upgrade",
			"--install",
			"$(RELEASE_NAME)",
			chartsPath,
			"--namespace",
			"$(RELEASE_NS)",
			"--create-namespace",
		}, viper.GetStringSlice(addonHelmInstallOptKey)...)

		installValues := addon.Spec.Helm.BuildMergedValues(addon.Spec.InstallSpec)
		if err = addon.Spec.Helm.BuildContainerArgs(helmContainer, installValues); err != nil {
			r.setRequeueWithErr(err, "")
			return
		}

		// set values from file
		for _, cmRef := range installValues.ConfigMapRefs {
			cm := &corev1.ConfigMap{}
			key := client.ObjectKey{
				Name:      cmRef.Name,
				Namespace: mgrNS}
			if err := r.reconciler.Get(ctx, key, cm); err != nil {
				if !apierrors.IsNotFound(err) {
					r.setRequeueWithErr(err, "")
					return
				}
				r.setRequeueAfter(time.Second, fmt.Sprintf("ConfigMap %s not found", cmRef.Name))
				setAddonErrorConditions(ctx, &r.stageCtx, addon, false, true, AddonRefObjError,
					fmt.Sprintf("ConfigMap object %v not found", key))
				return
			}
			if !findDataKey(cm.Data, cmRef) {
				setAddonErrorConditions(ctx, &r.stageCtx, addon, true, true, AddonRefObjError,
					fmt.Sprintf("Attach ConfigMap %v volume source failed, key %s not found", key, cmRef.Key))
				r.setReconciled()
				return
			}
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
			key := client.ObjectKey{
				Name:      secretRef.Name,
				Namespace: mgrNS}
			if err := r.reconciler.Get(ctx, key, secret); err != nil {
				if !apierrors.IsNotFound(err) {
					r.setRequeueWithErr(err, "")
					return
				}
				r.setRequeueAfter(time.Second, fmt.Sprintf("Secret %s not found", secret.Name))
				setAddonErrorConditions(ctx, &r.stageCtx, addon, false, true, AddonRefObjError,
					fmt.Sprintf("Secret object %v not found", key))
				return
			}
			if !findDataKey(secret.Data, secretRef) {
				setAddonErrorConditions(ctx, &r.stageCtx, addon, true, true, AddonRefObjError,
					fmt.Sprintf("Attach Secret %v volume source failed, key %s not found", key, secretRef.Key))
				r.setReconciled()
				return
			}
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

		// if chartLocationURL starts with 'file://', it means the charts is from local file system
		// we will copy the charts from charts image to shared volume. Addon container will use the
		// charts from shared volume to install the addon.
		setSharedVolume(addon, helmJobPodSpec)
		setInitContainer(addon, helmJobPodSpec)

		if err := r.reconciler.Create(ctx, helmInstallJob); err != nil {
			r.setRequeueWithErr(err, "")
			return
		}
		r.setRequeueAfter(time.Second, "")
	})
	//r.next.Handle(ctx)
}

func (r *helmTypeUninstallStage) Handle2(ctx context.Context) {
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
			// only handles this situation when addon is at terminating state.
			if helmUninstallJob.Status.StartTime.IsZero() && !addon.GetDeletionTimestamp().IsZero() {
				return
			}

			// requeue if uninstall job is active or under deleting
			if !helmUninstallJob.GetDeletionTimestamp().IsZero() || helmUninstallJob.Status.Active > 0 {
				r.setRequeueAfter(time.Second, "")
				return
			}
			// there are situations that job.status.[Active | Failed | Succeeded ] are all
			// 0, and len(job.status.conditions) > 0, and need to handle failed
			// info. from conditions.
			if helmUninstallJob.Status.Failed > 0 {
				r.reqCtx.Log.V(1).Info("helm uninstall job failed", "job", key)
				r.reconciler.Event(addon, corev1.EventTypeWarning, UninstallationFailed,
					fmt.Sprintf("Uninstallation failed, do inspect error from jobs.batch %s",
						key.String()))
				// only allow to do pod logs if max concurrent reconciles > 1, also considered that helm
				// cmd error only has limited contents
				if viper.GetInt(maxConcurrentReconcilesKey) > 1 {
					if err := logFailedJobPodToCondError(ctx, &r.stageCtx, addon, key.Name, UninstallationFailedLogs); err != nil {
						r.setRequeueWithErr(err, "")
						return
					}
				}

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
		helmUninstallJob.Spec.Template.Spec.Containers[0].Args = append([]string{
			"delete",
			"$(RELEASE_NAME)",
			"--namespace",
			"$(RELEASE_NS)",
		}, viper.GetStringSlice(addonHelmUninstallOptKey)...)
		r.reqCtx.Log.V(1).Info("create helm uninstall job", "job", key)
		if err := r.reconciler.Create(ctx, helmUninstallJob); err != nil {
			r.reqCtx.Log.V(1).Info("helmTypeUninstallStage", "job", key, "err", err)
			r.setRequeueWithErr(err, "")
			return
		}
		r.setRequeueAfter(time.Second, "")
	})
	//r.next.Handle(ctx)
}

func NewProgressingReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {

	return &progressingReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &progressingReconciler{}

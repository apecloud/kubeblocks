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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/exp/slices"
	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type stageCtx struct {
	reqCtx     *intctrlutil.RequestCtx
	reconciler *AddonReconciler
}

const (
	resultValueKey  = "result"
	errorValueKey   = "err"
	operandValueKey = "operand"
	trueVal         = "true"
	localChartsPath = "/charts"
)

func init() {
	viper.SetDefault(addonSANameKey, "kubeblocks-addon-installer")
	viper.SetDefault(addonHelmInstallOptKey, []string{
		"--atomic",
		"--cleanup-on-fail",
		"--wait",
	})
	viper.SetDefault(addonHelmUninstallOptKey, []string{})
}

func (r *stageCtx) setReconciled() {
	res, err := intctrlutil.Reconciled()
	r.updateResultNErr(&res, err)
}

func (r *stageCtx) setRequeueAfter(duration time.Duration, msg string) {
	res, err := intctrlutil.RequeueAfter(duration, r.reqCtx.Log, msg)
	r.updateResultNErr(&res, err)
}

// func (r *stageCtx) setRequeue(msg string) {
// 	res, err := intctrlutil.Requeue(r.reqCtx.Log, msg)
// 	r.updateResultNErr(&res, err)
// }

func (r *stageCtx) setRequeueWithErr(err error, msg string) {
	res, err := intctrlutil.CheckedRequeueWithError(err, r.reqCtx.Log, msg)
	r.updateResultNErr(&res, err)
}

func (r *stageCtx) updateResultNErr(res *ctrl.Result, err error) {
	r.reqCtx.UpdateCtxValue(errorValueKey, err)
	r.reqCtx.UpdateCtxValue(resultValueKey, res)
}

func (r *stageCtx) doReturn() (*ctrl.Result, error) {
	res, _ := r.reqCtx.Ctx.Value(resultValueKey).(*ctrl.Result)
	err, _ := r.reqCtx.Ctx.Value(errorValueKey).(error)
	return res, err
}

type deletionReconciler struct {
	stageCtx
	disablingReconciler disablingReconciler
}

type helmTypeInstallReconciler struct {
	stageCtx
}

type helmTypeUninstallReconciler struct {
	stageCtx
}

type enablingReconciler struct {
	stageCtx
	helmTypeInstallReconciler helmTypeInstallReconciler
}

type disablingReconciler struct {
	stageCtx
	helmTypeUninstallReconciler helmTypeUninstallReconciler
}

func (r *deletionReconciler) Reconcile(tree *kubebuilderx.ObjectTree) {
	r.disablingReconciler.stageCtx = r.stageCtx
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("deletionStage", "phase", addon.Status.Phase)

	res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
	if res != nil || err != nil {
		r.updateResultNErr(res, err)
		return
	}
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

func useLocalCharts(addon *extensionsv1alpha1.Addon) bool {
	return addon.Spec.Helm != nil && strings.HasPrefix(addon.Spec.Helm.ChartLocationURL, "file://")
}

// buildLocalChartsPath builds the local charts path if the chartLocationURL starts with "file://"
func buildLocalChartsPath(addon *extensionsv1alpha1.Addon) (string, error) {
	if !useLocalCharts(addon) {
		return "$(CHART)", nil
	}

	url := addon.Spec.Helm.ChartLocationURL
	last := strings.LastIndex(url, "/")
	name := url[last+1:]
	return fmt.Sprintf("%s/%s", localChartsPath, name), nil
}

// setSharedVolume sets shared volume to copy helm charts from charts image
func setSharedVolume(addon *extensionsv1alpha1.Addon, helmJobPodSpec *corev1.PodSpec) {
	if !useLocalCharts(addon) {
		return
	}

	helmJobPodSpec.Volumes = append(helmJobPodSpec.Volumes, corev1.Volume{
		Name: "charts",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	helmJobPodSpec.Containers[0].VolumeMounts = append(helmJobPodSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "charts",
		MountPath: localChartsPath,
	})
}

// setInitContainer sets init containers to copy dependent charts to shared volume
func setInitContainer(addon *extensionsv1alpha1.Addon, helmJobPodSpec *corev1.PodSpec) {
	if !useLocalCharts(addon) {
		return
	}

	fromPath := addon.Spec.Helm.ChartsPathInImage
	if fromPath == "" {
		fromPath = localChartsPath
	}
	copyChartsContainer := corev1.Container{
		Name:    "copy-charts",
		Image:   addon.Spec.Helm.ChartsImage,
		Command: []string{"sh", "-c", fmt.Sprintf("cp %s/* /mnt/charts", fromPath)},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "charts",
				MountPath: "/mnt/charts",
			},
		},
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&copyChartsContainer)
	helmJobPodSpec.InitContainers = append(helmJobPodSpec.InitContainers, copyChartsContainer)
}

func (r *AddonReconciler) GetInstallJob(ctx context.Context, jobType string, tree *kubebuilderx.ObjectTree) (job *batchv1.Job, err error) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	key := client.ObjectKey{}
	if jobType == "install" {
		key = client.ObjectKey{
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			Name:      getInstallJobName(addon),
		}
	} else if jobType == "uninstall" {
		key = client.ObjectKey{
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			Name:      getUninstallJobName(addon),
		}
	}
	job = &batchv1.Job{}
	err = r.Get(ctx, key, job)
	return job, err
}

func (r *AddonReconciler) PatchPhase(addon *extensionsv1alpha1.Addon, stageCtx stageCtx, phase extensionsv1alpha1.AddonPhase, reason string) error {
	stageCtx.reqCtx.Log.V(1).Info("patching status", "phase", phase)
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

	if err := r.Status().Patch(stageCtx.reqCtx.Ctx, addon, patch); err != nil {
		stageCtx.setRequeueWithErr(err, "")
		return err
	}
	r.Event(addon, corev1.EventTypeNormal, reason,
		fmt.Sprintf("Progress to %s phase", phase))
	stageCtx.setReconciled()
	return nil
}

func (r *helmTypeInstallReconciler) Reconcile(tree *kubebuilderx.ObjectTree) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("helmTypeInstallStage", "phase", addon.Status.Phase)
	fmt.Println("helmTypeInstallReconciler, phase: ", addon.Status.Phase)
	mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)

	key := client.ObjectKey{
		Namespace: mgrNS,
		Name:      getInstallJobName(addon),
	}

	helmInstallJob := &batchv1.Job{}
	if err := r.reconciler.Get(r.reqCtx.Ctx, key, helmInstallJob); client.IgnoreNotFound(err) != nil {
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
			setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, true, true, InstallationFailed,
				fmt.Sprintf("Installation failed, do inspect error from jobs.batch %s", key.String()))
			// only allow to do pod logs if max concurrent reconciles > 1, also considered that helm
			// cmd error only has limited contents
			if viper.GetInt(maxConcurrentReconcilesKey) > 1 {
				if err := logFailedJobPodToCondError(r.reqCtx.Ctx, &r.stageCtx, addon, key.Name, InstallationFailedLogs); err != nil {
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
		if err := r.reconciler.Get(r.reqCtx.Ctx, key, cm); err != nil {
			if !apierrors.IsNotFound(err) {
				r.setRequeueWithErr(err, "")
				return
			}
			r.setRequeueAfter(time.Second, fmt.Sprintf("ConfigMap %s not found", cmRef.Name))
			setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, false, true, AddonRefObjError,
				fmt.Sprintf("ConfigMap object %v not found", key))
			return
		}
		if !findDataKey(cm.Data, cmRef) {
			setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, true, true, AddonRefObjError,
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
		if err := r.reconciler.Get(r.reqCtx.Ctx, key, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				r.setRequeueWithErr(err, "")
				return
			}
			r.setRequeueAfter(time.Second, fmt.Sprintf("Secret %s not found", secret.Name))
			setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, false, true, AddonRefObjError,
				fmt.Sprintf("Secret object %v not found", key))
			return
		}
		if !findDataKey(secret.Data, secretRef) {
			setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, true, true, AddonRefObjError,
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

	if err := r.reconciler.Create(r.reqCtx.Ctx, helmInstallJob); err != nil {
		r.setRequeueWithErr(err, "")
		return
	}
	r.setRequeueAfter(time.Second, "")
}

func (r *helmTypeUninstallReconciler) Reconcile(tree *kubebuilderx.ObjectTree) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("helmTypeUninstallReconciler", "phase", addon.Status.Phase)
	fmt.Println("helmTypeUninstallReconciler, phase: ", addon.Status.Phase)
	key := client.ObjectKey{
		Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
		Name:      getUninstallJobName(addon),
	}
	helmUninstallJob := &batchv1.Job{}
	if err := r.reconciler.Get(r.reqCtx.Ctx, key, helmUninstallJob); client.IgnoreNotFound(err) != nil {
		r.setRequeueWithErr(err, "")
		return
	} else if err == nil {
		fmt.Println("get helmTypeUninstallJob!!!!!!!!!!!!!!!")
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
				if err := logFailedJobPodToCondError(r.reqCtx.Ctx, &r.stageCtx, addon, key.Name, UninstallationFailedLogs); err != nil {
					r.setRequeueWithErr(err, "")
					return
				}
			}

			if err := r.reconciler.Delete(r.reqCtx.Ctx, helmUninstallJob); client.IgnoreNotFound(err) != nil {
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
	if err := r.reconciler.List(r.reqCtx.Ctx, helmSecrets, client.MatchingLabels{
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
	if err := r.reconciler.Create(r.reqCtx.Ctx, helmUninstallJob); err != nil {
		r.reqCtx.Log.V(1).Info("helmTypeUninstallStage", "job", key, "err", err)
		r.setRequeueWithErr(err, "")
		return
	}
	r.setRequeueAfter(time.Second, "")
}

func (r *enablingReconciler) Reconcile(tree *kubebuilderx.ObjectTree) {
	r.helmTypeInstallReconciler.stageCtx = r.stageCtx
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("enablingStage", "phase", addon.Status.Phase)
	switch addon.Spec.Type {
	case extensionsv1alpha1.HelmType:
		r.helmTypeInstallReconciler.Reconcile(tree)
	default:
	}
}

func (r *disablingReconciler) Reconcile(tree *kubebuilderx.ObjectTree) {
	r.helmTypeUninstallReconciler.stageCtx = r.stageCtx
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("disablingStage", "phase", addon.Status.Phase, "type", addon.Spec.Type)
	switch addon.Spec.Type {
	case extensionsv1alpha1.HelmType:
		r.helmTypeUninstallReconciler.Reconcile(tree)
	default:
	}
}

// attachVolumeMount attaches a volumes to pod and added container.VolumeMounts to a ConfigMap
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

// createHelmJobProto creates a job.batch prototyped object
func createHelmJobProto(addon *extensionsv1alpha1.Addon) (*batchv1.Job, error) {
	ttl := time.Minute * 5
	if jobTTL := viper.GetString(constant.CfgKeyAddonJobTTL); jobTTL != "" {
		var err error
		if ttl, err = time.ParseDuration(jobTTL); err != nil {
			return nil, err
		}
	}
	ttlSec := int32(ttl.Seconds())
	backoffLimit := int32(3)
	container := corev1.Container{
		Name:            getJobMainContainerName(addon),
		Image:           viper.GetString(constant.KBToolsImage),
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
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

	helmProtoJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.AddonNameLabelKey:    addon.Name,
				constant.AppManagedByLabelKey: constant.AppName,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSec,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constant.AddonNameLabelKey:    addon.Name,
						constant.AppManagedByLabelKey: constant.AppName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: viper.GetString("KUBEBLOCKS_ADDON_SA_NAME"),
					Containers:         []corev1.Container{container},
					Volumes:            []corev1.Volume{},
					Tolerations:        []corev1.Toleration{},
					Affinity:           &corev1.Affinity{},
					NodeSelector:       map[string]string{},
				},
			},
		},
	}
	// inherit kubeblocks.io labels from primary resource
	for k, v := range addon.Labels {
		if !strings.Contains(k, constant.APIGroup) {
			continue
		}
		if _, ok := helmProtoJob.ObjectMeta.Labels[k]; !ok {
			helmProtoJob.ObjectMeta.Labels[k] = v
		}
	}

	podSpec := &helmProtoJob.Spec.Template.Spec
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
	return helmProtoJob, nil
}

func enabledAddonWithDefaultValues(ctx context.Context, stageCtx *stageCtx,
	addon *extensionsv1alpha1.Addon, reason, message string) {
	setInstallSpec := func(di *extensionsv1alpha1.AddonDefaultInstallSpecItem) {
		addon.Spec.InstallSpec = di.AddonInstallSpec.DeepCopy()
		addon.Spec.InstallSpec.Enabled = true
		if addon.Annotations == nil {
			addon.Annotations = map[string]string{}
		}
		if di.AddonInstallSpec.IsEmpty() {
			addon.Annotations[AddonDefaultIsEmpty] = trueVal
		}
		if err := stageCtx.reconciler.Client.Update(ctx, addon); err != nil {
			stageCtx.setRequeueWithErr(err, "")
			return
		}
		stageCtx.reconciler.Event(addon, corev1.EventTypeNormal, reason, message)
		stageCtx.setReconciled()
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
}

func setAddonErrorConditions(ctx context.Context,
	stageCtx *stageCtx,
	addon *extensionsv1alpha1.Addon,
	setFailedStatus, recordEvent bool,
	reason, message string,
	eventMessage ...string) {
	patch := client.MergeFrom(addon.DeepCopy())
	addon.Status.ObservedGeneration = addon.Generation
	if setFailedStatus {
		addon.Status.Phase = extensionsv1alpha1.AddonFailed
	}
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               extensionsv1alpha1.ConditionTypeChecked,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: addon.Generation,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})

	if err := stageCtx.reconciler.Status().Patch(ctx, addon, patch); err != nil {
		stageCtx.setRequeueWithErr(err, "")
		return
	}
	if !recordEvent {
		return
	}
	if len(eventMessage) > 0 && eventMessage[0] != "" {
		stageCtx.reconciler.Event(addon, corev1.EventTypeWarning, reason, eventMessage[0])
	} else {
		stageCtx.reconciler.Event(addon, corev1.EventTypeWarning, reason, message)
	}
}

func getJobMainContainerName(addon *extensionsv1alpha1.Addon) string {
	return strings.ToLower(string(addon.Spec.Type))
}

func logFailedJobPodToCondError(ctx context.Context, stageCtx *stageCtx, addon *extensionsv1alpha1.Addon,
	jobName, reason string) error {
	podList := &corev1.PodList{}
	if err := stageCtx.reconciler.List(ctx, podList,
		client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS)),
		client.MatchingLabels{
			constant.AddonNameLabelKey:    stageCtx.reqCtx.Req.Name,
			constant.AppManagedByLabelKey: constant.AppName,
			"job-name":                    jobName,
		}); err != nil {
		return err
	}

	// sort pod with latest creation place front
	slices.SortFunc(podList.Items, func(a, b corev1.Pod) bool {
		return b.CreationTimestamp.Before(&(a.CreationTimestamp))
	})

podsloop:
	for _, pod := range podList.Items {
		switch pod.Status.Phase {
		case corev1.PodFailed:
			clientset, err := corev1client.NewForConfig(stageCtx.reconciler.RestConfig)
			if err != nil {
				return err
			}
			currOpts := &corev1.PodLogOptions{
				Container: getJobMainContainerName(addon),
			}
			req := clientset.Pods(pod.Namespace).GetLogs(pod.Name, currOpts)
			data, err := req.DoRaw(ctx)
			if err != nil {
				return err
			}
			setAddonErrorConditions(ctx, stageCtx, addon, false, true, reason, string(data))
			break podsloop
		}
	}
	return nil
}

func findDataKey[V string | []byte](data map[string]V, refObj extensionsv1alpha1.DataObjectKeySelector) bool {
	for k := range data {
		if k != refObj.Key {
			continue
		}
		return true
	}
	return false
}

func getKubeBlocksDeploy(ctx context.Context, r *AddonReconciler) (*v1.Deployment, error) {
	deploys := &v1.DeploymentList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		constant.AppNameLabelKey:      constant.AppName,
		constant.AppComponentLabelKey: "apps",
	})
	if err := r.Client.List(ctx, deploys, client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS)),
		client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, err
	}
	if len(deploys.Items) == 0 {
		return nil, fmt.Errorf("there is no KubeBlocks deployment, please check your cluster")
	}
	if len(deploys.Items) > 1 {
		return nil, fmt.Errorf("found multiple KubeBlocks deployments, please check your cluster")
	}
	return &deploys.Items[0], nil
}

func getKubeBlocksVersion(ctx context.Context, r *AddonReconciler) (string, error) {
	deploy, err := getKubeBlocksDeploy(ctx, r)
	if err != nil {
		return "", err
	}
	labels := deploy.GetLabels()
	if labels == nil {
		return "", fmt.Errorf("KubeBlocks deployment has no labels")
	}

	v, ok := labels[constant.AppVersionLabelKey]
	if !ok {
		return "", fmt.Errorf("KubeBlocks deployment has no version label")
	}
	return v, nil
}

// this function checks if we try to install or enable an addon directly
func enableOrInstall(addon *extensionsv1alpha1.Addon) bool {
	return addon.Status.Phase == "" && addon.Spec.InstallSpec == nil ||
		addon.Status.Phase == extensionsv1alpha1.AddonFailed && addon.Spec.InstallSpec != nil && addon.Spec.InstallSpec.Enabled ||
		addon.Status.Phase == extensionsv1alpha1.AddonDisabled && addon.Spec.InstallSpec != nil && addon.Spec.InstallSpec.Enabled
}

// check the annotations constraint when install or enable an addon
func checkAnnotationsConstraint(ctx context.Context, reconciler *AddonReconciler, addon *extensionsv1alpha1.Addon) (bool, error) {
	if addon.Annotations == nil || len(addon.Annotations[KBVersionValidate]) == 0 {
		// there is no constraint
		return true, nil
	}
	// check when installing or enabling
	if enableOrInstall(addon) {
		kbVersion, err := getKubeBlocksVersion(ctx, reconciler)
		if err != nil {
			return false, err
		}
		if ok, err := validateVersion(addon.Annotations[KBVersionValidate], kbVersion); err == nil && !ok {
			// kb version is mismatch, set the event and modify the status of the addon
			reconciler.Event(addon, corev1.EventTypeWarning, "Kubeblocks Version Mismatch",
				fmt.Sprintf("The version of kubeblocks needs to be %s, current is %s", addon.Annotations[KBVersionValidate], kbVersion))
			if addon.Status.Phase != extensionsv1alpha1.AddonFailed || meta.FindStatusCondition(addon.Status.Conditions, extensionsv1alpha1.ConditionTypeFailed) == nil {
				patch := client.MergeFrom(addon.DeepCopy())
				addon.Status.Phase = extensionsv1alpha1.AddonFailed
				meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
					Type:               extensionsv1alpha1.ConditionTypeFailed,
					Status:             metav1.ConditionFalse,
					Reason:             InstallableRequirementUnmatched,
					LastTransitionTime: metav1.Now(),
					Message:            fmt.Sprintf("The version of kubeblocks needs to be %s, current is %s", addon.Annotations[KBVersionValidate], kbVersion),
				})
				if err := reconciler.Status().Patch(ctx, addon, patch); err != nil {
					return false, err
				}
			}
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	return true, nil
}

func validateVersion(annotations, kbVersion string) (bool, error) {
	if kbVersion == "" {
		return false, nil
	}
	addPreReleaseInfo := func(constraint string) string {
		constraint = strings.Trim(constraint, " ")
		split := strings.Split(constraint, "-")
		if len(split) == 1 && (strings.HasPrefix(constraint, ">") || strings.Contains(constraint, "<")) {
			constraint += "-0"
		}
		return constraint
	}
	if strings.Contains(kbVersion, "-") {
		rules := strings.Split(annotations, ",")
		for i := range rules {
			rules[i] = addPreReleaseInfo(rules[i])
		}
		annotations = strings.Join(rules, ",")
	}
	constraint, err := semver.NewConstraint(annotations)
	if err != nil {
		return false, err
	}
	v, err := semver.NewVersion(kbVersion)
	if err != nil {
		return false, err
	}
	validate, _ := constraint.Validate(v)
	return validate, nil
}

func checkAddonSpec(addon *extensionsv1alpha1.Addon) error {
	if addon.Spec.Type == extensionsv1alpha1.HelmType {
		if addon.Spec.Helm == nil {
			return fmt.Errorf("invalid Helm configuration: either 'Helm' is not specified")
		}
	}
	return nil
}

func setAddonProviderAndVersion(addon *extensionsv1alpha1.Addon) {
	// if not set provider and version in spec, set it from labels
	if addon.Labels == nil {
		addon.Labels = map[string]string{}
	}
	if addon.Spec.Provider == "" {
		addon.Spec.Provider = addon.Labels[AddonProvider]
	}
	if addon.Spec.Version == "" {
		addon.Spec.Version = addon.Labels[AddonVersion]
	}
	// handle the reverse logic
	if len(addon.Labels[AddonProvider]) == 0 && len(addon.Spec.Provider) != 0 {
		addon.Labels[AddonProvider] = addon.Spec.Provider
	}
	if len(addon.Labels[AddonVersion]) == 0 && len(addon.Spec.Version) != 0 {
		addon.Labels[AddonVersion] = addon.Spec.Version
	}
}

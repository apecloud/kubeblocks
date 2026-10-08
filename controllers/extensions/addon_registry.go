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

package extensions

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
)

// prepareRegistrySecret supplies Helm with a Docker credential file. Helm selects credentials
// for the chart's registry; an empty auths map or an unmatched registry permits anonymous access.
func (r *AddonReconciler) prepareRegistrySecret(ctx context.Context, addon *extensionsv1alpha1.Addon, job *batchv1.Job) error {
	ref := addon.Spec.Helm.RegistrySecretRef
	if ref == nil {
		return nil
	}
	if ref.Name == "" || ref.Namespace == "" {
		return apierrors.NewBadRequest("registrySecretRef.name and registrySecretRef.namespace are required")
	}
	key := client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, key, secret); err != nil {
		return fmt.Errorf("read registry Secret %s: %w", key, err)
	}
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return apierrors.NewBadRequest(fmt.Sprintf("registry Secret %s must be of type %s", key, corev1.SecretTypeDockerConfigJson))
	}

	secretName := secret.Name
	if secret.Namespace != job.Namespace {
		// Pods can only mount local Secrets. Refresh the copy for each installation;
		// the Addon owner reference lets Kubernetes reclaim it when the Addon is deleted.
		copySecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      registrySecretCopyName(addon),
			Namespace: job.Namespace,
		}}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, copySecret, func() error {
			if copySecret.ResourceVersion != "" && !metav1.IsControlledBy(copySecret, addon) {
				return apierrors.NewBadRequest(fmt.Sprintf("registry Secret copy %s is not owned by Addon %s",
					client.ObjectKeyFromObject(copySecret), addon.Name))
			}
			if err := controllerutil.SetControllerReference(addon, copySecret, r.Scheme); err != nil {
				return err
			}
			copySecret.Type = corev1.SecretTypeDockerConfigJson
			copySecret.Data = map[string][]byte{corev1.DockerConfigJsonKey: secret.Data[corev1.DockerConfigJsonKey]}
			return nil
		})
		if err != nil {
			return fmt.Errorf("prepare registry Secret copy: %w", err)
		}
		secretName = copySecret.Name
	}

	selector := extensionsv1alpha1.DataObjectKeySelector{Name: secretName, Key: corev1.DockerConfigJsonKey}
	path := mountDataObjectKey(&job.Spec.Template.Spec, selector, "addon", "registry", func() corev1.VolumeSource {
		return corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName: secretName,
			Items:      []corev1.KeyToPath{{Key: selector.Key, Path: selector.Key}},
		}}
	})
	job.Spec.Template.Spec.Containers[0].Args = append(job.Spec.Template.Spec.Containers[0].Args, "--registry-config", path)
	return nil
}

func registrySecretCopyName(addon *extensionsv1alpha1.Addon) string {
	return "addon-registry-" + string(addon.UID)
}

func (r *AddonReconciler) deleteRegistrySecretCopy(ctx context.Context, addon *extensionsv1alpha1.Addon, namespace string) error {
	copySecret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: namespace, Name: registrySecretCopyName(addon)}
	if err := r.Get(ctx, key, copySecret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(copySecret, addon) {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, copySecret))
}

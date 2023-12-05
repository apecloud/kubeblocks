/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package component

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func resolveServiceReferences(ctx context.Context, cli client.Reader, comp *SynthesizedComponent) error {
	for _, serviceDescriptor := range comp.ServiceReferences {
		// Only support referencing endpoint and port in configuration
		credentialVars := []*appsv1alpha1.CredentialVar{
			serviceDescriptor.Spec.Endpoint,
			serviceDescriptor.Spec.Port,
		}
		if err := resolveServiceRefCredentialVars(ctx, cli, serviceDescriptor.Namespace, credentialVars...); err != nil {
			return err
		}
	}
	return nil
}

// resolveServiceRefCredentialVars resolves the credentialVar.ValueFrom to the real value
// TODO: currently, we set the valueFrom to the value, which need to be refactored
func resolveServiceRefCredentialVars(ctx context.Context, cli client.Reader,
	namespace string, credentialVars ...*appsv1alpha1.CredentialVar) error {
	for _, credentialVar := range credentialVars {
		// TODO: replace the build-in placeholder with the real value
		if credentialVar == nil || credentialVar.Value != "" {
			continue
		}
		// TODO: currently, we set the valueFrom to the value, which need to be refactored
		if credentialVar.ValueFrom != nil {
			if err := resolveSecretRefCredentialVar(ctx, cli, namespace, credentialVar); err != nil {
				return err
			}
			if err := resolveConfigMapRefCredentialVar(ctx, cli, namespace, credentialVar); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSecretRefCredentialVar(ctx context.Context, cli client.Reader,
	namespace string, credentialVar *appsv1alpha1.CredentialVar) error {
	if credentialVar.ValueFrom == nil || credentialVar.ValueFrom.SecretKeyRef == nil {
		return nil
	}
	secretName := credentialVar.ValueFrom.SecretKeyRef.Name
	secretKey := credentialVar.ValueFrom.SecretKeyRef.Key
	secretRef := &corev1.Secret{}
	if err := cli.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secretRef); err != nil {
		return err
	}
	runtimeValBytes, ok := secretRef.Data[secretKey]
	if !ok {
		// return fmt.Errorf("couldn't find key %v in Secret %v/%v", secretKey, namespace, secretName)
		return nil
	}
	// Set the valueFrom to the value and clear the valueFrom
	credentialVar.ValueFrom = nil
	credentialVar.Value = string(runtimeValBytes)
	return nil
}

func resolveConfigMapRefCredentialVar(ctx context.Context, cli client.Reader,
	namespace string, credentialVar *appsv1alpha1.CredentialVar) error {
	if credentialVar.ValueFrom == nil || credentialVar.ValueFrom.ConfigMapKeyRef == nil {
		return nil
	}
	configMapName := credentialVar.ValueFrom.ConfigMapKeyRef.Name
	configMapKey := credentialVar.ValueFrom.ConfigMapKeyRef.Key
	configMapRef := &corev1.ConfigMap{}
	if err := cli.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: namespace}, configMapRef); err != nil {
		return err
	}
	runtimeValBytes, ok := configMapRef.Data[configMapKey]
	if !ok {
		// return fmt.Errorf("couldn't find key %v in ConfigMap %v/%v", configMapKey, namespace, configMapName)
		return nil
	}
	// Set the valueFrom to the value and clear the valueFrom
	credentialVar.ValueFrom = nil
	credentialVar.Value = runtimeValBytes
	return nil
}

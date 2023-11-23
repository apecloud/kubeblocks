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

package configuration

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

type Visitor interface {
	Visit(VisitorFunc) error
}

type VisitorFunc func(*component.SynthesizedComponent, error) error

// DecoratedVisitor will invoke the decorators in order prior to invoking the visitor function
// passed to Visit. An error will terminate the visit.
type DecoratedVisitor struct {
	visitor    Visitor
	decorators []VisitorFunc
}

// NewDecoratedVisitor will create a visitor that invokes the provided visitor functions before
// the user supplied visitor function is invoked, giving them the opportunity to mutate the Info
// object or terminate early with an error.
func NewDecoratedVisitor(v Visitor, fn ...VisitorFunc) Visitor {
	if len(fn) == 0 {
		return v
	}
	return DecoratedVisitor{v, fn}
}

// Visit implements Visitor
func (v DecoratedVisitor) Visit(fn VisitorFunc) error {
	return v.visitor.Visit(func(component *component.SynthesizedComponent, err error) error {
		if err != nil {
			return err
		}
		for i := range v.decorators {
			if err := v.decorators[i](component, nil); err != nil {
				return err
			}
		}
		return fn(component, nil)
	})
}

// ComponentVisitor implements Visitor, it will visit the component.SynthesizedComponent
type ComponentVisitor struct {
	component *component.SynthesizedComponent
}

// Visit implements Visitor
func (r *ComponentVisitor) Visit(fn VisitorFunc) error {
	return fn(r.component, nil)
}

// resolveServiceReferences is the visitor function to resolve the service reference
func resolveServiceReferences(cli client.Reader, ctx context.Context) VisitorFunc {
	return func(component *component.SynthesizedComponent, err error) error {
		if err != nil {
			return err
		}
		if component.ServiceReferences == nil {
			return nil
		}
		for _, serviceDescriptor := range component.ServiceReferences {
			// only support referencing endpoint and port in configuration
			credentialVars := []*appsv1alpha1.CredentialVar{
				serviceDescriptor.Spec.Endpoint,
				serviceDescriptor.Spec.Port,
			}
			if err = resolveCredentialVar(cli, ctx, serviceDescriptor.Namespace, credentialVars...); err != nil {
				return err
			}
		}
		return nil
	}
}

// resolveCredentialVar resolve the credentialVar.ValueFrom to the real value
// TODO: currently, we set the valueFrom to the value, which need to be refactored
func resolveCredentialVar(cli client.Reader, ctx context.Context, namespace string, credentialVars ...*appsv1alpha1.CredentialVar) error {
	for _, credentialVar := range credentialVars {
		// TODO: replace the build-in placeholder with the real value
		if credentialVar == nil || credentialVar.Value != "" {
			continue
		}
		// TODO: currently, we set the valueFrom to the value, which need to be refactored
		if credentialVar.ValueFrom != nil {
			if err := resolveSecretRef(cli, ctx, namespace, credentialVar); err != nil {
				return err
			}
			if err := resolveConfigMapRef(cli, ctx, namespace, credentialVar); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSecretRef(cli client.Reader, ctx context.Context, namespace string, credentialVar *appsv1alpha1.CredentialVar) error {
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

func resolveConfigMapRef(cli client.Reader, ctx context.Context, namespace string, credentialVar *appsv1alpha1.CredentialVar) error {
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

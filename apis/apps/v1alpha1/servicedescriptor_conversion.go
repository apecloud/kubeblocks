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

package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ConvertTo converts this Demo to the Hub version (v2).
func (r *ServiceDescriptor) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.ServiceDescriptor)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	dst.Spec.ServiceKind = r.Spec.ServiceKind
	dst.Spec.ServiceVersion = r.Spec.ServiceVersion
	dst.Spec.Endpoint = r.credentialVarTo(r.Spec.Endpoint)
	dst.Spec.Host = r.credentialVarTo(r.Spec.Host)
	dst.Spec.Port = r.credentialVarTo(r.Spec.Port)
	if r.Spec.Auth == nil {
		dst.Spec.Auth = nil
	} else {
		dst.Spec.Auth = &appsv1.ConnectionCredentialAuth{
			Username: r.credentialVarTo(r.Spec.Auth.Username),
			Password: r.credentialVarTo(r.Spec.Auth.Password),
		}
	}

	// status
	dst.Status.ObservedGeneration = r.Status.ObservedGeneration
	dst.Status.Phase = appsv1.Phase(r.Status.Phase)
	dst.Status.Message = r.Status.Message

	return nil
}

func (r *ServiceDescriptor) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.ServiceDescriptor)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	r.Spec.ServiceKind = src.Spec.ServiceKind
	r.Spec.ServiceVersion = src.Spec.ServiceVersion
	r.Spec.Endpoint = r.credentialVarFrom(src.Spec.Endpoint)
	r.Spec.Host = r.credentialVarFrom(src.Spec.Host)
	r.Spec.Port = r.credentialVarFrom(src.Spec.Port)
	if r.Spec.Auth == nil {
		r.Spec.Auth = nil
	} else {
		r.Spec.Auth = &ConnectionCredentialAuth{
			Username: r.credentialVarFrom(src.Spec.Auth.Username),
			Password: r.credentialVarFrom(src.Spec.Auth.Password),
		}
	}

	// status
	r.Status.ObservedGeneration = src.Status.ObservedGeneration
	r.Status.Phase = Phase(src.Status.Phase)
	r.Status.Message = src.Status.Message

	return nil
}

func (r *ServiceDescriptor) credentialVarTo(src *CredentialVar) *appsv1.CredentialVar {
	if src != nil {
		return &appsv1.CredentialVar{
			Value:     src.Value,
			ValueFrom: src.ValueFrom,
		}
	}
	return nil
}

func (r *ServiceDescriptor) credentialVarFrom(src *appsv1.CredentialVar) *CredentialVar {
	if src != nil {
		return &CredentialVar{
			Value:     src.Value,
			ValueFrom: src.ValueFrom,
		}
	}
	return nil
}

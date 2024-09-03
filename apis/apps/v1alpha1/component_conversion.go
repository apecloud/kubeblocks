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
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this Component to the Hub version (v1).
func (r *Component) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.Component)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	dst.Spec.CompatibilityRules = r.compatibilityRulesTo(r.Spec.CompatibilityRules)
	dst.Spec.Releases = r.releasesTo(r.Spec.Releases)

	// status
	dst.Status.ObservedGeneration = r.Status.ObservedGeneration
	dst.Status.Phase = appsv1.Phase(r.Status.Phase)
	dst.Status.Message = r.Status.Message
	dst.Status.ServiceVersions = r.Status.ServiceVersions

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (r *Component) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.Component)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	r.Spec.CompatibilityRules = r.compatibilityRulesFrom(src.Spec.CompatibilityRules)
	r.Spec.Releases = r.releasesFrom(src.Spec.Releases)

	// status
	r.Status.ObservedGeneration = src.Status.ObservedGeneration
	r.Status.Phase = Phase(src.Status.Phase)
	r.Status.Message = src.Status.Message
	r.Status.ServiceVersions = src.Status.ServiceVersions

	return nil
}

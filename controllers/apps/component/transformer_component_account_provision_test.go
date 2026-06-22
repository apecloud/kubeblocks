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

package component

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/constant"
	componentctrl "github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestAccountAlreadyProvisioned(t *testing.T) {
	transformer := &componentAccountProvisionTransformer{}
	transCtx := &componentTransformContext{
		SynthesizeComponent: &componentctrl.SynthesizedComponent{},
	}

	require.False(t, transformer.accountAlreadyProvisioned(transCtx, &corev1.Secret{}))

	require.True(t, transformer.accountAlreadyProvisioned(transCtx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey: "true",
			},
		},
	}))
}

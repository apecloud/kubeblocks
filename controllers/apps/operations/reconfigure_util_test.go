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

package operations

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func testUpdateConfigConfigmapResource(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	resource *OpsResource,
	config appsv1alpha1.ConfigurationItem,
	clusterName, componentName string) reconfiguringResult {

	return newPipeline(reconfigureContext{
		cli:           cli,
		reqCtx:        reqCtx,
		resource:      resource,
		config:        config,
		clusterName:   clusterName,
		componentName: componentName,
	}).Configuration().
		Validate().
		ConfigMap(config.Name).
		ConfigConstraints().
		Merge().
		UpdateOpsLabel().
		Sync().
		Complete()
}

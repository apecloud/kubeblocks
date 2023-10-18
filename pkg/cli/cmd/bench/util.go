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

package bench

import (
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
)

func getDriverAndHostAndPort(c *appsv1alpha1.Cluster, svcList *corev1.ServiceList) (driver string, host string, port int, err error) {
	var internalEndpoints []string
	var externalEndpoints []string

	if c == nil {
		return "", "", 0, fmt.Errorf("cluster is nil")
	}

	for _, comp := range c.Spec.ComponentSpecs {
		driver = comp.Name
		internalEndpoints, externalEndpoints = cluster.GetComponentEndpoints(svcList, &comp)
		if len(internalEndpoints) > 0 || len(externalEndpoints) > 0 {
			break
		}
	}
	switch {
	case len(internalEndpoints) > 0:
		host, port, err = parseHostAndPort(internalEndpoints[0])
	case len(externalEndpoints) > 0:
		host, port, err = parseHostAndPort(externalEndpoints[0])
	default:
		err = fmt.Errorf("no endpoints found")
	}

	return
}

func parseHostAndPort(s string) (string, int, error) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, err
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	return host, portInt, nil
}

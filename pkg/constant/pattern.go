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

package constant

import (
	"fmt"
)

// GenerateDefaultConnCredential generates default connection credential name of cluster
func GenerateDefaultConnCredential(clusterName string) string {
	return GenerateClusterConnCredential(clusterName, "")
}

// GenerateClusterConnCredential generates connection credential name of cluster
func GenerateClusterConnCredential(clusterName, name string) string {
	if len(name) == 0 {
		name = "conn-credential"
	}
	return fmt.Sprintf("%s-%s", clusterName, name)
}

// GenerateComponentConnCredential generates connection credential name of component
func GenerateComponentConnCredential(clusterName, compName, name string) string {
	if len(name) == 0 {
		name = "conn-credential"
	}
	return fmt.Sprintf("%s-%s-%s", clusterName, compName, name)
}

// GenerateComponentServiceEndpoint generates service endpoint of component
func GenerateComponentServiceEndpoint(clusterName, compName, svcName, namespace string) string {
	return fmt.Sprintf("%s-%s-%s.%s.svc", clusterName, compName, svcName, namespace)
}

// GenerateDefaultComponentServiceEndpoint generates default service endpoint of component
func GenerateDefaultComponentServiceEndpoint(clusterName, compName, namespace string) string {
	return fmt.Sprintf("%s-%s.%s.svc", clusterName, compName, namespace)
}

// GenerateComponentHeadlessServiceEndpoint generates headless service endpoint of component
func GenerateComponentHeadlessServiceEndpoint(clusterName, compName, svcName, namespace string) string {
	return fmt.Sprintf("%s-%s-%s-headless.%s.svc", clusterName, compName, svcName, namespace)
}

// GenerateDefaultComponentHeadlessServiceEndpoint generates default headless service endpoint of component
func GenerateDefaultComponentHeadlessServiceEndpoint(clusterName, compName, namespace string) string {
	return fmt.Sprintf("%s-%s-headless.%s.svc", clusterName, compName, namespace)
}

// GenerateClusterComponentPattern generates cluster and component pattern
func GenerateClusterComponentPattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateRSMNamePattern generates rsm name pattern
func GenerateRSMNamePattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateRSMServiceNamePattern generates rsm name pattern
func GenerateRSMServiceNamePattern(rsmName string) string {
	return fmt.Sprintf("%s-headless", rsmName)
}

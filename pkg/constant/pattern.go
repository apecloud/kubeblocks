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

// GenerateClusterComponentName generates the cluster component name.
func GenerateClusterComponentName(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateAccountSecretName generates the secret name of system accounts.
func GenerateAccountSecretName(clusterName, compName, name string) string {
	return fmt.Sprintf("%s-%s-account-%s", clusterName, compName, name)
}

// GenerateClusterServiceName generates the service name for cluster.
func GenerateClusterServiceName(clusterName, svcName string) string {
	if len(svcName) > 0 {
		return fmt.Sprintf("%s-%s", clusterName, svcName)
	}
	return clusterName
}

// GenerateClusterHeadlessServiceName generates the headless service name for cluster.
func GenerateClusterHeadlessServiceName(clusterName, svcName string) string {
	if len(svcName) > 0 {
		return fmt.Sprintf("%s-%s-headless", clusterName, svcName)
	}
	return fmt.Sprintf("%s-headless", clusterName)
}

// GenerateComponentServiceName generates the service name for component.
func GenerateComponentServiceName(clusterName, compName, svcName string) string {
	if len(svcName) > 0 {
		return fmt.Sprintf("%s-%s-%s", clusterName, compName, svcName)
	}
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateDefaultComponentServiceName generates the default service name for component.
func GenerateDefaultComponentServiceName(clusterName, compName string) string {
	return GenerateComponentServiceName(clusterName, compName, "")
}

// GenerateComponentHeadlessServiceName generates the headless service name for component.
func GenerateComponentHeadlessServiceName(clusterName, compName, svcName string) string {
	if len(svcName) > 0 {
		return fmt.Sprintf("%s-%s-%s-headless", clusterName, compName, svcName)
	}
	return fmt.Sprintf("%s-%s-headless", clusterName, compName)
}

// GenerateDefaultComponentHeadlessServiceName generates the default headless service name for component.
func GenerateDefaultComponentHeadlessServiceName(clusterName, compName string) string {
	return GenerateComponentHeadlessServiceName(clusterName, compName, "")
}

// GenerateDefaultConnCredential generates the default connection credential name for cluster.
func GenerateDefaultConnCredential(clusterName string) string {
	return GenerateClusterConnCredential(clusterName, "")
}

// GenerateClusterConnCredential generates the connection credential name for cluster.
func GenerateClusterConnCredential(clusterName, name string) string {
	if len(name) == 0 {
		name = "conn-credential"
	}
	return fmt.Sprintf("%s-%s", clusterName, name)
}

// GenerateComponentConnCredential generates the connection credential name for component.
func GenerateComponentConnCredential(clusterName, compName, name string) string {
	if len(name) == 0 {
		name = "conn-credential"
	}
	return fmt.Sprintf("%s-%s-%s", clusterName, compName, name)
}

// GenerateClusterComponentEnvPattern generates cluster and component pattern
func GenerateClusterComponentEnvPattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s-env", clusterName, compName)
}

// GenerateDefaultCompServiceAccountPattern generates default component service account pattern
// fullCompName is the full name of component with clusterName prefix
func GenerateDefaultCompServiceAccountPattern(fullCompName string) string {
	return fmt.Sprintf("%s-%s", KBLowerPrefix, fullCompName)
}

// GenerateRSMNamePattern generates rsm name pattern
func GenerateRSMNamePattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateRSMServiceNamePattern generates rsm name pattern
func GenerateRSMServiceNamePattern(rsmName string) string {
	return fmt.Sprintf("%s-headless", rsmName)
}

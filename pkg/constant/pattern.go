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

package constant

import (
	"fmt"
	"strings"
)

// GenerateClusterComponentName generates the cluster component name.
func GenerateClusterComponentName(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateAccountSecretName generates the secret name of system accounts.
func GenerateAccountSecretName(clusterName, compName, name string) string {
	replacedName := strings.ReplaceAll(name, "_", "-")
	return fmt.Sprintf("%s-%s-account-%s", clusterName, compName, replacedName)
}

// GenerateClusterServiceName generates the service name for cluster.
func GenerateClusterServiceName(clusterName, svcName string) string {
	if len(svcName) > 0 {
		return fmt.Sprintf("%s-%s", clusterName, svcName)
	}
	return clusterName
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
// TODO: deprecated, will be removed later.
func GenerateDefaultConnCredential(clusterName string) string {
	return fmt.Sprintf("%s-conn-credential", clusterName)
}

// GenerateClusterComponentEnvPattern generates cluster and component pattern
func GenerateClusterComponentEnvPattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s-env", clusterName, compName)
}

// GenerateDefaultServiceAccountName generates default service account name for a cluster.
func GenerateDefaultServiceAccountName(name string) string {
	return fmt.Sprintf("%s-%s", KBLowerPrefix, name)
}

// GenerateWorkloadNamePattern generates the workload name pattern
func GenerateWorkloadNamePattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

// GenerateServiceNamePattern generates the service name pattern
func GenerateServiceNamePattern(itsName string) string {
	return fmt.Sprintf("%s-headless", itsName)
}

// GeneratePodName generates the connection credential name for component.
func GeneratePodName(clusterName, compName string, ordinal int) string {
	return fmt.Sprintf("%s-%d", GenerateClusterComponentName(clusterName, compName), ordinal)
}

// GeneratePodSubDomain generates the connection credential name for component.
func GeneratePodSubDomain(clusterName, compName string) string {
	return GenerateDefaultComponentHeadlessServiceName(clusterName, compName)
}

// GeneratePodFQDN generates the connection credential name for component.
func GeneratePodFQDN(namespace, clusterName, compName string, ordinal int) string {
	return fmt.Sprintf("%s.%s.%s.svc",
		GeneratePodName(clusterName, compName, ordinal), GeneratePodSubDomain(clusterName, compName), namespace)
}

// GenerateVirtualComponentDefinition generates the virtual component definition name.
func GenerateVirtualComponentDefinition(compDefSuffix string) string {
	return fmt.Sprintf("%s-%s", KBGeneratedVirtualCompDefPrefix, compDefSuffix)
}

// GenerateResourceNameWithScalingSuffix generates name with '-scaling' suffix.
func GenerateResourceNameWithScalingSuffix(name string) string {
	return fmt.Sprintf("%s-%s", name, SlashScalingLowerSuffix)
}

// GenerateShardingNamePrefix generates sharding name prefix.
func GenerateShardingNamePrefix(shardingName string) string {
	return fmt.Sprintf("%s-", shardingName)
}

// GenerateShardingNameSvcPrefix generates sharding service name prefix.
func GenerateShardingNameSvcPrefix(shardingSvcName string) string {
	return fmt.Sprintf("%s-", shardingSvcName)
}

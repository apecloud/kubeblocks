/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"hash/fnv"
	"strings"
)

const KubeNameMaxLength = 63

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

// GenerateClusterComponentEnvPattern generates cluster and component pattern
func GenerateClusterComponentEnvPattern(clusterName, compName string) string {
	return GetCompEnvCMName(fmt.Sprintf("%s-%s", clusterName, compName))
}

func GetCompEnvCMName(compObjName string) string {
	return fmt.Sprintf("%s-env", compObjName)
}

// GenerateDefaultServiceAccountName generates default service account name for a component.
func GenerateDefaultServiceAccountName(cmpdName string) string {
	return fmt.Sprintf("%s-%s", KBLowerPrefix, cmpdName)
}

func GenerateDefaultServiceAccountNameNew(fullCompName string) string {
	return fmt.Sprintf("%s-%s", KBLowerPrefix, fullCompName)
}

// GenerateDefaultRoleName generates default role name for a component.
func GenerateDefaultRoleName(cmpdName string) string {
	return fmt.Sprintf("%s-%s", KBLowerPrefix, cmpdName)
}

// GenerateWorkloadNamePattern generates the workload name pattern
func GenerateWorkloadNamePattern(clusterName, compName string) string {
	return fmt.Sprintf("%s-%s", clusterName, compName)
}

func ShortenKubeName(raw string, maxLen int) string {
	if maxLen <= 0 || len(raw) <= maxLen {
		return raw
	}
	suffix := shortHash(raw)
	if maxLen <= len(suffix)+1 {
		return suffix[:maxLen]
	}
	prefixLen := maxLen - len(suffix) - 1
	prefix := strings.TrimSuffix(raw[:prefixLen], "-")
	if prefix == "" {
		return suffix[:maxLen]
	}
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

func ShortenKubeNameWithSuffix(raw, fixedSuffix string, maxLen int) string {
	if fixedSuffix == "" {
		return ShortenKubeName(raw, maxLen)
	}
	fullName := fmt.Sprintf("%s-%s", raw, fixedSuffix)
	if maxLen <= 0 || len(fullName) <= maxLen {
		return fullName
	}
	reserved := len(fixedSuffix) + 1
	if maxLen <= reserved {
		return fullName[:maxLen]
	}
	return fmt.Sprintf("%s-%s", ShortenKubeName(raw, maxLen-reserved), fixedSuffix)
}

func shortHash(raw string) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(raw))
	return fmt.Sprintf("%08x", hasher.Sum32())
}

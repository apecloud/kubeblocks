/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"strings"
)

type K8sProvider string

const (
	EKSProvider     K8sProvider = "EKS"
	UnknownProvider K8sProvider = "unknown"
)

func (p K8sProvider) IsCloud() bool {
	return p != UnknownProvider
}

func GetK8sProvider(version string) K8sProvider {
	strArr := strings.Split(version, "-")
	// for EKS, its version like v1.24.8-eks-*****
	if len(strArr) >= 2 {
		return K8sProvider(strings.ToUpper(strArr[1]))
	}
	return UnknownProvider
}

func GetK8sVersion(version string) string {
	removeFirstChart := func(v string) string {
		if len(v) == 0 {
			return v
		}
		if v[0] == 'v' {
			return v[1:]
		}
		return v
	}

	if len(version) == 0 {
		return version
	}

	strArr := strings.Split(version, "-")
	return removeFirstChart(strArr[0])
}

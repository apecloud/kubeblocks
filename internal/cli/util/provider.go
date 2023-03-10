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
	"regexp"
	"strings"
)

type K8sProvider string

const (
	EKSProvider     K8sProvider = "EKS"
	GKEProvider     K8sProvider = "GKE"
	AKSProvider     K8sProvider = "AKS"
	ACKProvider     K8sProvider = "ACK"
	UnknownProvider K8sProvider = "unknown"
)

func (p K8sProvider) IsCloud() bool {
	return p != UnknownProvider
}

var (
	/*
	   EKS version info:
	   WARNING: This version information is deprecated and will be replaced with the output from kubectl version --short.  Use --output=yaml|json to get the full version.
	   Client Version: version.Info{Major:"1", Minor:"26", GitVersion:"v1.26.1", GitCommit:"8f94681cd294aa8cfd3407b8191f6c70214973a4", GitTreeState:"clean", BuildDate:"2023-01-18T15:51:24Z", GoVersion:"go1.19.5", Compiler:"gc", Platform:"darwin/arm64"}
	   Kustomize Version: v4.5.7
	   Server Version: version.Info{Major:"1", Minor:"24+", GitVersion:"v1.24.10-eks-48e63af", GitCommit:"9176fb99b52f8d5ff73d67fea27f3a638f679f8a", GitTreeState:"clean", BuildDate:"2023-01-24T19:17:48Z", GoVersion:"go1.19.5", Compiler:"gc", Platform:"linux/amd64"}
	   WARNING: version difference between client (1.26) and server (1.24) exceeds the supported minor version skew of +/-1

	   GKE version info:
	   WARNING: This version information is deprecated and will be replaced with the output from kubectl version --short.  Use --output=yaml|json to get the full version.
	   Client Version: version.Info{Major:"1", Minor:"26", GitVersion:"v1.26.1", GitCommit:"8f94681cd294aa8cfd3407b8191f6c70214973a4", GitTreeState:"clean", BuildDate:"2023-01-18T15:51:24Z", GoVersion:"go1.19.5", Compiler:"gc", Platform:"darwin/arm64"}
	   Kustomize Version: v4.5.7
	   Server Version: version.Info{Major:"1", Minor:"24", GitVersion:"v1.24.9-gke.3200", GitCommit:"92ea556d4e7418d0e7b5db1ee576a73f8fc47e91", GitTreeState:"clean", BuildDate:"2023-01-20T09:29:29Z", GoVersion:"go1.18.9b7", Compiler:"gc", Platform:"linux/amd64"}
	   WARNING: version difference between client (1.26) and server (1.24) exceeds the supported minor version skew of +/-1
	*/
	k8sVersionRegex = map[K8sProvider]string{
		EKSProvider: "v.*-eks-.*",
		GKEProvider: "v.*-gke.*",
	}
)

func GetK8sProvider(version string) K8sProvider {
	for provider, reg := range k8sVersionRegex {
		match, err := regexp.Match(reg, []byte(version))
		if err != nil {
			continue
		}
		if match {
			return provider
		}
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
	if len(strArr) == 0 {
		return ""
	}
	return removeFirstChart(strArr[0])
}

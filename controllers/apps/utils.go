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

package apps

import (
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
)

func getEnvReplacementMapForConnCredential(clusterName string) map[string]string {
	return map[string]string{
		constant.ConnCredentialPlaceHolder: component.GenerateConnCredential(clusterName),
	}
}

func getEnvReplacementMapForAccount(name, passwd string) map[string]string {
	return map[string]string{
		"$(USERNAME)": name,
		"$(PASSWD)":   passwd,
	}
}

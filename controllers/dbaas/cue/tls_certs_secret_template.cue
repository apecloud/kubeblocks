// Copyright ApeCloud, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

pathedName: {
	namespace:			string
	clusterName:		string
	componentName:	string
}
secret: {
	apiVersion: "v1"
	kind:       "Secret"
	metadata: {
		name:      "\(pathedName.clusterName)-\(pathedName.componentName)-tls-certs"
		namespace: pathedName.namespace
		labels: {
			"app.kubernetes.io/instance":   pathedName.clusterName
			"app.kubernetes.io/managed-by": "kubeblocks"
		}
	}
	type: kubernetes.io/tls
	stringData: {}
}

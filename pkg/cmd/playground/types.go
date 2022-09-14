/*
Copyright © 2022 The dbctl Authors

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

package playground

import "github.com/apecloud/kubeblocks/version"

const (
	DBClusterName        = "mycluster"
	ClusterName          = "dbctl-playground"
	ClusterNamespace     = "dbctl-playground"
	DBClusterNamespace   = "default"
	DefaultEngine        = "wesql"
	DefaultCloudProvider = "local"
	DefaultVersion       = "8.0.28"
	DefaultRootUser      = "root"
	DefaultPassword      = "sakila"
	DefaultPort          = "3306"
	LocalHost            = "127.0.0.1"
	DefaultS3Endpoint    = "https://s3.cn-north-1.amazonaws.com.cn"
	DefaultS3Region      = "cn-north-1"

	helmUser   = "yimeisun"
	helmPasswd = "8V+PmX1oSDv4pumDvZp6m7LS8iPgbY3A"
	helmURL    = "yimeisun.azurecr.io"

	// K3sKubeConfigLocation is default path of k3s kubeconfig
	K3sKubeConfigLocation = "/etc/rancher/k3s/k3s.yaml"
	// K3sExternalKubeConfigLocation is where to generate kubeconfig for external access
	K3sExternalKubeConfigLocation = "/etc/rancher/k3s/k3s-external.yaml"
	// CliDockerNetwork is docker network for k3d cluster when `dbctl playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-dbctl-playground"
)

var (
	// K3sImage is k3s image repo
	K3sImage = "rancher/k3s:" + version.K3sImageTag
	// K3dToolsImage is k3d tools image repo
	K3dToolsImage = "docker.io/infracreate/k3d-tools:" + version.K3dVersion
	// K3dProxyImage is k3d proxy image repo
	K3dProxyImage = "docker.io/infracreate/k3d-proxy:" + version.K3dVersion
)

var kubeConfig = `
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://${KUBERNETES_API_SERVER_ADDRESS}:6444
  name: k3d-dbctl-playground
contexts:
- context:
    cluster: k3d-dbctl-playground
    user: admin@k3d-dbctl-playground
  name: k3d-dbctl-playground
current-context: k3d-dbctl-playground
kind: Config
preferences: {}
users:
- name: admin@k3d-dbctl-playground
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrRENDQVRlZ0F3SUJBZ0lJR1dEc0wyWmFtWjB3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOalU1TlRnek1EUTBNQjRYRFRJeU1EZ3dOREF6TVRjeU5Gb1hEVEl6TURndwpOREF6TVRjeU5Gb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJKbkxHR1FNUmZva2srWDcKSS9HNWRSbG5sUzYwODlqWGV3Q0l1OGVvNmc5bUVlU203NWRmdzc2R2IrZ29BbXFXK244MkNqRVd1QTNrSEQyeQpQTUxSS2JhalNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCU1Fhd1VYVEZjMzVCdWJkQkdrK3ExZXZ4VW5SVEFLQmdncWhrak9QUVFEQWdOSEFEQkUKQWlBVXl0dWxOQzVVbnRCcmlvOGlhd1gxUUdjTEVxUENPWk04VmFETXozMTBoUUlnTWIxSHJGa3JXUHFWSTVvQgpBdyttN2szK0I5SzBWem1mcTJtSmx3V2pNdmM9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUyTlRrMU9ETXdORFF3SGhjTk1qSXdPREEwTURNeE56STBXaGNOTXpJd09EQXhNRE14TnpJMApXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUyTlRrMU9ETXdORFF3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFRUWF0NDNGSFl0ZlpyT2YreHZwaFhacUEvaEFSTUhFd2JBcDBGSVdzTUcKMmlGVnZCbThBWE9MUWxYY0VKSW5EVmppZjFZYkFISWhiYVl2WjY4NXk0SzNvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWtHc0ZGMHhYTitRYm0zUVJwUHF0ClhyOFZKMFV3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQUxZUU1qMkRqbnNRd2lKUGd0UlE3d3VDN1piMDd1VzEKZXU2SDhoaFBCN2l4QWlCbkJmQlU3M3BkSWFCdVBxNGR2TGw1MDloTWNtU1FXTVo4VVpoV1lPS0FNUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUM1VUgzOC91VXJVQWJZbENnSTZmU25kTEhVUi9lNFJ4L3JQNkdUMUNoeXRvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFbWNzWVpBeEYraVNUNWZzajhibDFHV2VWTHJUejJOZDdBSWk3eDZqcUQyWVI1S2J2bDEvRAp2b1p2NkNnQ2FwYjZmellLTVJhNERlUWNQYkk4d3RFcHRnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
`

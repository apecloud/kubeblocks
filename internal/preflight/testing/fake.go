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

package testing

import (
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

const (
	clusterVersion = `
	{
	  "info": {
		"major": "1",
		"minor": "23",
		"gitVersion": "v1.23.15",
		"gitCommit": "b84cb8ab29366daa1bba65bc67f54de2f6c34848",
		"gitTreeState": "clean",
		"buildDate": "2022-12-08T10:42:57Z",
		"goVersion": "go1.17.13",
		"compiler": "gc",
		"platform": "linux/arm64"
	  },
	  "string": "v1.23.15"
	}`
	deploymentStatus = `
	{
	  "string": "1"
	}`
)

func FakeAnalyzers() []*troubleshoot.Analyze {
	return []*troubleshoot.Analyze{
		{
			ClusterVersion: &troubleshoot.ClusterVersion{
				AnalyzeMeta: troubleshoot.AnalyzeMeta{
					CheckName: "ClusterVersionCheck",
				},
				Outcomes: []*troubleshoot.Outcome{
					{
						Pass: &troubleshoot.SingleOutcome{
							Message: "version is ok.",
						},
					},
				},
			},
		},
		{
			DeploymentStatus: &troubleshoot.DeploymentStatus{
				AnalyzeMeta: troubleshoot.AnalyzeMeta{
					CheckName: "DeploymentStatusCheck",
				},
				Outcomes: []*troubleshoot.Outcome{
					{
						Warn: &troubleshoot.SingleOutcome{
							When:    "absent",
							Message: "The deployment is not present.",
						},
						Pass: &troubleshoot.SingleOutcome{
							Message: "There are multiple replicas of the deployment ready",
						},
					},
				},
			},
		},
	}
}

func FakeCollectedData() map[string][]byte {
	return map[string][]byte{
		"cluster-info/cluster_version.json":   []byte(clusterVersion),
		"cluster-info/deployment_status.json": []byte(deploymentStatus),
	}
}

func FakeKbPreflight() *preflightv1beta2.Preflight {
	var collectList []*troubleshoot.Collect
	collect := &troubleshoot.Collect{
		ClusterInfo:      &troubleshoot.ClusterInfo{},
		ClusterResources: &troubleshoot.ClusterResources{},
	}
	collectList = append(collectList, collect)

	return &preflightv1beta2.Preflight{
		Spec: preflightv1beta2.PreflightSpec{
			PreflightSpec: troubleshoot.PreflightSpec{
				Collectors: collectList,
			},
		},
	}
}

func FakeKbHostPreflight() *preflightv1beta2.HostPreflight {
	var extendCollectList []*preflightv1beta2.ExtendHostCollect
	var remoteCollectList []*troubleshoot.RemoteCollect
	extendCollectList = append(
		extendCollectList,
		&preflightv1beta2.ExtendHostCollect{HostUtility: &preflightv1beta2.HostUtility{}},
	)
	remoteCollectList = append(remoteCollectList, &troubleshoot.RemoteCollect{CPU: &troubleshoot.RemoteCPU{}})

	hostPreflight := &preflightv1beta2.HostPreflight{
		Spec: preflightv1beta2.HostPreflightSpec{
			ExtendCollectors: extendCollectList,
		},
	}
	hostPreflight.Spec.RemoteCollectors = remoteCollectList
	return hostPreflight
}

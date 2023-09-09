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

package types

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
)

// APIConfig contains options relevant to connecting to the K8s API
type APIConfig struct {
	// How to authenticate to the K8s API server.  This can be one of `none`
	// (for no auth), `serviceAccount` (to use the standard service account
	// token provided to the agent pod), or `kubeConfig` to use credentials
	// from `~/.kube/config`.
	AuthType AuthType `json:"authType"`
}

// AuthType describes the type of authentication to use for the K8s API
type AuthType string

const (
	// AuthTypeNone means no auth is required
	AuthTypeNone AuthType = "none"
	// AuthTypeServiceAccount means to use the built-in service account that
	// K8s automatically provisions for each pod.
	AuthTypeServiceAccount AuthType = "serviceAccount"
	// AuthTypeKubeConfig uses local credentials like those used by kubectl.
	AuthTypeKubeConfig AuthType = "kubeConfig"
	// AuthTypeTLS indicates that client TLS auth is desired
	AuthTypeTLS AuthType = "tls"
)

type KubeletStateConfig struct {
	Enabled bool `json:"enabled"`
	// MetricGroups are the groups of metrics to collect
	MetricGroups []string `json:"metricGroups"`
}

type K8sNodeConfig struct {
	Enabled bool `json:"enabled"`
}

type K8sClusterConfig struct {
	Enabled bool `json:"enabled"`
}

type MetricsDatasource struct {

	// KubeletStateConfig is the configuration to scrape metrics from Kubelet
	KubeletStateConfig *KubeletStateConfig `json:"kubeletState"`

	// K8sNodeConfig is the configuration to scrape metrics from K8s node
	K8sNodeConfig *K8sNodeConfig `json:"k8sNode"`

	// K8sClusterConfig is the configuration to scrape metrics from K8s cluster
	K8sClusterConfig *K8sClusterConfig `json:"k8sCluster"`

	// CollectionInterval is the metrics collection interval
	CollectionInterval *string `json:"collectionInterval"`

	// ExporterRef is the exporters to export metrics
	v1alpha1.ExporterRef `json:",inline"`
}

type PodsLogsConfig struct{}

type LogsDatasource struct {

	// PodsLogsConfig is the configuration to scrape logs from pods
	PodsLogsConfig *PodsLogsConfig `json:"podsLogs"`

	// ExporterRef is the exporters to export logs
	v1alpha1.ExporterRef `json:",inline"`
}

type Datasource struct {
	// MetricsDatasource defines the metrics to be scraped
	MetricsDatasource *MetricsDatasource `json:"metrics"`

	// LogsDatasource defines the logs to be scraped
	LogDatasource *LogsDatasource `json:"logs"`
}

type Config struct {
	// APIConfig is the authentication method used to connect to Kubelet
	APIConfig `json:",inline"`

	// CollectionInterval is the metrics collection interval
	CollectionInterval string `json:"collectionInterval"`

	// Datasource is the metrics and logs to be scraped
	Datasource Datasource `json:"datasource"`

	// Image is the image of the oteld
	Image string `json:"image,omitempty"`

	// Resources is the resource requirements for the oteld
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// UseConfigMap indicates whether to use configmap to store oteld config
	UseConfigMap bool `json:"useConfigMap"`
}

func LoadConfig(configFile string) (*Config, error) {
	config := &Config{}
	if err := cfgutil.FromYamlConfig(configFile, config); err != nil {
		return nil, err
	}
	return config, nil
}

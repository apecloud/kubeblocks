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

package config

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
)

var (
	HostIP                  string
	MaxENI                  int
	MinPrivateIP            int
	EnableDebug             bool
	RPCPort                 int
	CleanLeakedENIInterval  time.Duration
	ENIReconcileInterval    time.Duration
	RefreshNodeInterval     time.Duration
	TrafficNodeLabels       map[string]string
	EndpointsLabels         map[string]string
	ServiceLabels           map[string]string
	MaxConcurrentReconciles int
	CloudProvider           string
)

const (
	EnvHostIP                  = "HOST_IP"
	EnvMaxENI                  = "MAX_ENI"
	EnvMinPrivateIP            = "MIN_PRIVATE_IP"
	EnvEnableDebug             = "ENABLE_DEBUG"
	EnvRPCPort                 = "RPC_PORT"
	EnvENIReconcileInterval    = "ENI_RECONCILE_INTERVAL"
	EnvCleanLeakedENIInterval  = "CLEAN_LEAKED_ENI_INTERVAL"
	EnvRefreshNodes            = "REFRESH_NODES_INTERVAL"
	EnvTrafficNodeLabels       = "TRAFFIC_NODE_LABELS"
	EnvEndpointsLabels         = "ENDPOINTS_LABELS"
	EnvServiceLabels           = "SERVICE_LABELS"
	EnvMaxConcurrentReconciles = "MAX_CONCURRENT_RECONCILES"
	EnvCloudProvider           = "CLOUD_PROVIDER"
)

func init() {
	_ = viper.BindEnv(EnvHostIP)

	_ = viper.BindEnv(EnvMaxENI)
	viper.SetDefault(EnvMaxENI, -1)

	_ = viper.BindEnv(EnvMinPrivateIP)
	viper.SetDefault(EnvMinPrivateIP, 1)

	_ = viper.BindEnv(EnvENIReconcileInterval)
	viper.SetDefault(EnvENIReconcileInterval, 15)

	_ = viper.BindEnv(EnvCleanLeakedENIInterval)
	viper.SetDefault(EnvCleanLeakedENIInterval, 60)

	_ = viper.BindEnv(EnvRefreshNodes)
	viper.SetDefault(EnvRefreshNodes, 15)

	_ = viper.BindEnv(EnvRPCPort)
	viper.SetDefault(EnvRPCPort, 19200)

	_ = viper.BindEnv(EnvEnableDebug)
	viper.SetDefault(EnvEnableDebug, false)

	_ = viper.BindEnv(EnvTrafficNodeLabels)
	viper.SetDefault(EnvTrafficNodeLabels, "")

	_ = viper.BindEnv(EnvEndpointsLabels)
	viper.SetDefault(EnvEndpointsLabels, "")

	_ = viper.BindEnv(EnvServiceLabels)
	viper.SetDefault(EnvServiceLabels, "")

	_ = viper.BindEnv(EnvMaxConcurrentReconciles)
	viper.SetDefault(EnvMaxConcurrentReconciles, runtime.NumCPU()*2)

	_ = viper.BindEnv(EnvCloudProvider)
}

func ReadConfig(logger logr.Logger) {
	err := viper.ReadInConfig() // Find and read the config file
	if err == nil {             // Handle errors reading the config file
		logger.Info(fmt.Sprintf("config file: %s", viper.GetViper().ConfigFileUsed()))
	}

	HostIP = viper.GetString(EnvHostIP)
	MaxENI = viper.GetInt(EnvMaxENI)
	MinPrivateIP = viper.GetInt(EnvMinPrivateIP)
	EnableDebug = viper.GetBool(EnvEnableDebug)
	RPCPort = viper.GetInt(EnvRPCPort)
	ENIReconcileInterval = time.Duration(viper.GetInt(EnvENIReconcileInterval)) * time.Second
	CleanLeakedENIInterval = time.Duration(viper.GetInt(EnvCleanLeakedENIInterval)) * time.Second
	RefreshNodeInterval = time.Duration(viper.GetInt(EnvRefreshNodes)) * time.Second
	TrafficNodeLabels = ParseLabels(viper.GetString(EnvTrafficNodeLabels))
	EndpointsLabels = ParseLabels(viper.GetString(EnvEndpointsLabels))
	ServiceLabels = ParseLabels(viper.GetString(EnvServiceLabels))
	MaxConcurrentReconciles = viper.GetInt(EnvMaxConcurrentReconciles)
	CloudProvider = viper.GetString(EnvCloudProvider)
}

func ParseLabels(labels string) map[string]string {
	result := make(map[string]string)
	for _, label := range strings.Split(labels, ",") {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		parts := strings.SplitN(label, ":", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

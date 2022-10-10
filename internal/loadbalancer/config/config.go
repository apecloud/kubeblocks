package config

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
)

var (
	HostIP                 string
	MaxENI                 int
	MinPrivateIP           int
	EnableDebug            bool
	RPCPort                int
	CleanLeakedENIInterval time.Duration
	ENIReconcileInterval   time.Duration
)

const (
	EnvHostIP                 = "HOST_IP"
	EnvMaxENI                 = "MAX_ENI"
	EnvMinPrivateIP           = "MIN_PRIVATE_IP"
	EnvEnableDebug            = "ENABLE_DEBUG"
	EnvRPCPort                = "RPC_PORT"
	EnvENIReconcileInterval   = "ENI_RECONCILE_INTERVAL"
	EnvCleanLeakedENIInterval = "CLEAN_LEAKED_ENI_INTERVAL"
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

	_ = viper.BindEnv(EnvRPCPort)
	viper.SetDefault(EnvRPCPort, 19200)

	_ = viper.BindEnv(EnvEnableDebug)
	viper.SetDefault(EnvEnableDebug, false)
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
}

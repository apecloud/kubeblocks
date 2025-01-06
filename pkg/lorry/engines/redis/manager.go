/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package redis

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	redisUser   = "default"
	redisPasswd = ""
)

type Manager struct {
	engines.DBManagerBase
	client         redis.UniversalClient
	clientSettings *Settings
	sentinelClient *redis.SentinelClient

	role                    string
	roleSubscribeUpdateTime int64
	roleProbePeriod         int64
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("Redis")

	if viper.IsSet(constant.KBEnvServiceUser) {
		redisUser = viper.GetString(constant.KBEnvServiceUser)
	}

	if viper.IsSet(constant.KBEnvServicePassword) {
		redisPasswd = viper.GetString(constant.KBEnvServicePassword)
	}

	if viper.IsSet(constant.KBEnvServicePort) {
		properties["redisHost"] = fmt.Sprintf("127.0.0.1:%s", viper.GetString(constant.KBEnvServicePort))
	}

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}
	mgr := &Manager{
		DBManagerBase:   *managerBase,
		roleProbePeriod: int64(viper.GetInt(constant.KBEnvRoleProbePeriod)),
	}

	maJorVersion, err := getRedisMajorVersion()
	if err != nil {
		return nil, err
	}
	// The username is supported after 6.0
	if maJorVersion < 6 {
		redisUser = ""
	}

	defaultSettings := &Settings{
		Password: redisPasswd,
		Username: redisUser,
	}
	mgr.client, mgr.clientSettings, err = ParseClientFromProperties(properties, defaultSettings)
	if err != nil {
		return nil, err
	}

	mgr.sentinelClient = newSentinelClient(mgr.clientSettings, mgr.ClusterCompName)
	if mgr.sentinelClient != nil {
		go mgr.SubscribeRoleChange(context.Background())
	}

	return mgr, nil
}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if _, err := mgr.client.Ping(ctx).Result(); err != nil {
		mgr.Logger.Info("connecting to redis failed", "host", mgr.clientSettings.Host, "error", err)
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Info("DB startup ready")
	return true
}

func tokenizeCmd2Args(cmd string) []interface{} {
	args := strings.Split(cmd, " ")
	redisArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		redisArgs = append(redisArgs, arg)
	}
	return redisArgs
}

// we get redis version from 'redis-cli --version'
func getRedisMajorVersion() (int, error) {
	redisCliCMD, err := exec.LookPath("redis-cli")
	if err != nil {
		return -1, err
	}

	out, err := exec.Command(redisCliCMD, "--version").Output()
	if err != nil {
		return -1, err
	}

	// output eg: redis-cli 7.2.4
	versionInfo := strings.Split(strings.TrimSpace(string(out)), " ")
	if len(versionInfo) < 2 {
		return -1, fmt.Errorf("invalid redis version info: %s", string(out))
	}

	majorVersion, err := strconv.Atoi(strings.Split(strings.TrimSpace(versionInfo[1]), ".")[0])
	if err != nil {
		return -1, err
	}
	return majorVersion, nil
}

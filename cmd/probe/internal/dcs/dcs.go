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

package dcs

import (
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/internal/constant"
)

type DCS interface {
	Initialize() error

	// cluster manage functions
	GetClusterName() string
	GetCluster() (*Cluster, error)
	ResetCluser()
	DeleteCluser()

	// cluster scole ha config
	GetHaConfig() (*HaConfig, error)

	// member manager funtions
	GetMembers() ([]Member, error)
	AddCurrentMember() error

	// manual switchover
	GetSwitchover() (*Switchover, error)
	CreateSwitchover(string, string) error
	DeleteSwitchover() error

	// cluster scope leader lock
	AttempAcquireLock() error
	CreateLock() error
	IsLockExist() (bool, error)
	HasLock() bool
	ReleaseLock() error
	UpdateLock() error

	GetLeader() (*Leader, error)
}

var dcs DCS

func init() {
	viper.SetDefault("KB_TTL", 5)
	viper.SetDefault(constant.KubernetesClusterDomainEnv, constant.DefaultDNSDomain)
}

func GetStore() DCS {
	return dcs
}

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

package models

type EngineType string

const (
	MySQL              EngineType = "mysql"
	WeSQL              EngineType = "wesql"
	PostgreSQL         EngineType = "postgresql"
	OfficialPostgreSQL EngineType = "official-postgresql"
	ApecloudPostgreSQL EngineType = "apecloud-postgresql"
	Redis              EngineType = "redis"
	ETCD               EngineType = "etcd"
	MongoDB            EngineType = "mongodb"
	Nebula             EngineType = "nebula"
	PolarDBX           EngineType = "polardbx"
	PulsarBroker       EngineType = "pulsar-broker"
	PulsarProxy        EngineType = "pulsar-proxy"
	FoxLake            EngineType = "foxlake"
	Oceanbase          EngineType = "oceanbase"
	Oracle             EngineType = "oracle"
	OpenGauss          EngineType = "opengauss"
	Custom             EngineType = "custom"
)

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

package breakingchange

const (
	componentPostgresql = "postgresql"
	componentRedis      = "redis"
	componentMysql      = "mysql"
	componentMongodb    = "mongodb"

	// data volume name
	dataVolumeName = "data"

	// data mount path
	mysqlMountPath   = "/data/mysql"
	mongodbMountPath = "/data/mongodb"
	pgsqlMountPath   = "/home/postgres/pgdata"
	redisMountPath   = "/data"
)

// Version Scope: [0.5, 0.6]
const (
	backupTypeDatafile = "datafile"
	backupTypeSnapshot = "snapshot"
	backupTypeLogfile  = "logfile"
)

// Version Scope: [0.7]
const (

	// backup method
	volumeSnapshotMethodName = "volume-snapshot"
	pgbasebackupMethodName   = "pg-basebackup"
	xtrabackupMethodName     = "xtrabackup"
	datafileMethodName       = "datafile"

	// action set name
	pgBasebackupActionSet  = "postgres-basebackup"
	xtrabackupActionSet    = "xtrabackup-for-apecloud-mysql"
	volumeSnapshotForMysql = "volumesnapshot-for-apecloud-mysql"
	redisDatafileActionSet = "redis-physical-backup"
	volumeSnapshotForMongo = "mongodb-volumesnapshot"
	mongoDatafileActionSet = "mongodb-physical-backup"
)

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

package dataprotection

const (
	ClusterName   = "test-cluster"
	ComponentName = "test-comp"
	ContainerName = "test-container"

	BackupName         = "test-backup"
	BackupRepoName     = "test-repo"
	BackupPolicyName   = "test-backup-policy"
	BackupMethodName   = "xtrabackup"
	VSBackupMethodName = "volume-snapshot"
	BackupPathPrefix   = "/backup"
	ActionSetName      = "xtrabackup"
	VSActionSetName    = "volume-snapshot"

	DataVolumeName      = "data"
	DataVolumeMountPath = "/data"
	LogVolumeName       = "log"
	LogVolumeMountPath  = "/log"

	StorageProviderName = "test-sp"
	StorageClassName    = "test-sc"

	BackupScheduleName      = "test-backup-schedule"
	BackupScheduleCron      = "0 3 * * *"
	BackupRetention         = "7d"
	StartingDeadlineMinutes = 10

	KBToolImage   = "apecloud/kubeblocks-tool:latest"
	BackupPVCName = "test-backup-pvc"
	ImageTag      = "latest"
)

// Restore
const (
	RestoreName       = "test-restore"
	MysqlTemplateName = "data-mysql-mysql"
)

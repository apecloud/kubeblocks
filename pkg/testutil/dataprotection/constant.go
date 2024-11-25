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

package dataprotection

import dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

const (
	ClusterName   = "test-cluster"
	ComponentName = "test-comp"
	ContainerName = "test-container"
	PortName      = "test-port"
	ProtocolName  = "TCP"
	PortNum       = 10000

	BackupName         = "test-backup"
	BackupRepoName     = "test-repo"
	BackupPolicyName   = "test-backup-policy"
	BackupMethodName   = "xtrabackup"
	VSBackupMethodName = "volume-snapshot"
	BackupPathPrefix   = "/backup"
	ActionSetName      = "xtrabackup"

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

	KBToolImage         = "apecloud/kubeblocks-tool:latest"
	BackupPVCName       = "test-backup-pvc"
	ImageTag            = "latest"
	BackupPolicyTPLName = "test-backup-policy-template-mysql"
)

// Restore
const (
	RestoreName       = "test-restore"
	MysqlTemplateName = "data-mysql-mysql"
)

const (
	InvalidParameter    = "invalid"
	ParameterString     = "testString"
	ParameterStringType = "string"
	ParameterArray      = "testArray"
	ParameterArrayType  = "array"
)

var (
	TestParameters = []dpv1alpha1.ParameterPair{
		{
			Name:  ParameterString,
			Value: "stringValue",
		},
		{
			Name:  ParameterArray,
			Value: "v1,v2",
		},
	}
	InvalidParameters = []dpv1alpha1.ParameterPair{
		{
			Name:  "invalid",
			Value: "invalid",
		},
	}
)

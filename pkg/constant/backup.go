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

package constant

const (
	// BackupRetain always retained, unless manually deleted by the user
	BackupRetain = "Retain"

	// BackupRetainUntilExpired retains backup till it expires
	BackupRetainUntilExpired = "RetainUntilExpired"

	// BackupDelete (default) deletes backup immediately when cluster's terminationPolicy is WipeOut
	BackupDelete = "Delete"
)

const (
	BackupNameKeyForRestore           = "name"
	BackupNamespaceKeyForRestore      = "namespace"
	VolumeRestorePolicyKeyForRestore  = "volumeRestorePolicy"
	DoReadyRestoreAfterClusterRunning = "doReadyRestoreAfterClusterRunning"
	RestoreTimeKeyForRestore          = "restoreTime"
	ConnectionPassword                = "connectionPassword"
	EncryptedSystemAccounts           = "encryptedSystemAccounts"
)

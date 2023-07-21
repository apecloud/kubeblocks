// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

options: {
	backupName:      string
	containerName:   string
	namespace:       string
	image:           string
	imagePullPolicy: string
}

container: {
	image:           options.image
	name:            options.containerName
	imagePullPolicy: options.imagePullPolicy
	command: ["sh", "-c"]
	args: [
		"""
retryTimes=0
oldBackupInfo=
trap "echo 'Terminating...' && exit" TERM
while true; do
  sleep 3;
  if [ ! -f ${BACKUP_INFO_FILE} ]; then
    continue
  fi
  backupInfo=$(cat ${BACKUP_INFO_FILE})
  if [ "${oldBackupInfo}" == "${backupInfo}" ]; then
    continue
  fi
  echo "start to patch backupInfo: ${backupInfo}"
  eval kubectl -n \(options.namespace) patch backup \(options.backupName) --subresource=status --type=merge --patch '{\\\"status\\\":${backupInfo}}'
  if [ $? -ne 0 ]; then
    retryTimes=$(($retryTimes+1))
  else
    echo "update backup status successfully"
    retryTimes=0
    oldBackupInfo=${backupInfo}
  fi
  if [ $retryTimes -ge 3 ]; then
    echo "ERROR: update backup status failed, 3 attempts have been made!"
    exit 1
  fi
done
""",
	]
}

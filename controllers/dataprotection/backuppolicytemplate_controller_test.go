/*
Copyright 2022 The KubeBlocks Authors

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
package dataprotection

import (
	"context"
	"time"

	"github.com/sethvargo/go-password/password"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("BackupPolicyTemplate Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	checkedCreateObj := func(obj client.Object) error {
		if err := k8sClient.Create(context.Background(), obj); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}

	genarateNS := func(prefix string) types.NamespacedName {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      prefix + randomStr,
			Namespace: "default",
		}
		return key
	}

	assureBackupPolicyTemplateObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicyTemplate {
		By("By assure an backupPolicyTemplate obj")
		backupPolicyYaml := `
apiVersion: dataprotection.infracreate.com/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: backup-policy-template-demo
  namespace: default
spec:
  schedule: "0 3 * * *"
  ttl: 168h0m0s
  backupToolName: xtrabackup-mysql
  onFailAttempted: 3
`
		backupPolicyTemp := &dataprotectionv1alpha1.BackupPolicyTemplate{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicyTemp)).Should(Succeed())
		ns := genarateNS("backup-policy-template-")
		backupPolicyTemp.Name = ns.Name
		backupPolicyTemp.Namespace = ns.Namespace
		backupPolicyTemp.Spec.BackupToolName = backupTool
		Expect(checkedCreateObj(backupPolicyTemp)).Should(Succeed())
		return backupPolicyTemp
	}

	deleteBackupPolicyTemplateNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupPolicyTemplate{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupPolicyTemplate{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dataprotectionv1alpha1.BackupPolicyTemplate{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupToolObj := func() *dataprotectionv1alpha1.BackupTool {
		By("By assure an backupTool obj")
		backupToolYaml := `
apiVersion: dataprotection.infracreate.com/v1alpha1
kind: BackupTool
metadata:
  name: xtrabackup-mysql
  namespace: default
spec:
  image: percona/percona-xtrabackup
  databaseEngine: mysql
  deployKind: job
  resources:
    limits:
      cpu: "1"
      memory: 2Gi

  env:
    - name: DATA_DIR
      value: /var/lib/mysql

    - name: BACKUP_DIR_PREFIX
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace

    - name: BACKUP_DIR
      value: /data/$(BACKUP_DIR_PREFIX)

  restoreCommands:
    - |
      echo "BACKUP_DIR=${BACKUP_DIR} BACKUP_NAME=${BACKUP_NAME} DATA_DIR=${DATA_DIR}" && \
      mkdir -p /tmp/data/ && cd /tmp/data \
      && xbstream -x < /${BACKUP_DIR}/${BACKUP_NAME}.xbstream \
      && xtrabackup --decompress  --target-dir=/tmp/data/ \
      && find . -name "*.qp"|xargs rm -f \
      && rm -rf ${DATA_DIR}/* \
      && rsync -avrP /tmp/data/ ${DATA_DIR}/ \
      && rm -rf /tmp/data/ \
      && chmod -R 0777 ${DATA_DIR}

  # Optional
  incrementalRestoreCommands: []
  backupCommands:
    - echo "DB_HOST=${DB_HOST} DB_USER=${DB_USER} DB_PASSWORD=${DB_PASSWORD} DATA_DIR=${DATA_DIR} BACKUP_DIR=${BACKUP_DIR} BACKUP_NAME=${BACKUP_NAME}";
      mkdir -p /${BACKUP_DIR};
      xtrabackup --compress --backup  --safe-slave-backup --slave-info --stream=xbstream --host=${DB_HOST} --user=${DB_USER} --password=${DB_PASSWORD} --datadir=${DATA_DIR} > /${BACKUP_DIR}/${BACKUP_NAME}.xbstream
  # Optional
  incrementalBackupCommands: []
`
		backupTool := &dataprotectionv1alpha1.BackupTool{}
		Expect(yaml.Unmarshal([]byte(backupToolYaml), backupTool)).Should(Succeed())
		ns := genarateNS("backup-tool-")
		backupTool.Name = ns.Name
		backupTool.Namespace = ns.Namespace
		Expect(checkedCreateObj(backupTool)).Should(Succeed())
		return backupTool
	}

	deleteBackupToolNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupTool{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupTool{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dataprotectionv1alpha1.BackupTool{}
		}
		return client.IgnoreNotFound(err)
	}

	Context("When creating backupPolicyTemplate", func() {
		It("Should success with no error", func() {

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicyTemplate from backupTool: " + backupTool.Name)
			toCreate := assureBackupPolicyTemplateObj(backupTool.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}
			time.Sleep(waitDuration)
			result := &dataprotectionv1alpha1.BackupPolicyTemplate{}
			Expect(k8sClient.Get(context.Background(), key, result)).Should(Succeed())

			By("Deleting the scope")

			Eventually(func() error {
				_ = deleteBackupToolNWait(key)
				return deleteBackupPolicyTemplateNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

})

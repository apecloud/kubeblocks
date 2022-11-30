/*
Copyright ApeCloud Inc.

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
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("RestoreJob Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.RestoreJob{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupJob{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupTool{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupPolicyTemplate{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	genarateNS := func(prefix string) types.NamespacedName {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      prefix + randomStr,
			Namespace: testCtx.DefaultNamespace,
		}
		return key
	}

	assureRestoreJobObj := func(backupJob string) *dataprotectionv1alpha1.RestoreJob {
		By("By assure an restoreJob obj")
		restoreJobYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: RestoreJob
metadata:
  name: restore-demo
spec:
  backupJobName: backup-success-demo
  target:
    databaseEngine: mysql
    labelsSelector:
      matchLabels:
        mysql.oracle.com/cluster: mycluster
    secret:
      name: mycluster-cluster-secret
  targetVolumes:
    - name: mysql-restore-storage
      persistentVolumeClaim:
        claimName: datadir-mycluster-0
  targetVolumeMounts:
    - name: mysql-restore-storage
      mountPath: /var/lib/mysql
  onFailAttempted: 3
`
		restoreJob := &dataprotectionv1alpha1.RestoreJob{}
		Expect(yaml.Unmarshal([]byte(restoreJobYaml), restoreJob)).Should(Succeed())
		ns := genarateNS("restore-job-")
		restoreJob.Name = ns.Name
		restoreJob.Namespace = ns.Namespace
		restoreJob.Spec.BackupJobName = backupJob

		Expect(testCtx.CheckedCreateObj(ctx, restoreJob)).Should(Succeed())
		return restoreJob
	}

	deleteRestoreJobWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.RestoreJob{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.RestoreJob{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.RestoreJob{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupJobObj := func(backupPolicy string) *dataprotectionv1alpha1.BackupJob {
		By("By assure an backupJob obj")
		backupJobYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupJob
metadata:
  name: backup-success-demo
  namespace: default

  labels:
    dataprotection.kubeblocks.io/backup-type: full
    db.kubeblocks.io/name: mysqlcluster
    dataprotection.kubeblocks.io/backup-policy-name: backup-policy-demo
    dataprotection.kubeblocks.io/backup-index: "0"

spec:
  backupPolicyName: backup-policy-demo
  backupType: full
  ttl: 168h0m0s
status:
  phase: Completed
`
		backupJob := &dataprotectionv1alpha1.BackupJob{}
		Expect(yaml.Unmarshal([]byte(backupJobYaml), backupJob)).Should(Succeed())
		ns := genarateNS("backup-job-")
		backupJob.Name = ns.Name
		backupJob.Namespace = ns.Namespace
		backupJob.Spec.BackupPolicyName = backupPolicy

		Expect(testCtx.CheckedCreateObj(ctx, backupJob)).Should(Succeed())
		return backupJob
	}

	deleteBackupJobWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupJob{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupJob{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.BackupJob{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupPolicyObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		backupPolicyYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicy
metadata:
  name: backup-policy-demo
  namespace: default
spec:
  schedule: "0 3 * * *"
  ttl: 168h0m0s
  backupToolName: xtrabackup-mysql
  backupPolicyTemplateName: backup-config-mysql
  target:
    databaseEngine: mysql
    labelsSelector:
      matchLabels:
        mysql.oracle.com/cluster: mycluster
    secret:
      name: mycluster-cluster-secret
  targetVolume:
    name: mysql-persistent-storage
    persistentVolumeClaim:
      claimName: datadir-mycluster-0
  remoteVolumes:
    - name: backup-remote-volume
      persistentVolumeClaim:
        claimName: backup-host-path-pvc
  onFailAttempted: 3
`
		backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicy)).Should(Succeed())
		ns := genarateNS("backup-policy-")
		backupPolicy.Name = ns.Name
		backupPolicy.Namespace = ns.Namespace
		backupPolicy.Spec.BackupToolName = backupTool
		Expect(testCtx.CheckedCreateObj(ctx, backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	deleteBackupPolicyWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupPolicy{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupPolicy{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.BackupPolicy{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupToolObj := func() *dataprotectionv1alpha1.BackupTool {
		By("By assure an backupTool obj")
		backupToolYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
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
		Expect(testCtx.CheckedCreateObj(ctx, backupTool)).Should(Succeed())
		return backupTool
	}

	deleteBackupToolWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupTool{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupTool{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dataprotectionv1alpha1.BackupTool{}
		}
		return client.IgnoreNotFound(err)
	}

	assureStatefulSetObj := func() *appv1.StatefulSet {
		By("By assure an stateful obj")
		statefulYaml := `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  generation: 1
  labels:
    mysql.oracle.com/cluster: mycluster
  name: mycluster
  namespace: default
spec:
  podManagementPolicy: Parallel
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      mysql.oracle.com/cluster: mycluster
  serviceName: mycluster-instances
  template:
    metadata:
      labels:
        mysql.oracle.com/cluster: mycluster
    spec:
      containers:
      - command:
        - mysqlsh
        - --pym
        - mysqloperator
        - sidecar
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: MYSQL_UNIX_PORT
          value: /var/run/mysqld/mysql.sock
        - name: MYSQLSH_USER_CONFIG_HOME
          value: /mysqlsh
        image: mysql/mysql-operator:8.0.30-2.0.6
        imagePullPolicy: IfNotPresent
        name: sidecar
        resources: {}
        securityContext:
          runAsUser: 27
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/run/mysqld
          name: rundir
        - mountPath: /etc/my.cnf.d
          name: mycnfdata
          subPath: my.cnf.d
        - mountPath: /etc/my.cnf
          name: mycnfdata
          subPath: my.cnf
        - mountPath: /mysqlsh
          name: shellhome
      - args:
        - mysqld
        - --user=mysql
        env:
        - name: MYSQL_UNIX_PORT
          value: /var/run/mysqld/mysql.sock
        image: mysql/mysql-server:8.0.28
        imagePullPolicy: IfNotPresent
        lifecycle:
          preStop:
            exec:
              command:
              - sh
              - -c
              - sleep 20 && mysqladmin -ulocalroot shutdown
        livenessProbe:
          exec:
            command:
            - /livenessprobe.sh
          failureThreshold: 10
          initialDelaySeconds: 15
          periodSeconds: 15
          successThreshold: 1
          timeoutSeconds: 1
        name: mysql
        ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        - containerPort: 33060
          name: mysqlx
          protocol: TCP
        - containerPort: 33061
          name: gr-xcom
          protocol: TCP
        readinessProbe:
          exec:
            command:
            - /readinessprobe.sh
          failureThreshold: 10000
          initialDelaySeconds: 10
          periodSeconds: 5
          successThreshold: 1
          timeoutSeconds: 1
        resources: {}
        startupProbe:
          exec:
            command:
            - /livenessprobe.sh
            - "8"
          failureThreshold: 10000
          initialDelaySeconds: 5
          periodSeconds: 3
          successThreshold: 1
          timeoutSeconds: 1
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: datadir
        - mountPath: /var/run/mysqld
          name: rundir
        - mountPath: /etc/my.cnf.d
          name: mycnfdata
          subPath: my.cnf.d
        - mountPath: /etc/my.cnf
          name: mycnfdata
          subPath: my.cnf
        - mountPath: /livenessprobe.sh
          name: initconfdir
          subPath: livenessprobe.sh
        - mountPath: /readinessprobe.sh
          name: initconfdir
          subPath: readinessprobe.sh
      dnsPolicy: ClusterFirst
      initContainers:
      - command:
        - bash
        - -c
        - chown 27:27 /var/lib/mysql && chmod 0700 /var/lib/mysql
        image: mysql/mysql-operator:8.0.30-2.0.6
        imagePullPolicy: IfNotPresent
        name: fixdatadir
        resources: {}
        securityContext:
          runAsUser: 0
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: datadir
      - command:
        - mysqlsh
        - --log-level=@INFO
        - --pym
        - mysqloperator
        - init
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: MYSQLSH_USER_CONFIG_HOME
          value: /tmp
        image: mysql/mysql-operator:8.0.30-2.0.6
        imagePullPolicy: IfNotPresent
        name: initconf
        resources: {}
        securityContext:
          runAsUser: 27
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /mnt/initconf
          name: initconfdir
          readOnly: true
        - mountPath: /var/lib/mysql
          name: datadir
        - mountPath: /mnt/mycnfdata
          name: mycnfdata
      - args:
        - mysqld
        - --user=mysql
        env:
        - name: MYSQL_INITIALIZE_ONLY
          value: "1"
        - name: MYSQL_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              key: rootPassword
              name: mycluster-cluster-secret
        - name: MYSQLSH_USER_CONFIG_HOME
          value: /tmp
        image: mysql/mysql-server:8.0.28
        imagePullPolicy: IfNotPresent
        name: initmysql
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: datadir
        - mountPath: /var/run/mysqld
          name: rundir
        - mountPath: /etc/my.cnf.d
          name: mycnfdata
          subPath: my.cnf.d
        - mountPath: /docker-entrypoint-initdb.d
          name: mycnfdata
          subPath: docker-entrypoint-initdb.d
        - mountPath: /etc/my.cnf
          name: mycnfdata
          subPath: my.cnf
      readinessGates:
      - conditionType: mysql.oracle.com/configured
      - conditionType: mysql.oracle.com/ready
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        fsGroup: 27
        runAsGroup: 27
        runAsUser: 27
      serviceAccount: mycluster-sa
      serviceAccountName: mycluster-sa
      subdomain: mycluster
      terminationGracePeriodSeconds: 30
      volumes:
      - emptyDir: {}
        name: mycnfdata
      - emptyDir: {}
        name: rundir
      - configMap:
          defaultMode: 493
          name: mycluster-initconf
        name: initconfdir
      - emptyDir: {}
        name: shellhome
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: datadir
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 2Gi
      volumeMode: Filesystem
`
		statefulSet := &appv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(statefulYaml), statefulSet)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, statefulSet)).Should(Succeed())
		return statefulSet
	}

	patchBackupJobStatus := func(phase dataprotectionv1alpha1.BackupJobPhase, key types.NamespacedName) {
		backupJob := &dataprotectionv1alpha1.BackupJob{}
		Expect(k8sClient.Get(ctx, key, backupJob)).Should(Succeed())

		patch := client.MergeFrom(backupJob.DeepCopy())
		backupJob.Status.Phase = phase
		Expect(k8sClient.Status().Patch(ctx, backupJob, patch)).Should(Succeed())
	}

	Context("When creating restoreJob", func() {
		It("Should success with no error", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backupJob from backupPolicy: " + backupPolicy.Name)
			backupJob := assureBackupJobObj(backupPolicy.Name)

			By("By creating a restoreJob from backupJob: " + backupJob.Name)
			toCreate := assureRestoreJobObj(backupJob.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			patchBackupJobStatus(dataprotectionv1alpha1.BackupJobCompleted, types.NamespacedName{Name: backupJob.Name, Namespace: backupJob.Namespace})
			time.Sleep(waitDuration)

			result := &dataprotectionv1alpha1.RestoreJob{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())

			By("Deleting the scope")

			Eventually(func() error {
				key = types.NamespacedName{
					Name:      backupPolicy.Name,
					Namespace: backupPolicy.Namespace,
				}
				_ = deleteBackupPolicyWait(key)
				key = types.NamespacedName{
					Name:      backupTool.Name,
					Namespace: backupTool.Namespace,
				}
				_ = deleteBackupToolWait(key)

				key = types.NamespacedName{
					Name:      backupJob.Name,
					Namespace: backupJob.Namespace,
				}
				_ = deleteBackupJobWait(key)

				key = types.NamespacedName{
					Name:      toCreate.Name,
					Namespace: toCreate.Namespace,
				}
				return deleteRestoreJobWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

})

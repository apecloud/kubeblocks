package dataprotection

import (
	"context"
	batchv1 "k8s.io/api/batch/v1"
	"time"

	"github.com/sethvargo/go-password/password"
	appv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("BackupJob Controller", func() {

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

	assureBackupJobObj := func(backupPolicy string) *dataprotectionv1alpha1.BackupJob {
		By("By assure an backupJob obj")
		backupJobYaml := `
apiVersion: dataprotection.infracreate.com/v1alpha1
kind: BackupJob
metadata:
  name: backup-success-demo
  namespace: default

  labels:
    dataprotection.infracreate.com/backup-type: full
    db.infracreate.com/name: mysqlcluster
    dataprotection.infracreate.com/backup-policy-name: backup-policy-demo
    dataprotection.infracreate.com/backup-index: "0"

spec:
  backupPolicyName: backup-policy-demo
  backupType: full
  ttl: 168h0m0s
`
		backupJob := &dataprotectionv1alpha1.BackupJob{}
		Expect(yaml.Unmarshal([]byte(backupJobYaml), backupJob)).Should(Succeed())
		ns := genarateNS("backup-job-")
		backupJob.Name = ns.Name
		backupJob.Namespace = ns.Namespace
		backupJob.Spec.BackupPolicyName = backupPolicy

		Expect(checkedCreateObj(backupJob)).Should(Succeed())
		return backupJob
	}

	deleteBackupJobNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupJob{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupJob{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dataprotectionv1alpha1.BackupJob{}
		}
		return client.IgnoreNotFound(err)
	}

	assureBackupPolicyObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		backupPolicyYaml := `
apiVersion: dataprotection.infracreate.com/v1alpha1
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
    secretName: mycluster-cluster-secret
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
		Expect(checkedCreateObj(backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	deleteBackupPolicyNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupPolicy{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupPolicy{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dataprotectionv1alpha1.BackupPolicy{}
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
		Expect(checkedCreateObj(statefulSet)).Should(Succeed())
		return statefulSet
	}

	patchK8sJobStatus := func(jobStatus batchv1.JobConditionType, key types.NamespacedName) {
		k8sJob := &batchv1.Job{}
		Expect(k8sClient.Get(context.Background(), key, k8sJob)).Should(Succeed())

		patch := client.MergeFrom(k8sJob.DeepCopy())
		jobCondition := batchv1.JobCondition{Type: jobStatus}
		k8sJob.Status.Conditions = append(k8sJob.Status.Conditions, jobCondition)
		Expect(k8sClient.Patch(context.Background(), k8sJob, patch)).Should(Succeed())
	}

	Context("When creating backupJob", func() {
		It("Should success with no error", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backupJob from backupPolicy: " + backupPolicy.Name)
			toCreate := assureBackupJobObj(backupPolicy.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}
			time.Sleep(waitDuration)

			patchK8sJobStatus(batchv1.JobComplete, key)
			time.Sleep(waitDuration)

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(context.Background(), key, result)).Should(Succeed())

			By("Deleting the scope")

			Eventually(func() error {
				_ = deleteBackupPolicyNWait(key)
				_ = deleteBackupToolNWait(key)
				return deleteBackupJobNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

})

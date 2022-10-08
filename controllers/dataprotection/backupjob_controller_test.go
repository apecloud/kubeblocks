package dataprotection

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"time"

	batchv1 "k8s.io/api/batch/v1"

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
        app.kubernetes.io/instance: wesql-cluster	
    secret:
      name: wesql-cluster
      keyUser: username
      keyPassword: password
  remoteVolume:
    name: backup-remote-volume
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
  physical:
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
    incrementalRestoreCommands: []
  logical:
    restoreCommands: []
    incrementalRestoreCommands: []
  backupCommands:
    - echo "DB_HOST=${DB_HOST} DB_USER=${DB_USER} DB_PASSWORD=${DB_PASSWORD} DATA_DIR=${DATA_DIR} BACKUP_DIR=${BACKUP_DIR} BACKUP_NAME=${BACKUP_NAME}";
      mkdir -p /${BACKUP_DIR};
      xtrabackup --compress --backup  --safe-slave-backup --slave-info --stream=xbstream --host=${DB_HOST} --user=${DB_USER} --password=${DB_PASSWORD} --datadir=${DATA_DIR} > /${BACKUP_DIR}/${BACKUP_NAME}.xbstream
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
  generation: 2
  labels:
    app.kubernetes.io/instance: wesql-cluster
  name: wesql-cluster-replicasets-primary
  namespace: default
spec:
  minReadySeconds: 10
  podManagementPolicy: Parallel
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/component: replicasets-replicasets
      app.kubernetes.io/instance: wesql-cluster-replicasets-primary
      app.kubernetes.io/name: stat.mysql-wesql-clusterdefinition
  serviceName: wesql-cluster-replicasets-primary
  template:
    metadata:
      creationTimestamp: null
      labels:
        app.kubernetes.io/component: replicasets-replicasets
        app.kubernetes.io/instance: wesql-cluster-replicasets-primary
        app.kubernetes.io/name: stat.mysql-wesql-clusterdefinition
    spec:
      containers:
      - args: []
        command:
        - /bin/bash
        - -c
        image: docker.io/infracreate/wesql-server-8.0:0.1-SNAPSHOT
        imagePullPolicy: IfNotPresent
        name: mysql
        ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        - containerPort: 13306
          name: paxos
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: data
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      creationTimestamp: null
      name: data
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
      volumeMode: Filesystem
    status:
      phase: Pending
`
		podYaml := `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: '2022-09-28T16:03:21Z'
  generateName: wesql-cluster-replicasets-primary-
  labels:
    statefulset.kubernetes.io/pod-name: wesql-cluster-replicasets-primary-0
  name: wesql-cluster-replicasets-primary-0
  namespace: default
spec:
  containers:
    - args:
        - docker-entrypoint.sh mysqld
      command:
        - /bin/bash
        - '-c'
      env:
        - name: OPENDBAAS_MY_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: OPENDBAAS_REPLICASETS_PRIMARY_N
          value: '1'
        - name: OPENDBAAS_REPLICASETS_PRIMARY_0_HOSTNAME
          value: wesql-cluster-replicasets-primary-0
      image: 'docker.io/infracreate/wesql-server-8.0:0.1-SNAPSHOT'
      imagePullPolicy: IfNotPresent
      name: mysql
      ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        - containerPort: 13306
          name: paxos
          protocol: TCP
      resources: {}
      terminationMessagePath: /dev/termination-log
      terminationMessagePolicy: File
      volumeMounts:
        - mountPath: /var/lib/mysql
          name: data
  hostname: wesql-cluster-replicasets-primary-0
  preemptionPolicy: PreemptLowerPriority
  priority: 0
  restartPolicy: Always
  securityContext: {}
  subdomain: wesql-cluster-replicasets-primary
  terminationGracePeriodSeconds: 30
  tolerations:
    - effect: NoExecute
      key: node.kubernetes.io/not-ready
      operator: Exists
      tolerationSeconds: 300
    - effect: NoExecute
      key: node.kubernetes.io/unreachable
      operator: Exists
      tolerationSeconds: 300
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: data-wesql-cluster-replicasets-primary-0
  phase: Running
  qosClass: BestEffort
`
		statefulSet := &appv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(statefulYaml), statefulSet)).Should(Succeed())
		Expect(checkedCreateObj(statefulSet)).Should(Succeed())

		pod := &corev1.Pod{}
		Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(Succeed())
		Expect(checkedCreateObj(pod)).Should(Succeed())
		return statefulSet
	}

	patchK8sJobStatus := func(jobStatus batchv1.JobConditionType, key types.NamespacedName) {
		k8sJob := &batchv1.Job{}
		Expect(k8sClient.Get(context.Background(), key, k8sJob)).Should(Succeed())

		patch := client.MergeFrom(k8sJob.DeepCopy())
		jobCondition := batchv1.JobCondition{Type: jobStatus}
		k8sJob.Status.Conditions = append(k8sJob.Status.Conditions, jobCondition)
		Expect(k8sClient.Status().Patch(context.Background(), k8sJob, patch)).Should(Succeed())
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
				key = types.NamespacedName{
					Name:      backupPolicy.Name,
					Namespace: backupPolicy.Namespace,
				}
				_ = deleteBackupPolicyNWait(key)
				key = types.NamespacedName{
					Name:      backupTool.Name,
					Namespace: backupTool.Namespace,
				}
				_ = deleteBackupToolNWait(key)

				key = types.NamespacedName{
					Name:      toCreate.Name,
					Namespace: toCreate.Namespace,
				}
				return deleteBackupJobNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When failed creating backupJob", func() {
		It("Should failed with no error", func() {

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

			patchK8sJobStatus(batchv1.JobFailed, key)
			time.Sleep(waitDuration)

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(context.Background(), key, result)).Should(Succeed())

			By("Deleting the scope")

			Eventually(func() error {
				key = types.NamespacedName{
					Name:      backupPolicy.Name,
					Namespace: backupPolicy.Namespace,
				}
				_ = deleteBackupPolicyNWait(key)
				key = types.NamespacedName{
					Name:      backupTool.Name,
					Namespace: backupTool.Namespace,
				}
				_ = deleteBackupToolNWait(key)

				key = types.NamespacedName{
					Name:      toCreate.Name,
					Namespace: toCreate.Namespace,
				}
				return deleteBackupJobNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

})

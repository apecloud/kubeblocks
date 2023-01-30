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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"

	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Backup Policy Controller", func() {
	type Key = types.NamespacedName
	const timeout = time.Second * 20
	const interval = time.Second
	const TRUE = "true"

	var ctx = context.Background()

	viper.SetDefault("DP_BACKUP_SCHEDULE", "0 3 * * *")
	viper.SetDefault("DP_BACKUP_TTL", "168h0m0s")

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	genarateNS := func(prefix string) Key {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := Key{
			Name:      prefix + randomStr,
			Namespace: testCtx.DefaultNamespace,
		}
		return key
	}

	assureBackupPolicyObj := func(backupTool string, schedule string, ttl *metav1.Duration) *dpv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		backupPolicyYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicy
metadata:
  name: backup-policy-demo
spec:
  backupToolName: xtrabackup-mysql
  backupsHistoryLimit: 1
  target:
    databaseEngine: mysql
    labelsSelector:
      matchLabels:
        app.kubernetes.io/instance: wesql-cluster	
    secret:
      name: wesql-cluster
  hooks:
    preCommands:
    - touch /data/mysql/.restore;sync
    postCommands:
    - rm -f /data/mysql/.restore;sync
  remoteVolume:
    name: backup-remote-volume
    persistentVolumeClaim:
      claimName: backup-host-path-pvc
  onFailAttempted: 3
`
		backupPolicy := &dpv1alpha1.BackupPolicy{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicy)).Should(Succeed())
		ns := genarateNS("backup-policy-")
		backupPolicy.Name = ns.Name
		backupPolicy.Namespace = ns.Namespace
		backupPolicy.Spec.BackupToolName = backupTool
		backupPolicy.Spec.Schedule = schedule
		if nil != ttl {
			backupPolicy.Spec.TTL = ttl
		}
		Expect(testCtx.CreateObj(ctx, backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	assureBackupToolObj := func(withoutResources ...bool) *dpv1alpha1.BackupTool {
		By("By assure an backupTool obj")
		backupToolYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupTool
metadata:
  name: xtrabackup-mysql
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
		backupTool := &dpv1alpha1.BackupTool{}
		Expect(yaml.Unmarshal([]byte(backupToolYaml), backupTool)).Should(Succeed())
		nilResources := false
		// optional arguments, only use the first one.
		if len(withoutResources) > 0 {
			nilResources = withoutResources[0]
		}
		if nilResources {
			backupTool.Spec.Resources = nil
		}
		ns := genarateNS("backup-tool-")
		backupTool.Name = ns.Name
		backupTool.Namespace = ns.Namespace
		Expect(testCtx.CreateObj(ctx, backupTool)).Should(Succeed())
		return backupTool
	}

	assureStatefulSetObj := func() *appv1.StatefulSet {
		By("By assure an stateful obj")
		statefulYaml := `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/instance: wesql-cluster
  name: wesql-cluster-replicasets-primary
spec:
  minReadySeconds: 10
  podManagementPolicy: Parallel
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/component: replicasets-replicasets
      app.kubernetes.io/instance: wesql-cluster-replicasets-primary
      app.kubernetes.io/name: state.mysql-wesql-clusterdefinition
  serviceName: wesql-cluster-replicasets-primary
  template:
    metadata:
      creationTimestamp: null
      labels:
        app.kubernetes.io/component: replicasets-replicasets
        app.kubernetes.io/instance: wesql-cluster-replicasets-primary
        app.kubernetes.io/name: state.mysql-wesql-clusterdefinition
    spec:
      containers:
      - args: []
        command:
        - /bin/bash
        - -c
        image: docker.io/apecloud/wesql-server:latest
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
`
		podYaml := `
apiVersion: v1
kind: Pod
metadata:
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
        - name: KB_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: KB_REPLICASETS_PRIMARY_N
          value: '1'
        - name: KB_REPLICASETS_PRIMARY_0_HOSTNAME
          value: wesql-cluster-replicasets-primary-0
      image: 'docker.io/apecloud/wesql-server:latest'
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
		statefulSet.SetNamespace(testCtx.DefaultNamespace)
		statefulSet.Spec.Template.GetLabels()[testCtx.TestObjLabelKey] = TRUE
		Expect(testCtx.CreateObj(ctx, statefulSet)).Should(Succeed())

		if viper.GetBool("USE_EXISTING_CLUSTER") {
			return statefulSet
		}
		pod := &corev1.Pod{}
		Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(Succeed())
		pod.GetLabels()[testCtx.TestObjLabelKey] = TRUE
		Expect(testCtx.CreateObj(ctx, pod)).Should(Succeed())
		return statefulSet
	}

	assureBackupObj := func(backupPolicy string) *dpv1alpha1.Backup {
		By("By assure an backup obj")
		backupYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: backup-success-demo

  labels:
    dataprotection.kubeblocks.io/backup-type: full
    dataprotection.kubeblocks.io/backup-policy-name: backup-policy-demo
    dataprotection.kubeblocks.io/backup-index: "0"
    app.kubernetes.io/instance: wesql-cluster
spec:
  backupPolicyName: backup-policy-demo
  backupType: full
  ttl: 168h0m0s
`
		backup := &dpv1alpha1.Backup{}
		Expect(yaml.Unmarshal([]byte(backupYaml), backup)).Should(Succeed())
		ns := genarateNS("backup-job-")
		backup.Name = ns.Name
		backup.Namespace = ns.Namespace
		backup.Spec.BackupPolicyName = backupPolicy
		backup.Labels[dataProtectionLabelAutoBackupKey] = TRUE

		Expect(testCtx.CreateObj(ctx, backup)).Should(Succeed())
		return backup
	}

	patchBackupStatus := func(status dpv1alpha1.BackupStatus, key Key) {
		backup := dpv1alpha1.Backup{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &backup)
		}, timeout, interval).Should(Succeed())

		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status = status
		Expect(k8sClient.Status().Patch(ctx, &backup, patch)).Should(Succeed())

		Eventually(func() bool {
			if err := k8sClient.Get(ctx, key, &backup); err != nil {
				return false
			}
			return backup.Status.Expiration != nil
		}, timeout, interval).Should(BeTrue())
	}

	patchCronJobStatus := func(key Key) {
		cronJob := batchv1.CronJob{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &cronJob)
		}, timeout, interval).Should(Succeed())

		now := metav1.Now()
		patch := client.MergeFrom(cronJob.DeepCopy())
		cronJob.Status = batchv1.CronJobStatus{LastSuccessfulTime: &now, LastScheduleTime: &now}
		Expect(k8sClient.Status().Patch(ctx, &cronJob, patch)).Should(Succeed())

		Eventually(func() bool {
			if err := k8sClient.Get(ctx, key, &cronJob); err != nil {
				return false
			}
			return cronJob.Status.LastScheduleTime != nil
		}, timeout, interval).Should(BeTrue())
	}

	Context("When creating backup policy", func() {
		It("Should success with no error", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			toCreate := assureBackupPolicyObj(backupTool.Name, "0 3 * * *", nil)
			key := Key{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			result := &dpv1alpha1.BackupPolicy{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == dpv1alpha1.ConfigAvailable
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dpv1alpha1.ConfigAvailable))

			now := metav1.Now()
			backupStatus := dpv1alpha1.BackupStatus{
				Phase:               dpv1alpha1.BackupCompleted,
				Expiration:          &now,
				StartTimestamp:      &now,
				CompletionTimestamp: &now,
			}

			backupExpired := assureBackupObj(toCreate.Name)
			patchBackupStatus(backupStatus, Key{Namespace: backupExpired.Namespace, Name: backupExpired.Name})

			backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
			backupOutLimit1 := assureBackupObj(toCreate.Name)
			patchBackupStatus(backupStatus, Key{Namespace: backupOutLimit1.Namespace, Name: backupOutLimit1.Name})

			backupOutLimit2 := assureBackupObj(toCreate.Name)
			patchBackupStatus(backupStatus, Key{Namespace: backupOutLimit2.Namespace, Name: backupOutLimit2.Name})

			patchCronJobStatus(key)
		})
		It("Should success without schedule and ttl", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			toCreate := assureBackupPolicyObj(backupTool.Name, "", nil)
			key := Key{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			result := &dpv1alpha1.BackupPolicy{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, key, result); err != nil {
					return false
				}
				return result.Status.Phase == dpv1alpha1.ConfigAvailable
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dpv1alpha1.ConfigAvailable))
		})

		Context("When failed creating backup", func() {
			It("Should failed with no error", func() {
				By("By creating a statefulset")
				_ = assureStatefulSetObj()

				By("By creating a backupTool")
				backupTool := assureBackupToolObj()

				By("By creating a backupPolicy from backupTool: " + backupTool.Name)
				toCreate := assureBackupPolicyObj(backupTool.Name, "error schedule", nil)
				key := Key{
					Name:      toCreate.Name,
					Namespace: toCreate.Namespace,
				}

				result := &dpv1alpha1.BackupPolicy{}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, key, result); err != nil {
						return false
					}
					return result.Status.Phase != dpv1alpha1.ConfigAvailable
				}, timeout, interval).Should(BeTrue())
				Expect(result.Status.Phase).ShouldNot(Equal(dpv1alpha1.ConfigAvailable))
			})
		})
	})
})

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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

var _ = Describe("BackupJob Controller", func() {
	type Key = types.NamespacedName
	const timeout = time.Second * 20
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test

		err := k8sClient.DeleteAllOf(ctx, &appv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupJob{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupTool{}, client.HasLabels{testCtx.DefaultNamespace})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appv1.StatefulSet{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey},
		)
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

	assureBackupJobObj := func(backupPolicy string) *dataprotectionv1alpha1.BackupJob {
		By("By assure an backupJob obj")
		backupJobYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupJob
metadata:
  name: backup-success-demo

  labels:
    dataprotection.kubeblocks.io/backup-type: full
    db.kubeblocks.io/name: mysqlcluster
    dataprotection.kubeblocks.io/backup-policy-name: backup-policy-demo
    dataprotection.kubeblocks.io/backup-index: "0"

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

		Expect(testCtx.CheckedCreateObj(ctx, backupJob)).Should(Succeed())
		return backupJob
	}

	assureBackupJobSnapshotObj := func(backupPolicy string) *dataprotectionv1alpha1.BackupJob {
		By("By assure an backupJob obj")
		backupJobYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupJob
metadata:
  name: backup-success-demo
  namespace: default
spec:
  backupPolicyName: backup-policy-demo
  backupType: snapshot
  ttl: 168h0m0s
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

	deleteBackupJobNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupJob{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dataprotectionv1alpha1.BackupJob{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, f)
		}, waitDuration, interval).Should(Succeed())
		return client.IgnoreNotFound(err)
	}

	assureBackupPolicyObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		backupPolicyYaml := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicy
metadata:
  name: backup-policy-demo
spec:
  schedule: "0 3 * * *"
  ttl: 168h0m0s
  backupToolName: xtrabackup-mysql
  target:
    databaseEngine: mysql
    labelsSelector:
      matchLabels:
        app.kubernetes.io/instance: wesql-cluster	
    secret:
      name: wesql-cluster
      keyUser: username
      keyPassword: password
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
		backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		Expect(yaml.Unmarshal([]byte(backupPolicyYaml), backupPolicy)).Should(Succeed())
		ns := genarateNS("backup-policy-")
		backupPolicy.Name = ns.Name
		backupPolicy.Namespace = ns.Namespace
		backupPolicy.Spec.BackupToolName = backupTool
		Expect(testCtx.CheckedCreateObj(ctx, backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	deleteBackupPolicyNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupPolicy{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		f := &dataprotectionv1alpha1.BackupPolicy{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return nil
		}, waitDuration, interval).Should(Succeed())
		return nil
	}

	assureBackupToolObj := func(withoutResources ...bool) *dataprotectionv1alpha1.BackupTool {
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
		backupTool := &dataprotectionv1alpha1.BackupTool{}
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
		Expect(testCtx.CheckedCreateObj(ctx, backupTool)).Should(Succeed())
		return backupTool
	}

	deleteBackupToolNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dataprotectionv1alpha1.BackupTool{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		f := &dataprotectionv1alpha1.BackupTool{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return nil
		}, waitDuration, interval).Should(Succeed())
		return nil
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
		statefulSet.Spec.Template.GetLabels()[testCtx.TestObjLabelKey] = "true"
		Expect(testCtx.CheckedCreateObj(ctx, statefulSet)).Should(Succeed())

		if viper.GetBool("USE_EXISTING_CLUSTER") {
			return statefulSet
		}
		pod := &corev1.Pod{}
		Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(Succeed())
		pod.GetLabels()[testCtx.TestObjLabelKey] = "true"
		Expect(testCtx.CheckedCreateObj(ctx, pod)).Should(Succeed())
		return statefulSet
	}

	patchK8sJobStatus := func(jobStatus batchv1.JobConditionType, key types.NamespacedName) {
		k8sJob := batchv1.Job{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &k8sJob)
		}, timeout, interval).Should(Succeed())

		patch := client.MergeFrom(k8sJob.DeepCopy())
		jobCondition := batchv1.JobCondition{Type: jobStatus}
		k8sJob.Status.Conditions = append(k8sJob.Status.Conditions, jobCondition)
		Expect(k8sClient.Status().Patch(ctx, &k8sJob, patch)).Should(Succeed())
	}

	patchVolumeSnapshotStatus := func(key Key, readyToUse bool) {
		snap := snapshotv1.VolumeSnapshot{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &snap)
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Get(ctx, key, &snap)).Should(Succeed())

		patch := client.MergeFrom(snap.DeepCopy())
		snapStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
		snap.Status = &snapStatus
		Expect(k8sClient.Status().Patch(ctx, &snap, patch)).Should(Succeed())
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

			patchK8sJobStatus(batchv1.JobComplete, key)

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.BackupJobFailed ||
					result.Status.Phase == dataprotectionv1alpha1.BackupJobCompleted
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupJobCompleted))

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

		It("Without backupTool resources should success with no error", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj(true)

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backupJob from backupPolicy: " + backupPolicy.Name)
			toCreate := assureBackupJobObj(backupPolicy.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			patchK8sJobStatus(batchv1.JobComplete, key)

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.BackupJobFailed ||
					result.Status.Phase == dataprotectionv1alpha1.BackupJobCompleted
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupJobCompleted))

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

			patchK8sJobStatus(batchv1.JobFailed, key)

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())

			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.BackupJobFailed ||
					result.Status.Phase == dataprotectionv1alpha1.BackupJobCompleted
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupJobFailed))

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

	Context("When creating backupJob with snapshot", func() {
		It("Should success with no error", func() {
			viper.Set("VOLUMESNAPSHOT", "true")
			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backupJob from backupPolicy: " + backupPolicy.Name)
			toCreate := assureBackupJobSnapshotObj(backupPolicy.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			patchK8sJobStatus(batchv1.JobComplete, Key{Name: toCreate.Name + "-pre", Namespace: toCreate.Namespace})
			patchVolumeSnapshotStatus(key, true)
			patchK8sJobStatus(batchv1.JobComplete, Key{Name: toCreate.Name + "-post", Namespace: toCreate.Namespace})

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())

			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.BackupJobFailed ||
					result.Status.Phase == dataprotectionv1alpha1.BackupJobCompleted
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupJobCompleted))

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

		It("Should failed", func() {
			viper.Set("VOLUMESNAPSHOT", "true")
			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backupJob from backupPolicy: " + backupPolicy.Name)
			toCreate := assureBackupJobSnapshotObj(backupPolicy.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			patchK8sJobStatus(batchv1.JobFailed, Key{Name: toCreate.Name + "-pre", Namespace: toCreate.Namespace})

			result := &dataprotectionv1alpha1.BackupJob{}
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())

			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.BackupJobFailed ||
					result.Status.Phase == dataprotectionv1alpha1.BackupJobCompleted
			}, timeout, interval).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.BackupJobFailed))

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

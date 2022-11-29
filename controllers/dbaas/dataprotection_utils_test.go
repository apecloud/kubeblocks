package dbaas

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("buildSpecForRestore", func() {
	var sts appsv1.StatefulSet
	var params createParams
	ctx = context.Background()

	BeforeEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupJob{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupPolicy{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupTool{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())

		params = createParams{
			cluster:           &dbaasv1alpha1.Cluster{},
			clusterDefinition: &dbaasv1alpha1.ClusterDefinition{},
			applyObjs:         nil,
			cacheCtx:          &map[string]interface{}{},
			appVersion:        &dbaasv1alpha1.AppVersion{},
			component:         &Component{},
		}

		sts = appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "data",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "mysql",
								Image:           "docker.io/apecloud/wesql-server-8.0:0.1.2",
								ImagePullPolicy: "IfNotPresent",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "data",
										MountPath: "/data",
									},
								},
							},
						},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						Spec: corev1.PersistentVolumeClaimSpec{
							VolumeName: "data",
						},
					},
				},
			},
		}
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

	assureBackupJobObj := func(backupPolicy string, backupType dataprotectionv1alpha1.BackupJobType) *dataprotectionv1alpha1.BackupJob {
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
  ttl: 168h0m0s
`
		backupJob := &dataprotectionv1alpha1.BackupJob{}
		Expect(yaml.Unmarshal([]byte(backupJobYaml), backupJob)).Should(Succeed())
		ns := genarateNS("backup-job-")
		backupJob.Name = ns.Name
		backupJob.Namespace = ns.Namespace
		backupJob.Spec.BackupPolicyName = backupPolicy
		backupJob.Spec.BackupType = backupType

		Expect(testCtx.CheckedCreateObj(ctx, backupJob)).Should(Succeed())
		return backupJob
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
		Expect(testCtx.CheckedCreateObj(ctx, backupPolicy)).Should(Succeed())
		return backupPolicy
	}

	assureBackupToolObj := func() *dataprotectionv1alpha1.BackupTool {
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
		ns := genarateNS("backup-tool-")
		backupTool.Name = ns.Name
		backupTool.Namespace = ns.Namespace
		err := testCtx.CheckedCreateObj(ctx, backupTool)
		Expect(err).Should(Succeed())
		return backupTool
	}

	Context("Test buildSpecForRestore", func() {
		It("buildSpecForRestore with backupTool", func() {
			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backupJob from backupPolicy: " + backupPolicy.Name)
			backupJob := assureBackupJobObj(backupPolicy.Name, dataprotectionv1alpha1.BackupTypeFull)
			time.Sleep(time.Second)

			By("By buildSpecForRestore with backupTool")
			params.component.BackupSource = backupJob.Name
			err := buildSpecForRestore(reqCtx, params, k8sClient, &sts.Spec.Template.Spec, sts.Spec.VolumeClaimTemplates)
			Expect(err).Should(BeNil())
			Expect(len(sts.Spec.Template.Spec.InitContainers)).To(Equal(1))
		})

		It("buildSpecForRestore with backupSnapshot", func() {

			By("By creating a backupJob from backupPolicy ")
			ns := genarateNS("backup-policy-")
			backupJob := assureBackupJobObj(ns.Name, dataprotectionv1alpha1.BackupTypeSnapshot)
			time.Sleep(time.Second)

			By("By buildSpecForRestore with snapshot")
			params.component.BackupSource = backupJob.Name
			err := buildSpecForRestore(reqCtx, params, k8sClient, &sts.Spec.Template.Spec, sts.Spec.VolumeClaimTemplates)
			Expect(err).Should(BeNil())
			Expect(sts.Spec.VolumeClaimTemplates[0].Spec.DataSource.Name).To(Equal(backupJob.Name))
		})
	})
})

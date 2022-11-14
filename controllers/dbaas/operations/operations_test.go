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

package operations

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("OpsRequest Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	assureDefaultStorageClassObj := func() *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-hostpath-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
`
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, sc)).Should(Succeed())
		return sc
	}

	assureClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-for-operations
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Consensus
    defaultReplicas: 1
    podSpec:
      containers:
      - name: mysql
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        volumeMounts:
          - mountPath: /var/lib/mysql
            name: data
          - mountPath: /var/log
            name: log
        env:
          - name: "MYSQL_ROOT_PASSWORD"
            valueFrom:
              secretKeyRef:
                name: $(KB_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$KB_REPLICASETS_PRIMARY_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$KB_REPLICASETS_PRIMARY_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$KB_POD_NAME";
            echo $idx;
            cluster_info="$cluster_info@$(($idx+1))";
            echo $cluster_info;
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  - typeName: proxy
    componentType: Stateless
    defaultReplicas: 1
    podSpec:
      containers:
      - name: nginx
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version-operations
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: registry.jihulab.com/apecloud/mysql-server/mysql/wesql-server-arm:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, appVersion)).Should(Succeed())
		return appVersion
	}

	newClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		appVersionObj *dbaasv1alpha1.AppVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.AppVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and AppVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = assureClusterDefObj()
		}
		if appVersionObj == nil {
			appVersionObj = assureAppVersionObj()
		}

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}
		storageClassName := "csi-hostpath-sc"

		return &dbaasv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef:     clusterDefObj.GetName(),
				AppVersionRef:     appVersionObj.GetName(),
				TerminationPolicy: dbaasv1alpha1.WipeOut,
				Components: []dbaasv1alpha1.ClusterComponent{
					{
						Name: "replicasets",
						Type: "replicasets",
						VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
							{
								Name: "log",
								Spec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &storageClassName,
									AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("1Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
		}, clusterDefObj, appVersionObj, key
	}

	deleteClusterWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dbaasv1alpha1.Cluster{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dbaasv1alpha1.Cluster{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dbaasv1alpha1.Cluster{}
		}
		return client.IgnoreNotFound(err)
	}

	createOpsRequest := func(opsRequestName, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
		opsYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s`, opsRequestName, clusterName, opsType)
		opsRequest := &dbaasv1alpha1.OpsRequest{}
		_ = yaml.Unmarshal([]byte(opsYaml), opsRequest)
		return opsRequest
	}

	assureStatefulSetObj := func(clusterName string) *appv1.StatefulSet {
		By("By assure an stateful obj")
		statefulYaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: StatefulSet
metadata:
  generation: 1
  labels:
    mysql.oracle.com/cluster: mycluster
    app.kubernetes.io/instance: %s
    app.kubernetes.io/component-name: replicasets
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
`, clusterName)
		statefulSet := &appv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(statefulYaml), statefulSet)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, statefulSet)).Should(Succeed())
		return statefulSet
	}

	Context("Test OpsRequest", func() {
		It("Should Test all OpsRequest", func() {
			clusterObject, _, _, key := newClusterObj(nil, nil)
			Expect(testCtx.CreateObj(ctx, clusterObject)).Should(Succeed())

			By("Test Upgrade Ops")
			ops := createOpsRequest("upgrade_ops", clusterObject.Name, dbaasv1alpha1.UpgradeType)
			ops.Spec.ClusterOps = &dbaasv1alpha1.ClusterOps{Upgrade: &dbaasv1alpha1.Upgrade{AppVersionRef: "appversion-test"}}
			opsRes := &OpsResource{
				Ctx:        context.Background(),
				Cluster:    clusterObject,
				OpsRequest: ops,
				Client:     k8sClient,
				Recorder:   k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			_ = UpgradeAction(opsRes)

			By("Test OpsManager.MainEnter function with ClusterOps")
			opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			clusterObject.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{
				"replicasets": {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			Expect(k8sClient.Status().Update(context.Background(), clusterObject)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.RunningPhase
			_ = GetOpsManager().Reconcile(opsRes)

			By("Test VolumeExpansion")
			// create storageClass
			assureDefaultStorageClassObj()
			ops = createOpsRequest("volumeexpansion_ops", clusterObject.Name, dbaasv1alpha1.VolumeExpansionType)
			ops.Spec.ComponentOpsList = []dbaasv1alpha1.ComponentOps{
				{
					ComponentNames: []string{"replicasets"},
					VolumeExpansion: []dbaasv1alpha1.VolumeExpansion{
						{
							Name:    "log",
							Storage: resource.MustParse("2Gi"),
						},
					},
				},
			}
			opsRes.OpsRequest = ops
			_ = VolumeExpansionAction(opsRes)

			By("Test VerticalScaling")
			ops = createOpsRequest("verticalscaling_ops", clusterObject.Name, dbaasv1alpha1.VerticalScalingType)
			ops.Spec.ComponentOpsList = []dbaasv1alpha1.ComponentOps{
				{ComponentNames: []string{"replicasets"},
					VerticalScaling: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
					},
				},
			}
			opsRes.OpsRequest = ops
			_ = VerticalScalingAction(opsRes)

			By("Test Restart")
			ops = createOpsRequest("restart_ops", clusterObject.Name, dbaasv1alpha1.RestartType)
			ops.Spec.ComponentOpsList = []dbaasv1alpha1.ComponentOps{
				{ComponentNames: []string{"replicasets"}},
			}
			ops.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
			opsRes.OpsRequest = ops
			assureStatefulSetObj(clusterObject.Name)
			_ = RestartAction(opsRes)

			By("Test HorizontalScaling")
			ops = createOpsRequest("horizontalscaling_ops", clusterObject.Name, dbaasv1alpha1.HorizontalScalingType)
			ops.Spec.ComponentOpsList = []dbaasv1alpha1.ComponentOps{
				{
					ComponentNames: []string{"replicasets"},
					HorizontalScaling: &dbaasv1alpha1.HorizontalScaling{
						Replicas: 1,
					},
				},
			}
			opsRes.OpsRequest = ops
			_ = HorizontalScalingAction(opsRes)

			By("Test OpsManager.Do function with ComponentOps")
			_ = GetOpsManager().Do(opsRes)
			opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.RunningPhase
			_ = GetOpsManager().Do(opsRes)
			_ = GetOpsManager().Reconcile(opsRes)
			// test getOpsRequestAnnotation function
			opsRes.Cluster.Annotations = map[string]string{
				intctrlutil.OpsRequestAnnotationKey: `{"Updating":"horizontalscaling_ops"}`,
			}
			_ = GetOpsManager().Do(opsRes)

			By("Test OpsManager.Reconcile when opsRequest is succeed")
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.SucceedPhase
			opsRes.Cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{
				"replicasets": {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			_ = GetOpsManager().Reconcile(opsRes)

			By("Test the functions in ops_util.go")
			_ = patchOpsBehaviourNotFound(opsRes)
			_ = patchClusterPhaseMisMatch(opsRes)
			_ = patchClusterExistOtherOperation(opsRes, "horizontalscaling_ops")
			_ = PatchClusterNotFound(opsRes)
			_ = patchClusterPhaseWhenExistsOtherOps(opsRes, map[dbaasv1alpha1.Phase]string{
				dbaasv1alpha1.PendingPhase: "mysql-restart",
			})

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})
})

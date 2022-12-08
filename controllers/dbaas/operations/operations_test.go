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

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &storagev1.StorageClass{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	assureDefaultStorageClassObj := func(randomStr string) *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-hostpath-sc-%s
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
`, randomStr)
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, sc)).Should(Succeed())
		return sc
	}

	assureClusterDefObj := func(randomStr string) *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-for-operations-%s
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
                name: $(CONN_CREDENTIAL_SECRET_NAME)
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
`, randomStr)
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureAppVersionObj := func(randomStr string) *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version-operations-%s
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`, randomStr)
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, appVersion)).Should(Succeed())
		return appVersion
	}

	createPVC := func(clusterName, scName, vctName, pvcName string) {
		pvcYaml := fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: hostpath.csi.k8s.io
  labels:
    app.kubernetes.io/component-name: replicasets
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
    vct.kubeblocks.io/name: %s
  name: %s
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
  volumeMode: Filesystem
  storageClassName: %s
`, clusterName, vctName, pvcName, scName)
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(yaml.Unmarshal([]byte(pvcYaml), pvc)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), pvc)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, &corev1.PersistentVolumeClaim{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	newClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		appVersionObj *dbaasv1alpha1.AppVersion,
		randomStr string,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.AppVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and AppVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = assureClusterDefObj(randomStr)
		}
		if appVersionObj == nil {
			appVersionObj = assureAppVersionObj(randomStr)
		}
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}
		storageClassName := "csi-hostpath-sc-" + randomStr

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

	generateOpsRequestObj := func(opsRequestName, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
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

	createOpsRequest := func(opsRequest *dbaasv1alpha1.OpsRequest) *dbaasv1alpha1.OpsRequest {
		Expect(testCtx.CreateObj(ctx, opsRequest)).Should(Succeed())
		// wait until cluster created
		newOps := &dbaasv1alpha1.OpsRequest{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: opsRequest.Name, Namespace: testCtx.DefaultNamespace}, newOps)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return newOps
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

	mockDoOperationOnCluster := func(cluster *dbaasv1alpha1.Cluster, opsRequestName string, toClusterPhase dbaasv1alpha1.Phase) {
		tmpCluster := &dbaasv1alpha1.Cluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: testCtx.DefaultNamespace}, tmpCluster)).Should(Succeed())
		patch := client.MergeFrom(tmpCluster.DeepCopy())
		if tmpCluster.Annotations == nil {
			tmpCluster.Annotations = map[string]string{}
		}
		tmpCluster.Annotations[intctrlutil.OpsRequestAnnotationKey] = fmt.Sprintf("{\"%s\":\"%s\"}", toClusterPhase, opsRequestName)
		Expect(k8sClient.Patch(ctx, tmpCluster, patch)).Should(Succeed())
		Eventually(func() bool {
			myCluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: testCtx.DefaultNamespace}, myCluster)
			return getOpsRequestNameFromAnnotation(myCluster, dbaasv1alpha1.VolumeExpandingPhase) != nil
		}, timeout, interval).Should(BeTrue())
	}

	initResourcesForVolumeExpansion := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource, randomStr string) (*dbaasv1alpha1.OpsRequest, string) {
		// create storageClass
		sc := assureDefaultStorageClassObj(randomStr)
		ops := generateOpsRequestObj("volumeexpansion-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.VolumeExpansionType)
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
		// create opsRequest
		newOps := createOpsRequest(ops)

		By("mock do operation on cluster")
		mockDoOperationOnCluster(clusterObject, ops.Name, dbaasv1alpha1.VolumeExpandingPhase)

		// create-pvc
		pvcName := fmt.Sprintf("log-%s-replicasets-0", clusterObject.Name+randomStr)
		createPVC(clusterObject.Name, sc.Name, "log", pvcName)
		// waiting pvc controller mark annotation to OpsRequest
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: ops.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			if tmpOps.Annotations == nil {
				return false
			}
			_, ok := tmpOps.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey]
			return ok
		}, timeout, interval).Should(BeTrue())
		return newOps, pvcName
	}

	mockVolumeExpansionActionAndReconcile := func(opsRes *OpsResource, newOps *dbaasv1alpha1.OpsRequest) {
		patch := client.MergeFrom(newOps.DeepCopy())
		_ = volumeExpansion{}.Action(opsRes)
		newOps.Status.Phase = dbaasv1alpha1.RunningPhase
		newOps.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
		Expect(k8sClient.Status().Patch(ctx, newOps, patch)).Should(Succeed())
		opsRes.OpsRequest = newOps
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			statusComponents := tmpOps.Status.Components
			return statusComponents != nil && statusComponents["replicasets"].Phase == dbaasv1alpha1.VolumeExpandingPhase
		}, timeout, interval).Should(BeTrue())
	}

	testWarningEventOnPVC := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource) {

		randomStr := testCtx.GetRandomStr()
		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, randomStr)

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(opsRes, newOps)

		By("test warning event and volumeExpansion failed")
		// test when the event does not reach the conditions
		event := &corev1.Event{
			Count:   1,
			Type:    corev1.EventTypeWarning,
			Reason:  VolumeResizeFailed,
			Message: "You've reached the maximum modification rate per volume limit. Wait at least 6 hours between modifications per EBS volume.",
		}
		stsInvolvedObject := corev1.ObjectReference{
			Name:      pvcName,
			Kind:      intctrlutil.PersistentVolumeClaimKind,
			Namespace: "default",
		}
		event.InvolvedObject = stsInvolvedObject
		pvcEventHandler := PersistentVolumeClaimEventHandler{}
		reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			statusComponents := tmpOps.Status.Components
			return statusComponents != nil && statusComponents["replicasets"].Phase == dbaasv1alpha1.VolumeExpandingPhase
		}, timeout, interval).Should(BeTrue())

		// test when the event reach the conditions
		event.Count = 5
		event.FirstTimestamp = metav1.Time{Time: time.Now()}
		event.LastTimestamp = metav1.Time{Time: time.Now().Add(61 * time.Second)}
		Expect(pvcEventHandler.Handle(k8sClient, reqCtx, eventRecorder, event)).Should(Succeed())
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			vcts := tmpOps.Status.Components["replicasets"].VolumeClaimTemplates
			if len(vcts) == 0 || len(vcts["log"].PersistentVolumeClaimStatus) == 0 {
				return false
			}
			return vcts["log"].PersistentVolumeClaimStatus[pvcName].Status == dbaasv1alpha1.FailedPhase
		}, timeout, interval).Should(BeTrue())
	}

	testVolumeExpansion := func(clusterObject *dbaasv1alpha1.Cluster, opsRes *OpsResource, randomStr string) {
		// init resources for volume expansion
		newOps, pvcName := initResourcesForVolumeExpansion(clusterObject, opsRes, randomStr)

		By("mock run volumeExpansion action and reconcileAction")
		mockVolumeExpansionActionAndReconcile(opsRes, newOps)

		By("mock pvc is resizing")
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
			Type:               corev1.PersistentVolumeClaimResizing,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		},
		}
		Expect(k8sClient.Status().Patch(ctx, pvc, patch)).Should(Succeed())
		Eventually(func() bool {
			tmpPVC := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, tmpPVC)).Should(Succeed())
			conditions := tmpPVC.Status.Conditions
			return len(conditions) > 0 && conditions[0].Type == corev1.PersistentVolumeClaimResizing
		}, timeout, interval).Should(BeTrue())
		// waiting OpsRequest.status.components["replicasets"].vct["log"] is running
		_, _ = GetOpsManager().Reconcile(opsRes)
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			vcts := tmpOps.Status.Components["replicasets"].VolumeClaimTemplates
			return len(vcts) > 0 && vcts["log"].Status == dbaasv1alpha1.RunningPhase
		}, timeout, interval).Should(BeTrue())

		By("mock pvc resizing succeed")
		// mock pvc volumeExpansion succeed
		pvc = &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
		patch = client.MergeFrom(pvc.DeepCopy())
		pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}
		Expect(k8sClient.Status().Patch(ctx, pvc, patch)).Should(Succeed())
		Eventually(func() bool {
			tmpPVC := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, tmpPVC)).Should(Succeed())
			return tmpPVC.Status.Capacity[corev1.ResourceStorage] == resource.MustParse("2Gi")
		}, timeout, interval).Should(BeTrue())
		// waiting OpsRequest.status.phase is succeed
		_, _ = GetOpsManager().Reconcile(opsRes)
		Eventually(func() bool {
			tmpOps := &dbaasv1alpha1.OpsRequest{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: newOps.Name, Namespace: testCtx.DefaultNamespace}, tmpOps)
			return tmpOps.Status.Phase == dbaasv1alpha1.SucceedPhase
		}, timeout, interval).Should(BeTrue())

		testWarningEventOnPVC(clusterObject, opsRes)
	}

	Context("Test OpsRequest", func() {
		It("Should Test all OpsRequest", func() {
			randomStr := testCtx.GetRandomStr()
			clusterObject, _, _, key := newClusterObj(nil, nil, randomStr)
			Expect(testCtx.CreateObj(ctx, clusterObject)).Should(Succeed())

			By("Test Upgrade Ops")
			ops := generateOpsRequestObj("upgrade-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.UpgradeType)
			ops.Spec.ClusterOps = &dbaasv1alpha1.ClusterOps{Upgrade: &dbaasv1alpha1.Upgrade{AppVersionRef: "appversion-test-" + randomStr}}
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
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				"replicasets": {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), clusterObject, patch)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.RunningPhase
			_, _ = GetOpsManager().Reconcile(opsRes)

			By("Test VolumeExpansion")
			testVolumeExpansion(clusterObject, opsRes, randomStr)

			By("Test VerticalScaling")
			ops = generateOpsRequestObj("verticalscaling-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.VerticalScalingType)
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
			ops = generateOpsRequestObj("restart-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.RestartType)
			ops.Spec.ComponentOpsList = []dbaasv1alpha1.ComponentOps{
				{ComponentNames: []string{"replicasets"}},
			}
			ops.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
			opsRes.OpsRequest = ops
			assureStatefulSetObj(clusterObject.Name)
			_ = RestartAction(opsRes)

			By("Test HorizontalScaling")
			ops = generateOpsRequestObj("horizontalscaling-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.HorizontalScalingType)
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
			_, _ = GetOpsManager().Reconcile(opsRes)
			// test getOpsRequestAnnotation function
			opsRes.Cluster.Annotations = map[string]string{
				intctrlutil.OpsRequestAnnotationKey: fmt.Sprintf(`{"Updating":"horizontalscaling-ops-%s"}`, randomStr),
			}
			_ = GetOpsManager().Do(opsRes)

			By("Test OpsManager.Reconcile when opsRequest is succeed")
			opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.SucceedPhase
			opsRes.Cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				"replicasets": {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			_, _ = GetOpsManager().Reconcile(opsRes)

			By("Test the functions in ops_util.go")
			_ = patchOpsBehaviourNotFound(opsRes)
			_ = patchClusterPhaseMisMatch(opsRes)
			_ = patchClusterExistOtherOperation(opsRes, "horizontalscaling-ops-"+randomStr)
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

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

package dbaas

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/leaanthony/debme"
	"github.com/sethvargo/go-password/password"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

const (
	kFake = "fake"
)

var tlog = ctrl.Log.WithName("lifecycle_util_testing")

func TestReadCUETplFromEmbeddedFS(t *testing.T) {
	cueFS, err := debme.FS(cueTemplates, "cue")
	if err != nil {
		t.Error("Expected no error", err)
	}
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("conn_credential_template.cue"))
	if err != nil {
		t.Error("Expected no error", err)
	}
	tlog.Info("", "cueValue", cueTpl)
}

var _ = Describe("lifecycle_utils", func() {

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, the existence of old ones shall be found, which causes
		// new objects fail to create.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.VolumeSnapshotSignature, inNS, ml)
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("has the mergeMonitorConfig function", func() {
		var component *Component
		var cluster *dbaasv1alpha1.Cluster
		var clusterComp *dbaasv1alpha1.ClusterComponent
		var clusterDef *dbaasv1alpha1.ClusterDefinition
		var clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent

		BeforeEach(func() {
			component = &Component{}
			component.PodSpec = &corev1.PodSpec{}
			cluster = &dbaasv1alpha1.Cluster{}
			cluster.Name = "mysql-instance-3"
			clusterComp = &dbaasv1alpha1.ClusterComponent{}
			clusterComp.Monitor = true
			cluster.Spec.Components = append(cluster.Spec.Components, *clusterComp)
			clusterComp = &cluster.Spec.Components[0]

			clusterDef = &dbaasv1alpha1.ClusterDefinition{}
			clusterDef.Spec.Type = kStateMysql
			clusterDefComp = &dbaasv1alpha1.ClusterDefinitionComponent{}
			clusterDefComp.CharacterType = kMysql
			clusterDefComp.Monitor = &dbaasv1alpha1.MonitorConfig{
				BuiltIn: false,
				Exporter: &dbaasv1alpha1.ExporterConfig{
					ScrapePort: 9144,
					ScrapePath: "/metrics",
				},
			}
			clusterDef.Spec.Components = append(clusterDef.Spec.Components, *clusterDefComp)
			clusterDefComp = &clusterDef.Spec.Components[0]
		})

		It("should disable monitor if ClusterComponent.Monitor is false", func() {
			clusterComp.Monitor = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable builtin monitor if ClusterDefinitionComponent.Monitor.BuiltIn is false and has valid ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent.Monitor.BuiltIn is false and lacks ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent.Monitor.BuiltIn is true and CharacterType isn't recognizable", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent's CharacterType is empty", func() {
			// TODO fixme: seems setting clusterDef.Spec.Type has no effect to mergeMonitorConfig
			clusterComp.Monitor = true
			clusterDef.Spec.Type = kFake
			clusterDefComp.CharacterType = ""
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})
	})

	Context("has the checkAndUpdatePodVolumes function which generates Pod Volumes for mounting ConfigMap objects", func() {
		var sts appsv1.StatefulSet
		var volumes map[string]dbaasv1alpha1.ConfigTemplate
		BeforeEach(func() {
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
									Image:           "docker.io/apecloud/wesql-server:latest",
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
				},
			}
			volumes = make(map[string]dbaasv1alpha1.ConfigTemplate)

		})

		It("should succeed in corner case where input volumes is nil, which means no volume is added", func() {
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(1))
		})

		It("should succeed in normal test case, where one volume is added", func() {
			volumes["my_config"] = dbaasv1alpha1.ConfigTemplate{
				Name:                "myConfig",
				ConfigTplRef:        "myConfig",
				ConfigConstraintRef: "myConfig",
				VolumeName:          "myConfigVolume",
			}
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(2))
		})

		It("should succeed in normal test case, where two volumes are added", func() {
			volumes["my_config"] = dbaasv1alpha1.ConfigTemplate{
				Name:                "myConfig",
				ConfigTplRef:        "myConfig",
				ConfigConstraintRef: "myConfig",
				VolumeName:          "myConfigVolume",
			}
			volumes["my_config1"] = dbaasv1alpha1.ConfigTemplate{
				Name:                "myConfig",
				ConfigTplRef:        "myConfig",
				ConfigConstraintRef: "myConfig",
				VolumeName:          "myConfigVolume2",
			}
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(3))
		})

		It("should fail if updated volume doesn't contain ConfigMap", func() {
			const (
				cmName            = "my_config_for_test"
				replicaVolumeName = "mytest-cm-volume_for_test"
			)
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: replicaVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})
			volumes[cmName] = dbaasv1alpha1.ConfigTemplate{
				Name:                "configTplName",
				ConfigTplRef:        "configTplName",
				ConfigConstraintRef: "configTplName",
				VolumeName:          replicaVolumeName,
			}
			ps := &sts.Spec.Template.Spec
			Expect(checkAndUpdatePodVolumes(ps, volumes)).ShouldNot(Succeed())
		})

		It("should succeed if updated volume contains ConfigMap", func() {
			const (
				cmName            = "my_config_for_isv"
				replicaVolumeName = "mytest-cm-volume_for_isv"
			)

			// mock clusterdefinition has volume
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: replicaVolumeName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "anything"},
						},
					},
				})

			volumes[cmName] = dbaasv1alpha1.ConfigTemplate{
				Name:                "configTplName",
				ConfigTplRef:        "configTplName",
				ConfigConstraintRef: "configTplName",
				VolumeName:          replicaVolumeName,
			}
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(sts.Spec.Template.Spec.Volumes)).To(Equal(2))
			volume := intctrlutil.GetVolumeMountName(sts.Spec.Template.Spec.Volumes, cmName)
			Expect(volume).ShouldNot(BeNil())
			Expect(volume.ConfigMap).ShouldNot(BeNil())
			Expect(volume.ConfigMap.Name).Should(BeEquivalentTo(cmName))
			Expect(volume.Name).Should(BeEquivalentTo(replicaVolumeName))
		})

	})

	allFieldsClusterDefObj := func(needCreate bool) *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition
spec:
  type: state.mysql
  components:
  - typeName: replicasets
    componentType: Stateful
    configSpec:
      configTemplateRefs:
      - name: mysql-tree-node-template-8.0
        configTplRef: mysql-tree-node-template-8.0
        volumeName: mysql-config
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
          - mountPath: /data/config
            name: mysql-config
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
    service:
      ports:
      - protocol: TCP
        port: 80
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		if needCreate {
			Expect(testCtx.CheckedCreateObj(ctx, clusterDefinition)).Should(Succeed())
		}
		return clusterDefinition
	}

	allFieldsClusterVersionObj := func(needCreate bool) *dbaasv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterVersion
metadata:
  name:     app-version
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    configSpec:
      configTemplateRefs:
      - name: mysql-tree-node-template-8.0
        configTplRef: mysql-tree-node-template-8.0
        volumeName: mysql-config
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
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
          - mountPath: /data/config
            name: mysql-config
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
        workingDir: "/"
        envFrom: 
        - configMapRef: 
            name: test
        resources: 
          requests: 
            cpu: 2
            memory: 4Gi
        volumeDevices:
        - name: test
          devicePath: /test
        livenessProbe:
          exec:
            command:
            - cat
            - /tmp/healthy
          initialDelaySeconds: 5
          periodSeconds: 5
        readinessProbe:
          exec:
            command:
            - cat
            - /tmp/healthy
          initialDelaySeconds: 5
          periodSeconds: 5
        startupProbe:
          exec:
            command:
            - cat
            - /tmp/healthy
          initialDelaySeconds: 5
          periodSeconds: 5
        lifecycle: 
          postStart:
            exec: 
              command: 
              - cat
              - /tmp/healthy
          preStop:
            exec: 
              command: 
              - cat
              - /tmp/healthy
        terminationMessagePath: "/dev/termination-log"
        terminationMessagePolicy: File
        securityContext:
          allowPrivilegeEscalation: false
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`
		clusterVersion := &dbaasv1alpha1.ClusterVersion{}
		Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(Succeed())
		if needCreate {
			Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
		}
		return clusterVersion
	}

	newAllFieldsClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		clusterVersionObj *dbaasv1alpha1.ClusterVersion,
		needCreate bool,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj(needCreate)
		}
		if clusterVersionObj == nil {
			clusterVersionObj = allFieldsClusterVersionObj(needCreate)
		}

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}

		clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: %s
spec:
  clusterDefinitionRef: %s
  clusterVersionRef: %s
  terminationPolicy: WipeOut
  components:
  - name: replicasets
    type: replicasets
    monitor: true
    roleGroups:
    - name: primary
      type: primary
      replicas: 3
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
    resources: 
      requests: 
        cpu: 2
        memory: 4Gi
`, key.Name, key.Namespace, clusterDefObj.GetName(), clusterVersionObj.GetName())

		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		if needCreate {
			Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())
		}

		return cluster, clusterDefObj, clusterVersionObj, key
	}

	Context("has the mergeComponents function", func() {
		It("should work as expected with various inputs", func() {
			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, true)
			By("assign every available fields")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component := mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.podSpec nil")
			clusterVersion.Spec.Components[0].PodSpec = nil
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())

			clusterVersion = allFieldsClusterVersionObj(true)
			By("new container in clusterVersion not in clusterDefinition")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[1],
				&cluster.Spec.Components[0])
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))

			By("leave clusterComp nil")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				nil)
			Expect(component).ShouldNot(BeNil())

			By("leave clusterDefComp nil")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				nil,
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).Should(BeNil())
		})
	})

	// NOTES: following code are problematic, caused "Ginkgo detected an issue with your spec structure":
	//   It looks like you are calling By outside of a running spec.  Make sure you
	//   call By inside a runnable node such as It or BeforeEach and not inside the
	//   body of a container such as Describe or Context.

	newStsObj := func() *appsv1.StatefulSet {
		stsYAML := `
apiVersion: "apps/v1"
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/component-name: replicasets
    app.kubernetes.io/instance: mysql-cluster-01
    app.kubernetes.io/managed-by: kubeblocks
    app.kubernetes.io/name: state.mysql-apecloud-wesql
  name: mysql-cluster-01-replicasets
  namespace: default
spec:
  minReadySeconds: 10
  podManagementPolicy: Parallel
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/component-name: replicasets
      app.kubernetes.io/instance: mysql-cluster-01
      app.kubernetes.io/managed-by: kubeblocks
      app.kubernetes.io/name: state.mysql-apecloud-wesql
  serviceName: mysql-cluster-01-replicasets-headless
  template:
    metadata:
      creationTimestamp: null
      labels:
        app.kubernetes.io/component-name: replicasets
        app.kubernetes.io/instance: mysql-cluster-01
        app.kubernetes.io/managed-by: kubeblocks
        app.kubernetes.io/name: state.mysql-apecloud-wesql
    spec:
      containers:
      - command:
        - /bin/bash
        - -c
        image: docker.io/apecloud/wesql-server:8.0.30-4.alpha2.20221109.g819b319
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
        - mountPath: /data/mysql
          name: data
        - mountPath: /opt/mysql
          name: mysql-config
      dnsPolicy: ClusterFirst
      initContainers:
      - command:
        - sh
        - -c
        image: lynnleelhl/kubectl:latest
        imagePullPolicy: IfNotPresent
        name: init
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccount: kubeblocks
      serviceAccountName: kubeblocks
      terminationGracePeriodSeconds: 30
      volumes:
      - configMap:
          defaultMode: 420
          name: mysql-cluster-01-replicasets-mysql-config
        name: mysql-config
      - emptyDir: {}
        name: data
  updateStrategy:
    type: OnDelete
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      creationTimestamp: null
      labels:
        app.kubernetes.io/component-name: replicasets
        app.kubernetes.io/instance: mysql-cluster-01
        app.kubernetes.io/managed-by: kubeblocks
        app.kubernetes.io/name: state.mysql-apecloud-wesql
        vct.kubeblocks.io/name: data
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
		sts := appsv1.StatefulSet{}
		Expect(yaml.Unmarshal([]byte(stsYAML), &sts)).Should(Succeed())
		return &sts
	}
	pvcKey := types.NamespacedName{
		Namespace: "default",
		Name:      "data-wesql-01-replicasets-0",
	}
	snapshotName := "test-snapshot-name"
	ctx := context.Background()
	newReqCtx := func() intctrlutil.RequestCtx {
		reqCtx := intctrlutil.RequestCtx{
			Ctx:      ctx,
			Log:      logger,
			Recorder: clusterRecorder,
		}
		return reqCtx
	}
	newAllFieldsComponent := func() *Component {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
		reqCtx := newReqCtx()
		By("assign every available fields")
		component := mergeComponents(
			reqCtx,
			cluster,
			clusterDef,
			&clusterDef.Spec.Components[0],
			&clusterVersion.Spec.Components[0],
			&cluster.Spec.Components[0])
		Expect(component).ShouldNot(BeNil())
		return component
	}
	newParams := func() *createParams {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
		params := createParams{
			clusterDefinition: clusterDef,
			clusterVersion:    clusterVersion,
			cluster:           cluster,
			component:         newAllFieldsComponent(),
			applyObjs:         nil,
			cacheCtx:          &map[string]interface{}{},
		}
		return &params
	}
	newBackupPolicyTemplate := func() *dataprotectionv1alpha1.BackupPolicyTemplate {
		backupPolicyTemplateYAML := `
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  labels:
    clusterdefinition.kubeblocks.io/name: apecloud-wesql
  name: backup-policy-template-mysql
spec:
  backupToolName: mysql-xtrabackup
  hooks:
    ContainerName: mysql
    image: rancher/kubectl:v1.23.7
    preCommands:
    - touch /data/mysql/data/.restore; sync
  onFailAttempted: 3
  schedule: 0 2 * * *
  ttl: 168h0m0s
`
		backupPolicyTemplate := dataprotectionv1alpha1.BackupPolicyTemplate{}
		Expect(yaml.Unmarshal([]byte(backupPolicyTemplateYAML), &backupPolicyTemplate)).Should(Succeed())
		return &backupPolicyTemplate
	}

	Context("has helper function which builds specific object from cue template", func() {
		It("builds PVC correctly", func() {
			sts := newStsObj()
			pvc, err := buildPVCFromSnapshot(sts, pvcKey, snapshotName)
			Expect(err).Should(BeNil())
			Expect(pvc).ShouldNot(BeNil())
			Expect(pvc.Spec.AccessModes).Should(Equal(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes))
			Expect(pvc.Spec.Resources).Should(Equal(sts.Spec.VolumeClaimTemplates[0].Spec.Resources))
		})

		It("builds Service correctly", func() {
			params := newParams()
			svc, err := buildSvc(*params, true)
			Expect(err).Should(BeNil())
			Expect(svc).ShouldNot(BeNil())
		})

		It("builds ConnCredential correctly", func() {
			params := newParams()
			credential, err := buildConnCredential(*params)
			Expect(err).Should(BeNil())
			Expect(credential).ShouldNot(BeNil())
		})

		It("builds StatefulSet correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			envConfigName := "test-env-config-name"
			newParams := params
			newComponent := *params.component
			newComponent.Replicas = 0
			newParams.component = &newComponent
			sts, err := buildSts(reqCtx, *newParams, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			sts, err = buildSts(reqCtx, *params, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
		})

		It("builds Deploy correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			deploy, err := buildDeploy(reqCtx, *params)
			Expect(err).Should(BeNil())
			Expect(deploy).ShouldNot(BeNil())
		})

		It("builds PDB correctly", func() {
			params := newParams()
			pdb, err := buildPDB(*params)
			Expect(err).Should(BeNil())
			Expect(pdb).ShouldNot(BeNil())
		})

		It("builds Env Config correctly", func() {
			params := newParams()
			cfg, err := buildEnvConfig(*params)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			Expect(len(cfg.Data) == 2).Should(BeTrue())
		})

		It("builds BackupPolicy correctly", func() {
			sts := newStsObj()
			backupPolicyTemplate := newBackupPolicyTemplate()
			backupKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-backup",
			}
			policy, err := buildBackupPolicy(sts, backupPolicyTemplate, backupKey)
			Expect(err).Should(BeNil())
			Expect(policy).ShouldNot(BeNil())
		})

		It("builds BackupJob correctly", func() {
			sts := newStsObj()
			backupJobKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-backup-job",
			}
			backupPolicyName := "test-backup-policy"
			backupJob, err := buildBackup(sts, backupPolicyName, backupJobKey)
			Expect(err).Should(BeNil())
			Expect(backupJob).ShouldNot(BeNil())
		})

		It("builds VolumeSnapshot correctly", func() {
			sts := newStsObj()
			snapshotKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-snapshot",
			}
			pvcName := "test-pvc-name"
			vs, err := buildVolumeSnapshot(snapshotKey, pvcName, sts)
			Expect(err).Should(BeNil())
			Expect(vs).ShouldNot(BeNil())
		})

		It("builds CronJob correctly", func() {
			sts := newStsObj()
			pvcKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-pvc",
			}
			schedule := "* * * * *"
			cronJob, err := buildCronJob(pvcKey, schedule, sts)
			Expect(err).Should(BeNil())
			Expect(cronJob).ShouldNot(BeNil())
		})
	})

	newVolumeSnapshot := func(clusterName string) *snapshotv1.VolumeSnapshot {
		vsYAML := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  labels:
    app.kubernetes.io/component-name: replicasets
    app.kubernetes.io/created-by: kubeblocks
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
    app.kubernetes.io/name: state.mysql-apecloud-wesql
    backupjobs.dataprotection.kubeblocks.io/name: wesql-01-replicasets-scaling-qf6cr
    backuppolicies.dataprotection.kubeblocks.io/name: wesql-01-replicasets-scaling-hcxps
    dataprotection.kubeblocks.io/backup-type: snapshot
  name: test-volume-snapshot
  namespace: default
spec:
  source:
    persistentVolumeClaimName: data-wesql-01-replicasets-0
  volumeSnapshotClassName: csi-aws-ebs-snapclass
`, clusterName)
		vs := snapshotv1.VolumeSnapshot{}
		Expect(yaml.Unmarshal([]byte(vsYAML), &vs)).Should(Succeed())
		return &vs
	}

	Context("with HorizontalScalePolicy set to CloneFromSnapshot and VolumeSnapshot exists", func() {
		It("determines return value of doBackup according to whether VolumeSnapshot is ReadyToUse", func() {
			By("prepare cluster and construct component")
			reqCtx := newReqCtx()
			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
			component := mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())
			component.HorizontalScalePolicy = &dbaasv1alpha1.HorizontalScalePolicy{
				Type:             dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot,
				VolumeMountsName: "data",
			}

			By("prepare VolumeSnapshot and set ReadyToUse to true")
			vs := newVolumeSnapshot(cluster.Name)
			Expect(testCtx.CreateObj(ctx, vs)).Should(Succeed())
			Expect(testdbaas.ChangeObjStatus(&testCtx, vs, func() {
				t := true
				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: &t}
			})).Should(Succeed())
			// ensure cache is up-to-date before calling doBackup
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(vs),
				func(g Gomega, fetched *snapshotv1.VolumeSnapshot) {
					g.Expect(fetched.Status != nil && fetched.Status.ReadyToUse != nil).To(BeTrue())
				})).Should(Succeed())

			// prepare doBackup input parameters
			snapshotKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-snapshot",
			}
			sts := newStsObj()
			stsProto := *sts.DeepCopy()
			r := int32(3)
			stsProto.Spec.Replicas = &r

			By("doBackup should return requeue=false")
			shouldRequeue, err := doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
			Expect(shouldRequeue).Should(BeFalse())
			Expect(err).ShouldNot(HaveOccurred())

			By("Set ReadyToUse to nil, doBackup should return requeue=true")
			Expect(testdbaas.ChangeObjStatus(&testCtx, vs, func() {
				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: nil}
			})).Should(Succeed())
			// ensure cache is up-to-date before calling doBackup
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(vs),
				func(g Gomega, fetched *snapshotv1.VolumeSnapshot) {
					g.Expect(fetched.Status.ReadyToUse).To(BeNil())
				})).Should(Succeed())
			shouldRequeue, err = doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
			Expect(shouldRequeue).Should(BeTrue())
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

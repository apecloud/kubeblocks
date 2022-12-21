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
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/leaanthony/debme"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("mergeMonitorConfig", func() {
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
			clusterDefComp.CharacterType = KMysql
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

		It("Monitor disable in ClusterComponent", func() {
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

		It("Disable builtIn monitor in ClusterDefinitionComponent", func() {
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

		It("Disable builtIn monitor with wrong monitorConfig in ClusterDefinitionComponent", func() {
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

		It("Enable builtIn with wrong CharacterType in ClusterDefinitionComponent", func() {
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

		It("Enable builtIn with empty CharacterType and wrong clusterType in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDef.Spec.Type = kFake
			clusterDefComp.CharacterType = KEmpty
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

		It("Enable builtIn with empty CharacterType and right clusterType in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDef.Spec.Type = kStateMysql
			clusterDefComp.CharacterType = KEmpty
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9104))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			Expect(len(component.PodSpec.Containers)).To(Equal(1))
			Expect(strings.HasPrefix(component.PodSpec.Containers[0].Name, "inject-")).To(BeTrue())
		})
	})

	Context("checkAndUpdatePodVolumes", func() {
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
				},
			}
			volumes = make(map[string]dbaasv1alpha1.ConfigTemplate)

		})

		It("Corner case volume is nil, and add no volume", func() {
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(1))
		})

		It("Normal test case, and add one volume", func() {
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

		It("Normal test case, and add two volume", func() {
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

		It("replica configmap volumes test case", func() {
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

		It("ISV config volumes test case", func() {
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

	allFieldsClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Stateful
    configSpec:
      configTemplateRefs:
      - name: mysql-tree-node-template-8.0
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
                name: $(OPENDBAAS_MY_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$OPENDBAAS_REPLICASETS_PRIMARY_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$OPENDBAAS_REPLICASETS_PRIMARY_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$OPENDBAAS_MY_POD_NAME";
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
		Expect(testCtx.CheckedCreateObj(ctx, clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	allFieldsAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    configSpec:
      configTemplateRefs:
      - name: mysql-tree-node-template-8.0
        volumeName: mysql-config
    podSpec:
      containers:
      - name: mysql
        image: apecloud/wesql-server:latest
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
                name: $(OPENDBAAS_MY_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$OPENDBAAS_REPLICASETS_PRIMARY_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$OPENDBAAS_REPLICASETS_PRIMARY_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$OPENDBAAS_MY_POD_NAME";
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
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, appVersion)).Should(Succeed())
		return appVersion
	}

	newAllFieldsClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		appVersionObj *dbaasv1alpha1.AppVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.AppVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and AppVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj()
		}
		if appVersionObj == nil {
			appVersionObj = allFieldsAppVersionObj()
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
  appVersionRef: %s
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
`, key.Name, key.Namespace, clusterDefObj.GetName(), appVersionObj.GetName())

		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())

		return cluster, clusterDefObj, appVersionObj, key
	}

	Context("When mergeComponents", func() {
		It("Should merge with no error", func() {
			cluster, clusterDef, appVer, _ := newAllFieldsClusterObj(nil, nil)
			By("assign every available fields")
			component := mergeComponents(
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&appVer.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())
			By("leave appVer.podSpec nil")
			appVer.Spec.Components[0].PodSpec = nil
			component = mergeComponents(
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&appVer.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())
			appVer = allFieldsAppVersionObj()
			By("new container in appversion not in clusterdefinition")
			component = mergeComponents(
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&appVer.Spec.Components[1],
				&cluster.Spec.Components[0])
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))
			By("leave clusterComp nil")
			component = mergeComponents(
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&appVer.Spec.Components[0],
				nil)
			Expect(component).ShouldNot(BeNil())
			By("leave clusterDefComp nil")
			component = mergeComponents(
				cluster,
				clusterDef,
				nil,
				&appVer.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).Should(BeNil())
		})
	})
})

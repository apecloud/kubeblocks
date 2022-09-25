/*
Copyright 2022.

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("OpsRequest Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	checkedCreateObj := func(obj client.Object) error {
		err := k8sClient.Create(context.Background(), obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}

	assureClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-ops
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    roleGroups:
    - primary
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
    roleGroups: ["proxy"]
    defaultReplicas: 1
    isStateless: true
    podSpec:
      containers:
      - name: nginx
  roleGroupTemplates:
  - typeName: primary
    defaultReplicas: 3
    updateStrategy:
      # 对应 pdb 中的两个字段，两个中只能填一个
      maxUnavailable: 1
  - typeName: proxy
    defaultReplicas: 2
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(checkedCreateObj(clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version-ops
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: registry.jihulab.com/infracreate/mysql-server/mysql/wesql-server-arm:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(checkedCreateObj(appVersion)).Should(Succeed())
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

		return &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "dbaas.infracreate.com/v1alpha1",
				Kind:       "Cluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef: clusterDefObj.GetName(),
				AppVersionRef: appVersionObj.GetName(),
			},
		}, clusterDefObj, appVersionObj, key
	}

	deleteClusterNWait := func(key types.NamespacedName) error {
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

	createOpsDefinition := func(opsDefinitionName, clusterDefName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsDefinition {
		clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: OpsDefinition
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  type: %s
`, opsDefinitionName, clusterDefName, opsType)
		opsDefinition := &dbaasv1alpha1.OpsDefinition{}
		_ = yaml.Unmarshal([]byte(clusterYaml), opsDefinition)
		opsDefinition.Spec.Strategy = &dbaasv1alpha1.Strategy{
			Components: []dbaasv1alpha1.OpsDefComponent{
				{Type: "replicasets"},
			},
		}
		return opsDefinition
	}

	createOpsRequest := func(opsRequestName, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
		clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.infracreate.com/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s
`, opsRequestName, clusterName, opsType)
		opsRequest := &dbaasv1alpha1.OpsRequest{}
		_ = yaml.Unmarshal([]byte(clusterYaml), opsRequest)
		return opsRequest
	}

	Context("Test OpsRequest and OpsDefinition", func() {
		It("Should Test all OpsRequest", func() {
			clusterObject, clusterDef, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), clusterObject)).Should(Succeed())

			verticalScalingOpsDef := createOpsDefinition("mysql-verticalscaling-def", clusterDef.Name, dbaasv1alpha1.VerticalScalingType)
			Expect(k8sClient.Create(context.Background(), verticalScalingOpsDef)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: verticalScalingOpsDef.Name}, verticalScalingOpsDef)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// test sync OpsDefinition.status when update clusterDefinition
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name}, clusterDef)
				clusterDef.Spec.DefaultTerminatingPolicy = "Delete"
				err := k8sClient.Update(context.Background(), clusterDef)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name}, clusterDef)
				return clusterDef.Generation == clusterDef.Status.ObservedGeneration
			}, timeout, interval).Should(BeTrue())

			verticalScalingOpsRequest := createOpsRequest("mysql-verticalscaling", clusterObject.Name, dbaasv1alpha1.VerticalScalingType)
			verticalScalingOpsRequest.Spec.ComponentOps = &dbaasv1alpha1.ComponentOps{
				ComponentNames: []string{"replicasets"},
				VerticalScaling: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("400m"),
						"memory": resource.MustParse("330Mi"),
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), verticalScalingOpsRequest)).Should(Succeed())

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), client.ObjectKey{
					Name:      verticalScalingOpsRequest.Name,
					Namespace: verticalScalingOpsRequest.Namespace},
					verticalScalingOpsRequest)
				return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase},
					verticalScalingOpsRequest.Status.Phase) != -1
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})
})

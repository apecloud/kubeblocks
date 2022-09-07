package dbaas

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("ClusterDefinition Controller", func() {
	Context("When updating clusterDefinition", func() {
		It("Should update status of appVersion at the same time", func() {
			By("By creating a clusterDefinition")
			clusterDefYaml := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     mysql-cluster-definition
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    roleGroups:
    - primary
    defaultReplicas: 1
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
  roleGroupTemplates:
  - typeName: primary
    defaultReplicas: 3
`
			clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())
			Expect(k8sClient.Create(ctx, clusterDefinition)).Should(Succeed())
			createdClusterDef := &dbaasv1alpha1.ClusterDefinition{}
			// check reconciled finalizer and status
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: clusterDefinition.Namespace,
					Name:      clusterDefinition.Name,
				}, createdClusterDef)
				if err != nil {
					return false
				}
				return len(createdClusterDef.Finalizers) > 0 &&
					createdClusterDef.Status.ObservedGeneration == 1
			}, time.Second*10, time.Second*1).Should(BeTrue())
			By("By creating an appVersion")
			appVerYaml := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       AppVersion
metadata:
  name:     appversion-mysql-latest
spec:
  clusterDefinitionRef: mysql-cluster-definition
  components:
  - type: replicasets
    containers:
    - name: mysql
      image: registry.jihulab.com/infracreate/mysql-server/mysql/wesql-server-arm:latest
`
			appVersion := &dbaasv1alpha1.AppVersion{}
			Expect(yaml.Unmarshal([]byte(appVerYaml), appVersion)).Should(Succeed())
			Expect(k8sClient.Create(ctx, appVersion)).Should(Succeed())
			createdAppVersion := &dbaasv1alpha1.AppVersion{}
			// check reconciled finalizer
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: appVersion.Namespace,
					Name:      appVersion.Name,
				}, createdAppVersion)
				if err != nil {
					return false
				}
				return len(createdAppVersion.Finalizers) > 0
			}, time.Second*10, time.Second*1).Should(BeTrue())
			By("By updating clusterDefinition's spec")
			createdClusterDef.Spec.Type = "state.mysql-7"
			Expect(k8sClient.Update(ctx, createdClusterDef)).Should(Succeed())
			// check appVersion.Status.ClusterDefSyncStatus to be OutOfSync
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: appVersion.Namespace,
					Name:      appVersion.Name,
				}, createdAppVersion)
				if err != nil {
					return false
				}
				return createdAppVersion.Status.ClusterDefSyncStatus == "OutOfSync"
			}, time.Second*10, time.Second*1).Should(BeTrue())
		})
	})
})

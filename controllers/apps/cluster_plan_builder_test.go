package apps

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("cluster plan builder test", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterName        = "test-cluster" // this become cluster prefix name if used with testapps.NewClusterFactory().WithRandomName()
		mode               = "raftGroup"
	)

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, generics.BackupToolSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefName, clusterVersionName).GetObject()
		clusterObj.Spec.Mode = mode
		clusterObj.Spec.Parameters = map[string]string{
			"proxyEnabled": "true",
		}
		Expect(testCtx.Cli.Create(testCtx.Ctx, clusterObj)).Should(Succeed())
		clusterKey := client.ObjectKeyFromObject(clusterObj)
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, true)).Should(Succeed())

		clusterTplCMYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-cluster-template
  namespace: default
  labels:
    helm.sh/chart: apecloud-mysql-0.5.1-beta.0
    app.kubernetes.io/name: apecloud-mysql
    app.kubernetes.io/instance: %s
    kubeblocks.io/mode: %s
    app.kubernetes.io/version: "8.0.30"
    app.kubernetes.io/managed-by: Helm
data:
  clusterTpl: |-
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: 
    spec:
      componentSpecs:
        - name: mysql # user-defined
          componentDefRef: mysql # ref clusterdefinition componentDefs.name
          monitor: false
          replicas: 1
          serviceAccountName: kb-release-name-apecloud-mysql-cluster
          enabledLogs:     ["slow","error"]
          volumeClaimTemplates:
            - name: data # ref clusterdefinition components.containers.volumeMounts.name
              spec:
                storageClassName:
                accessModes:
                  - ReadWriteOnce
                resources:
                  requests:
                    storage: 1Gi
        {{- $withProxy := and (eq .mode "raftGroup") .parameters.proxyEnabled -}}
        {{- if $withProxy }}
        - name: etcd
          componentDefRef: etcd # ref clusterdefinition componentDefs.name
          replicas: 1
        - name: vtctld
          componentDefRef: vtctld # ref clusterdefinition componentDefs.name
          replicas: 1
        - name: vtconsensus
          componentDefRef: vtconsensus # ref clusterdefinition componentDefs.name
          replicas: 1
        - name: vtgate
          componentDefRef: vtgate # ref clusterdefinition componentDefs.name
          replicas: 1
        {{- end }}
`, clusterName, mode)
		clusterTplCM := corev1.ConfigMap{}
		Expect(yaml.Unmarshal([]byte(clusterTplCMYAML), &clusterTplCM)).Should(Succeed())
		Expect(testCtx.Cli.Create(testCtx.Ctx, &clusterTplCM)).Should(Succeed())
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("test render", func() {
		It("should render tpl successfully", func() {
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      clusterName,
				},
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Req: req,
				Log: log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
			}
			planBuilder := NewClusterPlanBuilder(reqCtx, testCtx.Cli, req)
			Expect(planBuilder.Init()).Should(Succeed())
			// TODO: CT
			//Expect(planBuilder.(*clusterPlanBuilder).transCtx.ClusterTemplate).Should(Not(BeNil()))
			//Expect(len(planBuilder.(*clusterPlanBuilder).transCtx.ClusterTemplate.Spec.ComponentSpecs)).Should(Equal(5))
		})
	})
})

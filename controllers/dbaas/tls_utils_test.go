/*
Copyright ApeCloud, Inc.

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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tls cert creation/check function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const statefulCompType = "replicasets"
	const statefulCompName = "mysql"

	const mysqlContainerName = "mysql"

	ctx := context.Background()

	// Cleanups

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest configurations
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// Testcases

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		tlsIssuer         *dbaasv1alpha1.Issuer
		clusterKey        types.NamespacedName
	)

	// Scenarios

	Context("with tls enabled", func() {
		BeforeEach(func() {
			By("Create a clusterDef obj")
			clusterDefObj = testdbaas.NewClusterDefFactory(&testCtx, clusterDefName, testdbaas.MySQLType).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}).
				AddComponent(testdbaas.ConsensusMySQL, statefulCompType).
				AddContainerEnv(mysqlContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				Create().GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(&testCtx, clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).AddContainerShort(mysqlContainerName, testdbaas.ApeCloudMySQLImage).
				Create().GetClusterVersion()

		})

		Context("when issuer is SelfSigned", func() {
			BeforeEach(func() {
				tlsIssuer = &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfSigned,
				}
			})

			It("should create the tls cert Secret and with proper configs set", func() {
				By("create a cluster obj")
				clusterObj = testdbaas.NewClusterFactory(&testCtx, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTls(true).
					SetIssuer(tlsIssuer).
					Create().
					GetCluster()
				// todo get secret
			})
		})

		Context("when issuer is SelfProvided", func() {

		})
	})
})

func TestBuildFromTemplate(t *testing.T) {
	const tpl = `{{- $cert := genSelfSignedCert "KubeBlocks" nil nil 365 }}
{{ $cert.Cert }}{{ $cert.Key }}
`
	cert, err := buildFromTemplate(tpl, nil)
	if err != nil {
		t.Error("build cert error", err)
	}
	index := strings.Index(cert, "-----BEGIN RSA PRIVATE KEY-----")
	if index < 0 {
		t.Error("error cert", cert)
	}
}
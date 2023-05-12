package lifecycle

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Component Constraints Validation", func() {
	const clusterDefName = "test-clusterdef"
	const clusterName = "test-cluster"
	const clusterVersionName = "test-clusterversion"
	const clusterNamespace = "test-constraint"
	const mysqlCompName = "mysql"
	var clusterDefObj *appsv1alpha1.ClusterDefinition

	getRandomStr := func() string {
		seq, _ := password.Generate(10, 2, 0, true, true)
		return seq
	}

	Context("test numberOfOcc", func() {
		BeforeEach(func() {
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddConstraints(&appsv1alpha1.Constraints{
					NumberOfOccurrence: appsv1alpha1.ZeroOrOnce,
				}).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddConstraints(&appsv1alpha1.Constraints{
					NumberOfOccurrence: appsv1alpha1.ExactlyOnce,
				}).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddConstraints(&appsv1alpha1.Constraints{
					NumberOfOccurrence: appsv1alpha1.OnceOrMore,
				}).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddConstraints(&appsv1alpha1.Constraints{
					NumberOfOccurrence: appsv1alpha1.Unlimited,
				}).
				GetObject()
		})
		It("component with zeroOrOnce", func() {
			comDef := clusterDefObj.Spec.ComponentDefs[0]

			By("creating a cluster with 0 component, should pass")
			cluster := testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).GetObject()
			clusterDefMap := cluster.Spec.GetDefNameMappingComponents()

			clusterComps := clusterDefMap[comDef.Name]
			err := meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 1 component, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(mysqlCompName, comDef.Name).GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 2 components, should fail")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				AddComponent(getRandomStr(), comDef.Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).ToNot(BeNil())

			By("creating a cluster with 2 components of different compDef, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				// inject a different component
				AddComponent(getRandomStr(), clusterDefObj.Spec.ComponentDefs[1].Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())
		})

		It("component with ExactlyOnce", func() {
			comDef := clusterDefObj.Spec.ComponentDefs[1]

			By("creating a cluster with 0 component, should fail")
			cluster := testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).GetObject()
			clusterDefMap := cluster.Spec.GetDefNameMappingComponents()

			clusterComps := clusterDefMap[comDef.Name]
			err := meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).NotTo(BeNil())

			By("creating a cluster with 1 component, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(mysqlCompName, comDef.Name).GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 2 components, should fail")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				AddComponent(getRandomStr(), comDef.Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).ToNot(BeNil())

			By("creating a cluster with 2 components of different compDef, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				// inject a different component
				AddComponent(getRandomStr(), clusterDefObj.Spec.ComponentDefs[0].Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())
		})

		It("component with OnceOrMore", func() {
			comDef := clusterDefObj.Spec.ComponentDefs[2]

			By("creating a cluster with 0 component, should fail")
			cluster := testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).GetObject()
			clusterDefMap := cluster.Spec.GetDefNameMappingComponents()

			clusterComps := clusterDefMap[comDef.Name]
			err := meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).NotTo(BeNil())

			By("creating a cluster with 1 component, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(mysqlCompName, comDef.Name).GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 2 components, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				AddComponent(getRandomStr(), comDef.Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 2 components of different compDef, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				// inject a different component
				AddComponent(getRandomStr(), clusterDefObj.Spec.ComponentDefs[1].Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())
		})

		It("component with Unlimited", func() {
			comDef := clusterDefObj.Spec.ComponentDefs[3]

			By("creating a cluster with 0 component, should pass")
			cluster := testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).GetObject()
			clusterDefMap := cluster.Spec.GetDefNameMappingComponents()

			clusterComps := clusterDefMap[comDef.Name]
			err := meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 1 component, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(mysqlCompName, comDef.Name).GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 2 components, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				AddComponent(getRandomStr(), comDef.Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())

			By("creating a cluster with 2 components of different compDef, should pass")
			cluster = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(getRandomStr(), comDef.Name).
				// inject a different component
				AddComponent(getRandomStr(), clusterDefObj.Spec.ComponentDefs[1].Name).
				GetObject()
			clusterDefMap = cluster.Spec.GetDefNameMappingComponents()
			clusterComps = clusterDefMap[comDef.Name]
			err = meetsNumOfOccConstraint(&comDef, clusterComps)
			Expect(err).To(BeNil())
		})
	})
	Context("test component reference", func() {

		It("nil component def reference", func() {
			compref := &appsv1alpha1.ComponentRef{
				ComponentDefName: mysqlCompName,
				FieldRefs: []*appsv1alpha1.ComponentFieldRef{
					{
						EnvName:   "MYSQL_REPLICAS",
						FieldPath: "replicas",
					},
				},
			}
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddComponentRef(compref).
				GetObject()
			compDef := clusterDefObj.Spec.ComponentDefs[0]
			err := meetsComponentRefConstraint(&compDef, clusterDefObj)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("non-existing componentDef"))
		})

		It("nil component def and nil component name", func() {
			compref := &appsv1alpha1.ComponentRef{
				FieldRefs: []*appsv1alpha1.ComponentFieldRef{
					{
						EnvName:   "MYSQL_REPLICAS",
						FieldPath: "replicas",
					},
				},
			}
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddComponentRef(compref).
				GetObject()
			compDef := clusterDefObj.Spec.ComponentDefs[0]
			err := meetsComponentRefConstraint(&compDef, clusterDefObj)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("must specify either componentDefName or componentName"))
		})

		It("invalid service def reference", func() {
			By("invalid service name, should fail")
			compref := &appsv1alpha1.ComponentRef{
				ComponentDefName: mysqlCompName,
				ServiceRefs: []*appsv1alpha1.ComponentServiceRef{
					{
						EnvNamePrefix: "MYSQL_REPLICAS",
						ServiceName:   "replicas",
					},
				},
			}
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, fmt.Sprintf("%s-%s", mysqlCompName, getRandomStr())).
				AddComponentRef(compref).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompName).
				GetObject()
			compDef := clusterDefObj.Spec.ComponentDefs[0]
			err := meetsComponentRefConstraint(&compDef, clusterDefObj)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("does not have a service"))

			By("valid service name, should pass")
			serviceName := "mysql-tcp"
			compref = &appsv1alpha1.ComponentRef{
				ComponentDefName: mysqlCompName,
				ServiceRefs: []*appsv1alpha1.ComponentServiceRef{
					{
						EnvNamePrefix: "MYSQL_REPLICAS",
						ServiceName:   serviceName,
					},
				},
			}
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompName).
				AddComponentRef(compref).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompName).
				AddNamedServicePort(serviceName, 3306).
				GetObject()
			compDef = clusterDefObj.Spec.ComponentDefs[0]
			err = meetsComponentRefConstraint(&compDef, clusterDefObj)
			Expect(err).To(BeNil())
		})
	})
})

package apps

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("resolve CompDefinition, ServiceVersion and shardingSpec", func() {
	var (
		compVersionObj *appsv1alpha1.ComponentVersion
		compDefNames   = []string{
			testapps.CompDefName("v1.0"),
			testapps.CompDefName("v1.1"),
			testapps.CompDefName("v2.0"),
			testapps.CompDefName("v3.0"),
		}
		clusterDefName       = "test-clusterdef"
		clusterVersionName   = "test-clusterversion"
		clusterName          = "test-cluster"
		componentDefName     = "test-componentdef"
		mysqlCompName        = "mysql"
		mysqlShardingName    = "mysql-sharding"
		defaultActionHandler = &appsv1alpha1.LifecycleActionHandler{CustomHandler: &appsv1alpha1.Action{Image: testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID(""))}}

		compDefs     map[string]*appsv1alpha1.ComponentDefinition
		cluster      *appsv1alpha1.Cluster
		shardingSpec *appsv1alpha1.ShardingSpec
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentVersionSignature, true, ml)

		// namespaced
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
	}
	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	createShardingClusterObj := func() *appsv1alpha1.Cluster {
		By("create default Cluster obj")
		trueValue := true
		compDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
		compDefs[componentDefName] = testapps.NewComponentDefinitionFactory(componentDefName).
			SetDefaultSpec().
			AddVar(appsv1alpha1.EnvVar{
				ValueFrom: &appsv1alpha1.VarSource{
					ComponentVarRef: &appsv1alpha1.ComponentVarSelector{
						ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
							MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
								RequireAllComponentObjects: &trueValue,
								Strategy:                   "individual",
							},
						},
					},
				},
			}).
			Create(&testCtx).GetObject()
		clusterDef := testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, componentDefName).
			GetObject()
		clusterVersion := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponentVersion(componentDefName).
			AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			GetObject()
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDef.Name, clusterVersion.Name).
			SetUID(types.UID(clusterName)).
			AddComponent(mysqlCompName, componentDefName).
			AddShardingSpecV2(mysqlShardingName, componentDefName).
			SetShards(1).
			Create(&testCtx).GetObject()
		cluster.Spec.ShardingSpecs[0].Template.ProvisionStrategy = ptr.To(appsv1alpha1.ParallelStrategy)
		cluster.Spec.ShardingSpecs[0].Template.UpdateStrategy = ptr.To(appsv1alpha1.ParallelStrategy)
		shardingSpec = &cluster.Spec.ShardingSpecs[0]
		return cluster
	}

	createCompDefinitionObjs := func() []*appsv1alpha1.ComponentDefinition {
		By("create default ComponentDefinition objs")
		objs := make([]*appsv1alpha1.ComponentDefinition, 0)
		for _, name := range compDefNames {
			f := testapps.NewComponentDefinitionFactory(name).
				SetServiceVersion(testapps.ServiceVersion("v0")) // use v0 as init service version
			for _, app := range []string{testapps.AppName, testapps.AppNameSamePrefix, testapps.DefaultActionName} {
				// use empty revision as init image tag
				f = f.SetRuntime(&corev1.Container{Name: app, Image: testapps.AppImage(app, testapps.ReleaseID(""))})
			}
			f.SetLifecycleAction(testapps.DefaultActionName, defaultActionHandler)
			objs = append(objs, f.Create(&testCtx).GetObject())
		}
		for _, obj := range objs {
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
				func(g Gomega, compDef *appsv1alpha1.ComponentDefinition) {
					g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				})).Should(Succeed())
		}
		return objs
	}

	createCompVersionObj := func() *appsv1alpha1.ComponentVersion {
		By("create a default ComponentVersion obj with multiple releases")
		obj := testapps.NewComponentVersionFactory(testapps.CompVersionName).
			SetSpec(appsv1alpha1.ComponentVersionSpec{
				CompatibilityRules: []appsv1alpha1.ComponentVersionCompatibilityRule{
					{
						// use prefix
						CompDefs: []string{testapps.CompDefName("v1"), testapps.CompDefName("v2")},
						Releases: []string{testapps.ReleaseID("r0"), testapps.ReleaseID("r1"), testapps.ReleaseID("r2"), testapps.ReleaseID("r3"), testapps.ReleaseID("r4")}, // sv: v1, v2
					},
					{
						// use prefix
						CompDefs: []string{testapps.CompDefName("v3")},
						Releases: []string{testapps.ReleaseID("r5")}, // sv: v3
					},
				},
				Releases: []appsv1alpha1.ComponentVersionRelease{
					{
						Name:           testapps.ReleaseID("r0"),
						Changes:        "init release",
						ServiceVersion: testapps.ServiceVersion("v1"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r0")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r0")),
							testapps.DefaultActionName: testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID("r0")),
						},
					},
					{
						Name:           testapps.ReleaseID("r1"),
						Changes:        "update app image",
						ServiceVersion: testapps.ServiceVersion("v1"),
						Images: map[string]string{
							testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r1")),
						},
					},
					{
						Name:           testapps.ReleaseID("r2"),
						Changes:        "publish a new service version",
						ServiceVersion: testapps.ServiceVersion("v2"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r2")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r2")),
							testapps.DefaultActionName: testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID("r2")),
						},
					},
					{
						Name:           testapps.ReleaseID("r3"),
						Changes:        "update app image",
						ServiceVersion: testapps.ServiceVersion("v2"),
						Images: map[string]string{
							testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r3")),
						},
					},
					{
						Name:           testapps.ReleaseID("r4"),
						Changes:        "update all app images for previous service version",
						ServiceVersion: testapps.ServiceVersion("v1"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r4")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r4")),
							testapps.DefaultActionName: testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID("r4")),
						},
					},
					{
						Name:           testapps.ReleaseID("r5"),
						Changes:        "publish a new service version",
						ServiceVersion: testapps.ServiceVersion("v3"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r5")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r5")),
							testapps.DefaultActionName: testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID("r5")),
						},
					},
				},
			}).
			Create(&testCtx).
			GetObject()
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
			func(g Gomega, compVersion *appsv1alpha1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
			})).Should(Succeed())

		return obj
	}

	updateNCheckCompDefinitionImages := func(compDef *appsv1alpha1.ComponentDefinition, serviceVersion string, r0, r1 string) {
		Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[0].Name, testapps.ReleaseID(""))))
		Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[1].Name, testapps.ReleaseID(""))))
		Expect(component.UpdateCompDefinitionImages4ServiceVersion(testCtx.Ctx, testCtx.Cli, compDef, serviceVersion)).Should(Succeed())
		Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(3))
		Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[0].Name, testapps.ReleaseID(r0))))
		Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[1].Name, testapps.ReleaseID(r1))))

		Expect(compDef.Spec.LifecycleActions).ShouldNot(BeNil())
		Expect(compDef.Spec.LifecycleActions.PreTerminate).ShouldNot(BeNil())
		Expect(compDef.Spec.LifecycleActions.PreTerminate.CustomHandler).ShouldNot(BeNil())
		Expect(compDef.Spec.LifecycleActions.PreTerminate.CustomHandler.Image).Should(Equal(testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID(""))))
	}

	Context("resolve component definition, service version and images", func() {
		BeforeEach(func() {
			createCompDefinitionObjs()
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("full match", func() {

			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v1.1 and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.1"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2.0 and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v1.0 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v1.1 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.1"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2.0 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2.0"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v3.0 and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v3.0"), testapps.ServiceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")
		})

		It("w/o service version", func() {
			By("with definition v1.0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v1.1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2.0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v3.0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v3.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")
		})

		It("prefix match definition", func() {
			By("with definition prefix and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition prefix and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition prefix and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, testapps.ServiceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 prefix and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2 prefix and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("prefix match definition and w/o service version", func() {
			By("with definition prefix")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 prefix")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2 prefix")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("regular expression match definition", func() {
			By("with definition exact regex and service version 1")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithExactRegex("v2.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition exact regex and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithExactRegex("v2.0"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition exact regex and service version v3")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithExactRegex("v3.0"), testapps.ServiceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 fuzzy regex and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v1"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2 fuzzy regex and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v2"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("regular expression match definition and w/o service version", func() {
			By("with definition regex")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, "^"+testapps.CompDefinitionName, "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 regex")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2 regex")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v2"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("match from definition", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v0")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "", "") // empty revision of image tag
		})

		It("resolve images from definition and version", func() {
			By("create new definition v4.0 with service version v4")
			compDefObj := testapps.NewComponentDefinitionFactory(testapps.CompDefName("v4.0")).
				SetServiceVersion(testapps.ServiceVersion("v4")).
				SetRuntime(&corev1.Container{Name: testapps.AppName, Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID(""))}).
				SetRuntime(&corev1.Container{Name: testapps.AppNameSamePrefix, Image: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID(""))}).
				SetRuntime(&corev1.Container{Name: testapps.DefaultActionName, Image: testapps.AppImage(testapps.DefaultActionName, testapps.ReleaseID(""))}).
				SetLifecycleAction(testapps.DefaultActionName, defaultActionHandler).
				Create(&testCtx).
				GetObject()
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compDefObj),
				func(g Gomega, compDef *appsv1alpha1.ComponentDefinition) {
					g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				})).Should(Succeed())

			By("new release for the definition")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				release := appsv1alpha1.ComponentVersionRelease{
					Name:           testapps.ReleaseID("r6"),
					Changes:        "publish a new service version",
					ServiceVersion: testapps.ServiceVersion("v4"),
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r6")),
						// not provide image for this app
						// testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r6")),
					},
				}
				rule := appsv1alpha1.ComponentVersionCompatibilityRule{
					CompDefs: []string{testapps.CompDefName("v4")}, // use prefix
					Releases: []string{testapps.ReleaseID("r6")},
				}
				compVersion.Spec.CompatibilityRules = append(compVersion.Spec.CompatibilityRules, rule)
				compVersion.Spec.Releases = append(compVersion.Spec.Releases, release)
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, compVersionKey, func(g Gomega, compVersion *appsv1alpha1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
			})).Should(Succeed())

			By("with definition v4.0 and service version v3")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v4.0"), testapps.ServiceVersion("v4"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v4.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v4")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r6", "") // app is r6 and another one is ""
		})
	})

	Context("resolve component definition, service version without serviceVersion in componentDefinition", func() {
		BeforeEach(func() {
			compDefs := createCompDefinitionObjs()
			for _, compDef := range compDefs {
				compDefKey := client.ObjectKeyFromObject(compDef)
				Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *appsv1alpha1.ComponentDefinition) {
					compDef.Spec.ServiceVersion = ""
				})).Should(Succeed())
			}
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("full match", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")
		})

		It("w/o service version", func() {
			By("with definition v1.0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})
	})

	Context("shardingSpec provision & update strategy", func() {
		BeforeEach(func() {
			createShardingClusterObj()
		})
		AfterEach(func() {
			cleanEnv()
		})

		It("should reject invalid provision strategy", func() {
			invalidStrategy := appsv1alpha1.UpdateStrategy("Invalid")
			shardingSpec.Template.ProvisionStrategy = &invalidStrategy
			err := validateProvisionNUpdateStrategy(cluster, compDefs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported provision strategy"))
		})

		It("should reject serial provision strategy", func() {
			shardingSpec.Template.ProvisionStrategy = ptr.To(appsv1alpha1.SerialStrategy)
			err := validateProvisionNUpdateStrategy(cluster, compDefs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("conflicted with vars"))
		})

		It("should skip requireAll check", func() {
			// with serial provision strategy
			shardingSpec.Template.ComponentDef = "non-exist-compdef"
			Expect(validateProvisionNUpdateStrategy(cluster, compDefs)).To(Succeed())
		})

		It("should pass validation with parallel provision strategy", func() {
			shardingSpec.Template.ProvisionStrategy = ptr.To(appsv1alpha1.ParallelStrategy)
			Expect(validateProvisionNUpdateStrategy(cluster, compDefs)).To(Succeed())
		})

		It("should reject invalid update strategy", func() {
			invalidStrategy := appsv1alpha1.UpdateStrategy("Invalid")
			shardingSpec.Template.UpdateStrategy = &invalidStrategy
			err := validateProvisionNUpdateStrategy(cluster, compDefs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported update strategy"))
		})
	})
})

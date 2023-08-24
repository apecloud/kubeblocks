package fuzzer

import (
	"context"
	"log"
	"strings"

	fuzz "github.com/google/gofuzz"
	"github.com/sirupsen/logrus"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

var Funcs = func(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(s *appsv1alpha1.Cluster, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.ClusterVersion, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.ClusterDefinition, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.ClusterComponentDefinition, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.ClusterComponentDefinition, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.OpsRequest, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.BackupPolicyTemplate, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *appsv1alpha1.ConfigConstraint, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *dataprotectionv1alpha1.Backup, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *dataprotectionv1alpha1.BackupPolicy, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *dataprotectionv1alpha1.BackupRepo, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *dataprotectionv1alpha1.BackupTool, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *dataprotectionv1alpha1.RestoreJob, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
		func(s *extensionsv1alpha1.Addon, c fuzz.Continue) {
			c.FuzzNoCustom(s)
		},
	}
}

func UnstructuredToCRD(u *unstructured.Unstructured) (*crdv1.CustomResourceDefinition, error) {

	var crd crdv1.CustomResourceDefinition

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &crd)
	if err != nil {
		return nil, err
	}

	return &crd, nil
}

func GetAllCRDs() []*crdv1.CustomResourceDefinition {
	cfg, err := e2eutil.GetConfig()
	if err != nil {
		logrus.WithError(err).Fatal("could not get config")
	}

	dynamic, e := dynamic.NewForConfig(cfg)
	if e != nil {
		logrus.WithError(e).Fatal("could not generate dynamic client for config")
	}

	crds, err := dynamic.Resource(types.CustomResourceDefinitionGVR()).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		logrus.WithError(err).Fatal("could not get CRDs")
	}

	if crds == nil || len(crds.Items) == 0 {
		log.Println("No CRDs found")
	}

	var crdsToTest []*crdv1.CustomResourceDefinition

	for _, crd := range crds.Items {
		name := crd.GetName()
		if strings.Contains(name, "kubeblocks.io") {
			c, _ := UnstructuredToCRD(&crd)

			crdsToTest = append(crdsToTest, c)
		}
	}
	return crdsToTest
}

package install

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	crdfuzz "kmodules.xyz/crd-schema-fuzz"

	fuzz "github.com/apecloud/kubeblocks/apis/schema/fuzzer"
)

func TestCrdTypes(t *testing.T) {
	Install(clientsetscheme.Scheme)
	crds := fuzz.GetAllCRDs()
	for _, crd := range crds {
		crdfuzz.SchemaFuzzTestForV1CRD(t, clientsetscheme.Scheme, crd, fuzzer.Funcs)
	}

}

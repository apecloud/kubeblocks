package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
)

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(appsv1alpha1.SchemeGroupVersion))
	utilruntime.Must(dataprotectionv1alpha1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(dataprotectionv1alpha1.SchemeGroupVersion))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(extensionsv1alpha1.SchemeGroupVersion))
}

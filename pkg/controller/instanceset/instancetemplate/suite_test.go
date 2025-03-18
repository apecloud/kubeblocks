package instancetemplate

import (
	"testing"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "foo"
	name      = "bar"
)

var (
	pod = builder.NewPodBuilder("", "").
		AddContainer(corev1.Container{
			Name:  "foo",
			Image: "bar",
			Ports: []corev1.ContainerPort{
				{
					Name:          "my-svc",
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: 12345,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		}).GetObject()
	template = corev1.PodTemplateSpec{
		ObjectMeta: pod.ObjectMeta,
		Spec:       pod.Spec,
	}
	volumeClaimTemplates = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("2G"),
					},
				},
			},
		},
	}
)

func TestInstanceTemplate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceTemplate Suite")
}

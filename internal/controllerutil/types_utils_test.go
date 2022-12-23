package controllerutil

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNamespacedName(t *testing.T) {
	g := NewGomegaWithT(t)

	obj := metav1.ObjectMeta{}
	obj.Name = "testobj"
	obj.Namespace = "default"

	g.Expect(GetNamespacedName(&obj).String()).To(Equal("default/testobj"))
	g.Expect(GetNamespacedName(nil).String()).To(Equal("/"))
}

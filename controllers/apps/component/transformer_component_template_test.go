/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package component

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("file templates transformer test", func() {
	const (
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *appsutil.MockReader
		dag      *graph.DAG
		transCtx *componentTransformContext

		tls = &appsv1.TLS{
			VolumeName:  "tls",
			MountPath:   "/etc/pki/tls",
			DefaultMode: ptr.To(int32(0600)),
			CAFile:      ptr.To("ca.pem"),
			CertFile:    ptr.To("cert.pem"),
			KeyFile:     ptr.To("key.pem"),
		}

		tlsConfig4KB = &appsv1.TLSConfig{
			Enable: true,
			Issuer: &appsv1.Issuer{
				Name: appsv1.IssuerKubeBlocks,
			},
		}

		tlsSecret4User = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "tls-secret-4-user",
			},
			Data: map[string][]byte{
				"ca":   []byte("ca-4-user"),
				"cert": []byte("cert-4-user"),
				"key":  []byte("key-4-user"),
			},
		}
		tlsConfig4User = &appsv1.TLSConfig{
			Enable: true,
			Issuer: &appsv1.Issuer{
				Name: appsv1.IssuerUserProvided,
				SecretRef: &appsv1.TLSSecretRef{
					Namespace: tlsSecret4User.Namespace,
					Name:      tlsSecret4User.Name,
					CA:        "ca",
					Cert:      "cert",
					Key:       "key",
				},
			},
		}

		newDAG = func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
			d := graph.NewDAG()
			graphCli.Root(d, comp, comp, model.ActionStatusPtr())
			return d
		}
	)

	BeforeEach(func() {
		reader = &appsutil.MockReader{
			Objects: []client.Object{tlsSecret4User},
		}

		compDef := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: compDefName,
			},
			Spec: appsv1.ComponentDefinitionSpec{},
		}
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)

		transCtx = &componentTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			CompDef:       compDef,
			Component:     comp,
			ComponentOrig: comp.DeepCopy(),
			SynthesizeComponent: &component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: clusterName,
				Name:        compName,
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
						},
					},
				},
			},
		}
	})

	checkTLSSecret := func(exist bool, issuer ...appsv1.IssuerName) {
		graphCli := transCtx.Client.(model.GraphClient)
		objs := graphCli.FindAll(dag, &corev1.Secret{})
		if !exist {
			Expect(len(objs)).Should(Equal(0))
		} else {
			Expect(objs).Should(HaveLen(1))
			secret := objs[0].(*corev1.Secret)
			Expect(secret.GetName()).Should(Equal(tlsSecretName(clusterName, compName)))
			if issuer[0] == appsv1.IssuerKubeBlocks {
				Expect(secret.Data).Should(HaveKey(*tls.CAFile))
				Expect(secret.Data).Should(HaveKey(*tls.CertFile))
				Expect(secret.Data).Should(HaveKey(*tls.KeyFile))
			} else {
				Expect(secret.Data).Should(HaveKeyWithValue(*tls.CAFile, tlsSecret4User.Data[tlsConfig4User.Issuer.SecretRef.CA]))
				Expect(secret.Data).Should(HaveKeyWithValue(*tls.CertFile, tlsSecret4User.Data[tlsConfig4User.Issuer.SecretRef.Cert]))
				Expect(secret.Data).Should(HaveKeyWithValue(*tls.KeyFile, tlsSecret4User.Data[tlsConfig4User.Issuer.SecretRef.Key]))
			}
		}
	}

	checkVolumeNMounts := func(exist bool) {
		targetVolume := corev1.Volume{
			Name: tls.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  tlsSecretName(clusterName, compName),
					Optional:    ptr.To(false),
					DefaultMode: tls.DefaultMode,
				},
			},
		}
		targetVolumeMount := corev1.VolumeMount{
			Name:      tls.VolumeName,
			MountPath: tls.MountPath,
			ReadOnly:  true,
		}

		podSpec := transCtx.SynthesizeComponent.PodSpec
		if exist {
			Expect(podSpec.Volumes).Should(ContainElements(targetVolume))
			for _, c := range podSpec.Containers {
				Expect(c.VolumeMounts).Should(ContainElements(targetVolumeMount))
			}
		} else {
			Expect(podSpec.Volumes).ShouldNot(ContainElements(targetVolume))
			for _, c := range podSpec.Containers {
				Expect(c.VolumeMounts).ShouldNot(ContainElements(targetVolumeMount))
			}
		}
	}

	Context("provision", func() {
		It("w/o define, disabled", func() {
			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the secret, volume and mounts
			checkTLSSecret(false)
			checkVolumeNMounts(false)
		})

		It("w/o define, enabled", func() {
			// enable the TLS
			transCtx.SynthesizeComponent.TLSConfig = tlsConfig4KB

			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring(
				fmt.Sprintf("the TLS is enabled but the component definition %s doesn't support it", transCtx.CompDef.Name)))
		})

		It("w/ define, disabled", func() {
			// define the TLS
			transCtx.CompDef.Spec.TLS = tls

			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the secret, volume and mounts
			checkTLSSecret(false)
			checkVolumeNMounts(false)
		})

		It("w/ define, enabled - kb", func() {
			// define and enable the TLS
			transCtx.CompDef.Spec.TLS = tls
			transCtx.SynthesizeComponent.TLSConfig = tlsConfig4KB

			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the secret, volume and mounts
			checkTLSSecret(true, appsv1.IssuerKubeBlocks)
			checkVolumeNMounts(true)
		})

		It("w/ define, enabled - user", func() {
			// define and enable the TLS
			transCtx.CompDef.Spec.TLS = tls
			transCtx.SynthesizeComponent.TLSConfig = tlsConfig4User

			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the secret, volume and mounts
			checkTLSSecret(true, appsv1.IssuerUserProvided)
			checkVolumeNMounts(true)
		})
	})

	Context("update & disable", func() {
		BeforeEach(func() {
			// mock the TLS secret object
			reader.Objects = append(reader.Objects, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      tlsSecretName(clusterName, compName),
				},
			})
		})

		It("update", func() {
			// define and enable the TLS
			transCtx.CompDef.Spec.TLS = tls
			transCtx.SynthesizeComponent.TLSConfig = tlsConfig4User // user only

			// update the certs
			tlsSecret4User.Data = map[string][]byte{
				"ca":   []byte("ca-4-user-updated"),
				"cert": []byte("cert-4-user-updated"),
				"key":  []byte("key-4-user-updated"),
			}

			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the secret updated
			checkTLSSecret(true, appsv1.IssuerUserProvided)
		})

		It("disable after provision", func() {
			// define the TLS
			transCtx.CompDef.Spec.TLS = tls

			transformer := &componentTLSTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the secret, volume and mounts to be deleted
			graphCli := transCtx.Client.(model.GraphClient)

			objs := graphCli.FindAll(dag, &corev1.Secret{})
			Expect(objs).Should(HaveLen(1))
			Expect(graphCli.IsAction(dag, objs[0], model.ActionDeletePtr())).Should(BeTrue())
			secret := objs[0].(*corev1.Secret)
			Expect(secret.GetName()).Should(Equal(tlsSecretName(clusterName, compName)))

			checkVolumeNMounts(false)
		})
	})
})

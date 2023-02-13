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
	"bytes"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	volumeName = "tls"
	caName     = "ca.crt"
	certName   = "tls.crt"
	keyName    = "tls.key"
	mountPath  = "/etc/pki/tls"
)

type componentPathedName struct {
	Namespace   string `json:"namespace,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	Name        string `json:"name,omitempty"`
}

func createOrCheckTLSCerts(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, scheme *runtime.Scheme) error {
	if cluster == nil {
		return nil
	}

	// secretList contains all secrets successfully created
	var secretList []v1.Secret

	for _, component := range cluster.Spec.Components {
		if !component.TLS {
			continue
		}

		if component.Issuer == nil {
			return errors.New("issuer shouldn't be nil when tls enabled")
		}

		var err error
		var secret *v1.Secret
		switch component.Issuer.Name {
		case dbaasv1alpha1.IssuerSelfProvided:
			err = checkTLSSecretRef(reqCtx, cli, cluster.Namespace, component.Issuer.SecretRef)
		case dbaasv1alpha1.IssuerSelfSigned:
			secret, err = createTLSSecret(reqCtx, cli, cluster, component.Name, scheme)
			if secret != nil {
				secretList = append(secretList, *secret)
			}
		}
		if err != nil {
			// best-effort to make tls secret creation atomic
			deleteTLSSecrets(reqCtx, cli, secretList)
			return err
		}
	}

	return nil
}

func deleteTLSSecrets(reqCtx intctrlutil.RequestCtx, cli client.Client, secretList []v1.Secret) {
	for _, secret := range secretList {
		err := cli.Delete(reqCtx.Ctx, &secret)
		if err != nil {
			reqCtx.Log.Info("delete tls secret error", "err", err)
		}
	}
}

func createTLSSecret(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentName string, scheme *runtime.Scheme) (*v1.Secret, error) {
	secret, err := composeTLSSecret(cluster.Namespace, cluster.Name, componentName)
	if err != nil {
		return nil, err
	}
	if err := intctrlutil.SetOwnership(cluster, secret, scheme, dbClusterFinalizerName); err != nil {
		return nil, err
	}
	if err := cli.Create(reqCtx.Ctx, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func composeTLSSecret(namespace, clusterName, componentName string) (*v1.Secret, error) {
	const tplFile = "tls_certs_secret_template.cue"

	secret := &v1.Secret{}
	pathedName := componentPathedName{
		Namespace:   namespace,
		ClusterName: clusterName,
		Name:        componentName,
	}
	if err := buildFromCUE(tplFile, map[string]any{"pathedName": pathedName}, "secret", secret); err != nil {
		return nil, err
	}
	secret.Name = generateTLSSecretName(clusterName, componentName)

	const tpl = `{{- $cert := genSelfSignedCert "KubeBlocks" nil nil 365 }}
{{ $cert.Cert }}
{{ $cert.Key }}
`
	out, err := buildFromTemplate(tpl, nil)
	if err != nil {
		return nil, err
	}
	index := strings.Index(out, "-----BEGIN RSA PRIVATE KEY-----")
	if index < 0 {
		return nil, errors.Errorf("wrong cert format: %s", out)
	}
	cert := out[:index]
	key := out[index:]
	secret.StringData[caName] = cert
	secret.StringData[certName] = cert
	secret.StringData[keyName] = key

	return secret, nil
}

func generateTLSSecretName(clusterName, componentName string) string {
	return clusterName + "-" + componentName + "-tls-certs"
}

func buildFromTemplate(tpl string, vars interface{}) (string, error) {
	fmap := sprig.TxtFuncMap()
	t := template.Must(template.New("tls").Funcs(fmap).Parse(tpl))
	var b bytes.Buffer
	err := t.Execute(&b, vars)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func checkTLSSecretRef(reqCtx intctrlutil.RequestCtx, cli client.Client, namespace string, secretRef *dbaasv1alpha1.TLSSecretRef) error {
	if secretRef == nil {
		return errors.New("issuer.secretRef shouldn't be nil when issuer is SelfProvided")
	}

	secret := &v1.Secret{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: namespace, Name: secretRef.Name}, secret); err != nil {
		return err
	}
	if secret.Data == nil {
		return errors.New("tls secret's data field shouldn't be nil")
	}
	keys := []string{secretRef.CA, secretRef.Cert, secretRef.Key}
	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return errors.Errorf("tls secret's data[%s] field shouldn't be empty", key)
		}
	}

	return nil
}

func updateTLSVolumeAndVolumeMount(podSpec *v1.PodSpec, clusterName string, component Component) error {
	if !component.TLS {
		return nil
	}

	// update volume
	volumes := podSpec.Volumes
	volume, err := composeTLSVolume(clusterName, component)
	if err != nil {
		return err
	}
	volumes = append(volumes, *volume)
	podSpec.Volumes = volumes

	// update volumeMount
	for index, container := range podSpec.Containers {
		volumeMounts := container.VolumeMounts
		volumeMount := composeTLSVolumeMount()
		volumeMounts = append(volumeMounts, volumeMount)
		podSpec.Containers[index].VolumeMounts = volumeMounts
	}

	return nil
}

func composeTLSVolume(clusterName string, component Component) (*v1.Volume, error) {
	if !component.TLS {
		return nil, errors.New("can't compose TLS volume when TLS not enabled")
	}
	if component.Issuer == nil {
		return nil, errors.New("issuer shouldn't be nil when TLS enabled")
	}
	if component.Issuer.Name == dbaasv1alpha1.IssuerSelfProvided && component.Issuer.SecretRef == nil {
		return nil, errors.New("secret ref shouldn't be nil when issuer is SelfProvided")
	}

	var secretName, ca, cert, key string
	switch component.Issuer.Name {
	case dbaasv1alpha1.IssuerSelfSigned:
		secretName = generateTLSSecretName(clusterName, component.Name)
		ca = caName
		cert = certName
		key = keyName
	case dbaasv1alpha1.IssuerSelfProvided:
		secretName = component.Issuer.SecretRef.Name
		ca = component.Issuer.SecretRef.CA
		cert = component.Issuer.SecretRef.Cert
		key = component.Issuer.SecretRef.Key
	}
	volume := v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: secretName,
				Items: []v1.KeyToPath{
					{Key: ca, Path: caName},
					{Key: cert, Path: certName},
					{Key: key, Path: keyName},
				},
				Optional: func() *bool { o := false; return &o }(),
			},
		},
	}

	return &volume, nil
}

func composeTLSVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
		ReadOnly:  true,
	}
}

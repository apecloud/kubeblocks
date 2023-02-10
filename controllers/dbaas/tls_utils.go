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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type componentPathedName struct {
	Namespace   string `json:"namespace,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	Name        string `json:"name,omitempty"`
}
func createOrCheckTlsCerts(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster) error {
	if cluster == nil {
		return nil
	}

	// secretList contains all secrets successfully created
	var secretList []v1.Secret

	for _, component := range cluster.Spec.Components {
		if !component.Tls {
			continue
		}

		if component.Issuer == nil {
			return errors.New("issuer shouldn't be nil when tls enabled")
		}

		var err error
		var secret *v1.Secret
		switch component.Issuer.Name {
		case dbaasv1alpha1.IssuerSelfProvided:
			err = checkTlsSecretRef(reqCtx, cli, cluster.Namespace, component.Issuer.SecretRef)
		case dbaasv1alpha1.IssuerSelfSigned:
			secret, err = createTlsSecret(reqCtx, cli, cluster.Namespace, cluster.Name, component.Name)
			if secret != nil {
				secretList = append(secretList, *secret)
			}
		}
		if err != nil {
			// best-effort to make tls secret creation atomic
			deleteTlsSecrets(reqCtx, cli, secretList)
			return err
		}
	}

	return nil
}

func deleteTlsSecrets(reqCtx intctrlutil.RequestCtx, cli client.Client, secretList []v1.Secret) {
	for _, secret := range secretList {
		err := cli.Delete(reqCtx.Ctx, &secret)
		if err != nil {
			reqCtx.Log.Info("delete tls secret error", "err", err)
		}
	}
}

func createTlsSecret(reqCtx intctrlutil.RequestCtx, cli client.Client, ns string, clusterName string, componentName string) (*v1.Secret, error) {
	const tplFile = "tls_certs_secret_template.cue"

	secret := &v1.Secret{}
	pathedName := componentPathedName{
		Namespace: ns,
		ClusterName: clusterName,
		Name: componentName,
	}
	if err := buildFromCUE(tplFile, map[string]any{"pathedName": pathedName}, "secret", secret); err != nil {
		return nil, err
	}

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
	secret.StringData["ca.crt"] = cert
	secret.StringData["tls.crt"] = cert
	secret.StringData["tls.key"] = key

	return secret, err
}

func buildFromTemplate(tpl string, vars interface{}) (string, error)  {
	fmap := sprig.TxtFuncMap()
	t := template.Must(template.New("tls").Funcs(fmap).Parse(tpl))
	var b bytes.Buffer
	err := t.Execute(&b, vars)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func checkTlsSecretRef(reqCtx intctrlutil.RequestCtx, cli client.Client, namespace string, secretRef *dbaasv1alpha1.TlsSecretRef) error {
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
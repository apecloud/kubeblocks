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

package plan

import (
	"bytes"
	"context"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	client2 "github.com/apecloud/kubeblocks/internal/controller/client"
)

// ComposeTLSSecret compose a TSL secret object.
// REVIEW/TODO:
//  1. missing public function doc
//  2. should avoid using Go template to call a function, this is too hack & costly,
//     should just call underlying registered Go template function.
func ComposeTLSSecret(namespace, clusterName, componentName string) (*v1.Secret, error) {
	secret, err := builder.BuildTLSSecret(namespace, clusterName, componentName)
	if err != nil {
		return nil, err
	}
	secret.Name = GenerateTLSSecretName(clusterName, componentName)

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
	secret.StringData[builder.CAName] = cert
	secret.StringData[builder.CertName] = cert
	secret.StringData[builder.KeyName] = key
	return secret, nil
}

func GenerateTLSSecretName(clusterName, componentName string) string {
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

func CheckTLSSecretRef(ctx context.Context, cli client2.ReadonlyClient, namespace string,
	secretRef *dbaasv1alpha1.TLSSecretRef) error {
	if secretRef == nil {
		return errors.New("issuer.secretRef shouldn't be nil when issuer is UserProvided")
	}

	secret := &v1.Secret{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretRef.Name}, secret); err != nil {
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

func GetTLSKeyWord(cType string) string {
	switch cType {
	case "mysql":
		return "ssl_cert"
	case "postgresql":
		return "ssl_cert_file"
	case "redis":
		return "tls-cert-file"
	default:
		return "unsupported-character-type"
	}
}

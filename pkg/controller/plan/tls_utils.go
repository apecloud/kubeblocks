/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package plan

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// ComposeTLSSecret composes a TSL secret object.
// REVIEW/TODO:
//  1. missing public function doc
//  2. should avoid using Go template to call a function, this is too hacky & costly,
//     should just call underlying registered Go template function.
func ComposeTLSSecret(namespace, clusterName, componentName string) (*v1.Secret, error) {
	name := GenerateTLSSecretName(clusterName, componentName)
	secret := builder.NewSecretBuilder(namespace, name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		AddLabels(constant.AppInstanceLabelKey, clusterName).
		AddLabels(constant.KBAppComponentLabelKey, componentName).
		SetStringData(map[string]string{}).
		GetObject()

	// use ca gen cert
	// IP: 127.0.0.1 and ::1
	// DNS: localhost and *.<clusterName>-<componentName>-headless.<namespace>.svc.cluster.local
	const spliter = "___spliter___"
	SignedCertTpl := fmt.Sprintf(`
	{{- $ca := genCA "KubeBlocks" 36500 -}}
	{{- $cert := genSignedCert "%s peer" (list "127.0.0.1" "::1") (list "localhost" "*.%s-%s-headless.%s.svc.cluster.local") 36500 $ca -}}
	{{- $ca.Cert -}}
	{{- print "%s" -}}
	{{- $cert.Cert -}}
	{{- print "%s" -}}
	{{- $cert.Key -}}
`, componentName, clusterName, componentName, namespace, spliter, spliter)
	out, err := buildFromTemplate(SignedCertTpl, nil)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(out, spliter)
	if len(parts) != 3 {
		return nil, errors.Errorf("generate TLS certificates failed with cluster name %s, component name %s in namespace %s", clusterName, componentName, namespace)
	}
	secret.StringData[constant.CAName] = parts[0]
	secret.StringData[constant.CertName] = parts[1]
	secret.StringData[constant.KeyName] = parts[2]
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

func CheckTLSSecretRef(ctx context.Context, cli client.Reader, namespace string,
	secretRef *dbaasv1alpha1.TLSSecretRef) error {
	if secretRef == nil {
		return errors.New("issuer.secretRef shouldn't be nil when issuer is UserProvided")
	}

	secret := &v1.Secret{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretRef.Name}, secret); err != nil {
		return err
	}
	if secret.StringData == nil {
		return errors.New("tls secret's data field shouldn't be nil")
	}
	keys := []string{secretRef.CA, secretRef.Cert, secretRef.Key}
	for _, key := range keys {
		if _, ok := secret.StringData[key]; !ok {
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

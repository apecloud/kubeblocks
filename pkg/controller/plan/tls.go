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
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

func GenerateTLSSecretName(clusterName, componentName string) string {
	return clusterName + "-" + componentName + "-tls-certs"
}

func BuildTLSSecret(synthesizedComp component.SynthesizedComponent) *v1.Secret {
	name := GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name)
	return builder.NewSecretBuilder(synthesizedComp.Namespace, name).
		// Priority: static < dynamic < built-in
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotationsInMap(synthesizedComp.DynamicAnnotations).
		SetStringData(map[string]string{}).
		SetData(map[string][]byte{}).
		GetObject()
}

// ComposeTLSSecret composes a TSL secret object.
// REVIEW/TODO:
//  1. missing public function doc
//  2. should avoid using Go template to call a function, this is too hacky & costly,
//     should just call underlying registered Go template function.
func ComposeTLSSecret(compDef *appsv1.ComponentDefinition, synthesizedComp component.SynthesizedComponent, secret *v1.Secret) (*v1.Secret, error) {
	var (
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
	)
	if secret == nil {
		secret = BuildTLSSecret(synthesizedComp)
	}
	// use ca gen cert
	// IP: 127.0.0.1 and ::1
	// DNS: localhost and *.<clusterName>-<compName>-headless.<namespace>.svc.cluster.local
	const spliter = "___spliter___"
	SignedCertTpl := fmt.Sprintf(`
	{{- $ca := genCA "KubeBlocks" 36500 -}}
	{{- $cert := genSignedCert "%s peer" (list "127.0.0.1" "::1") (list "localhost" "*.%s-%s-headless.%s.svc.cluster.local") 36500 $ca -}}
	{{- $ca.Cert -}}
	{{- print "%s" -}}
	{{- $cert.Cert -}}
	{{- print "%s" -}}
	{{- $cert.Key -}}
`, compName, clusterName, compName, namespace, spliter, spliter)
	out, err := buildFromTemplate(SignedCertTpl, nil)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(out, spliter)
	if len(parts) != 3 {
		return nil, errors.Errorf("generate TLS certificates failed with cluster name %s, component name %s in namespace %s",
			clusterName, compName, namespace)
	}
	if compDef.Spec.TLS.CAFile != nil {
		secret.StringData[*compDef.Spec.TLS.CAFile] = parts[0]
	}
	if compDef.Spec.TLS.CertFile != nil {
		secret.StringData[*compDef.Spec.TLS.CertFile] = parts[1]
	}
	if compDef.Spec.TLS.KeyFile != nil {
		secret.StringData[*compDef.Spec.TLS.KeyFile] = parts[2]
	}
	return secret, nil
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

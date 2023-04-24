/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	client2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/component"
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

func updateTLSVolumeAndVolumeMount(podSpec *v1.PodSpec, clusterName string, component component.SynthesizedComponent) error {
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

func composeTLSVolume(clusterName string, component component.SynthesizedComponent) (*v1.Volume, error) {
	if !component.TLS {
		return nil, errors.New("can't compose TLS volume when TLS not enabled")
	}
	if component.Issuer == nil {
		return nil, errors.New("issuer shouldn't be nil when TLS enabled")
	}
	if component.Issuer.Name == dbaasv1alpha1.IssuerUserProvided &&
		component.Issuer.SecretRef == nil {
		return nil, errors.New("secret ref shouldn't be nil when issuer is UserProvided")
	}
	var secretName, ca, cert, key string
	switch component.Issuer.Name {
	case dbaasv1alpha1.IssuerKubeBlocks:
		secretName = GenerateTLSSecretName(clusterName, component.Name)
		ca = builder.CAName
		cert = builder.CertName
		key = builder.KeyName
	case dbaasv1alpha1.IssuerUserProvided:
		secretName = component.Issuer.SecretRef.Name
		ca = component.Issuer.SecretRef.CA
		cert = component.Issuer.SecretRef.Cert
		key = component.Issuer.SecretRef.Key
	}
	volume := v1.Volume{
		Name: builder.VolumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: secretName,
				Items: []v1.KeyToPath{
					{Key: ca, Path: builder.CAName},
					{Key: cert, Path: builder.CertName},
					{Key: key, Path: builder.KeyName},
				},
				Optional: func() *bool { o := false; return &o }(),
			},
		},
	}
	return &volume, nil
}

func composeTLSVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      builder.VolumeName,
		MountPath: builder.MountPath,
		ReadOnly:  true,
	}
}

func IsTLSSettingsUpdated(cType string, oldCm v1.ConfigMap, newCm v1.ConfigMap) bool {
	// build intersection sets
	oldKeys := make([]string, 0)
	for key := range oldCm.Data {
		oldKeys = append(oldKeys, key)
	}
	oldSet := sets.New(oldKeys...)
	newKeys := make([]string, 0)
	for key := range newCm.Data {
		newKeys = append(newKeys, key)
	}
	newSet := sets.New(newKeys...)
	interSet := oldSet.Intersection(newSet)

	// get tls key-word based on cType
	tlsKeyWord := GetTLSKeyWord(cType)

	// search key-word in both old and new set
	for _, configFileName := range interSet.UnsortedList() {
		oldConfigFile := oldCm.Data[configFileName]
		newConfigFile := newCm.Data[configFileName]
		oldIndex := strings.Index(oldConfigFile, tlsKeyWord)
		newIndex := strings.Index(newConfigFile, tlsKeyWord)
		// tls key-word appears in one file and disappears in another, means tls settings updated
		if oldIndex >= 0 && newIndex < 0 ||
			oldIndex < 0 && newIndex >= 0 {
			return true
		}
	}

	return false
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

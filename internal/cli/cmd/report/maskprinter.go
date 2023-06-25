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

package report

import (
	"encoding/base64"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
)

var _ printers.ResourcePrinter = &MaskPrinter{}

const (
	EncryptedData = "KUBE_BLOCKS_ENCRYPTED_DATA"
)

type MaskPrinter struct {
	Delegate printers.ResourcePrinter
}

func (p *MaskPrinter) PrintObj(obj runtime.Object, w io.Writer) error {
	if obj == nil {
		return p.Delegate.PrintObj(obj, w)
	}
	if meta.IsListType(obj) {
		obj = obj.DeepCopyObject()
		_ = meta.EachListItem(obj, func(item runtime.Object) error {
			maskDataField(item)
			return nil
		})
	} else if _, err := meta.Accessor(obj); err == nil {
		obj = maskDataField(obj.DeepCopyObject())
	}
	return p.Delegate.PrintObj(obj, w)
}

func maskDataField(o runtime.Object) runtime.Object {
	objKind := o.GetObjectKind().GroupVersionKind().Kind
	if objKind == "Secret" || objKind == "ConfigMap" {
		switch o := o.(type) {
		case *corev1.Secret:
			for k := range o.Data {
				o.Data[k] = []byte(EncryptedData)
			}
		case *corev1.ConfigMap:
			for k := range o.Data {
				o.Data[k] = EncryptedData
			}
		case *unstructured.Unstructured:
			data := o.Object["data"]
			if data == nil {
				return o
			}
			if data := data.(map[string]interface{}); data != nil {
				for k := range data {
					if objKind == "Secret" {
						data[k] = base64.StdEncoding.EncodeToString([]byte(EncryptedData))
					} else {
						data[k] = EncryptedData
					}
				}
			}
		}
	}
	return o
}

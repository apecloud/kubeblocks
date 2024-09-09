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

package v1alpha1

import (
	"encoding/json"
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kbIncrementConverterAK = "kb-increment-converter"
)

type incrementChange any

type incrementConverter interface {
	// incrementConvertTo converts the object to the target version,
	// returning any incremental changes that need to be persisted out of the target object.
	incrementConvertTo(dstRaw metav1.Object) (incrementChange, error)

	// incrementConvertFrom converts the object from the source version,
	// and applies any incremental changes that were persisted out of the source object.
	incrementConvertFrom(srcRaw metav1.Object, ic incrementChange) error
}

func incrementConvertTo(converter incrementConverter, target metav1.Object) error {
	if converter == nil {
		return nil
	}

	changes, err := converter.incrementConvertTo(target)
	if err != nil {
		return err
	}
	if changes == nil {
		return nil
	}

	bytes, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[kbIncrementConverterAK] = string(bytes)
	target.SetAnnotations(annotations)

	return nil
}

func incrementConvertFrom(converter incrementConverter, source metav1.Object, ic incrementChange) error {
	if converter == nil {
		return nil
	}

	annotations := source.GetAnnotations()
	if annotations != nil {
		data, ok := annotations[kbIncrementConverterAK]
		if ok {
			if err := json.Unmarshal([]byte(data), ic); err != nil {
				return err
			}
			maps.DeleteFunc(annotations, func(k, v string) bool { return k == kbIncrementConverterAK })
			source.SetAnnotations(annotations)
		}
	}

	return converter.incrementConvertFrom(source, ic)
}

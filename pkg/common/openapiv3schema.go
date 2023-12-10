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

package common

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"
)

// ValidateDataWithSchema validates if the data is valid with the jsonSchema.
func ValidateDataWithSchema(openAPIV3Schema *apiextensionsv1.JSONSchemaProps, data interface{}) error {
	if openAPIV3Schema == nil {
		return fmt.Errorf("openAPIV3Schema can not be empty")
	}
	out := &apiextensions.JSONSchemaProps{}
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(openAPIV3Schema, out, nil); err != nil {
		return err
	}
	openapiSchema := &spec.Schema{}
	if err := validation.ConvertJSONSchemaPropsWithPostProcess(out, openapiSchema, validation.StripUnsupportedFormatsPostProcess); err != nil {
		return err
	}
	validator := validate.NewSchemaValidator(openapiSchema, nil, "", strfmt.Default)
	res := validator.Validate(data)
	if !res.IsValid() && res.HasErrors() {
		// throw the head error
		return res.Errors[0]
	}
	return nil
}

func CoverStringToInterfaceBySchemaType(openAPIV3Schema *apiextensionsv1.JSONSchemaProps, input map[string]string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	properties := openAPIV3Schema.Properties
	covertError := func(key string, err error) error {
		return fmt.Errorf(`covert "%s" failed: %s`, key, err.Error())
	}
	var err error
	for k, v := range input {
		p, ok := properties[k]
		if !ok {
			continue
		}
		switch p.Type {
		case "integer":
			out[k], err = strconv.ParseInt(v, 10, 64)
		case "number":
			out[k], err = strconv.ParseFloat(v, 64)
		case "boolean":
			out[k], err = strconv.ParseBool(v)
		case "array":
			out[k] = strings.Split(v, ",")
			// TODO: validate element type of the array
		default:
			out[k] = v
		}
		if err != nil {
			return nil, covertError(k, err)
		}
	}
	return out, nil
}

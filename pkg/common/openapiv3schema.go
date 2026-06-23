/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"math"
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
	sanitized := stripIntegerOverflow(openAPIV3Schema)
	out := &apiextensions.JSONSchemaProps{}
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(sanitized, out, nil); err != nil {
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

func ConvertStringToInterfaceBySchemaType(openAPIV3Schema *apiextensionsv1.JSONSchemaProps, input map[string]string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	properties := openAPIV3Schema.Properties
	convertError := func(key string, err error) error {
		return fmt.Errorf(`convert "%s" failed: %s`, key, err.Error())
	}
	var err error
	for k, v := range input {
		p, ok := properties[k]
		if !ok {
			continue
		}
		switch p.Type {
		case "integer":
			if IsUnsignedIntegerFormat(p.Format) || isUnsignedByBounds(p) {
				out[k], err = strconv.ParseUint(v, 10, 64)
			} else {
				out[k], err = strconv.ParseInt(v, 10, 64)
			}
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
			return nil, convertError(k, err)
		}
	}
	return out, nil
}

func IsUnsignedIntegerFormat(format string) bool {
	switch format {
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

// IsUnsignedInteger returns true if the schema represents an unsigned integer,
// detected by explicit Format or by CUE-generated bounds (Minimum >= 0, Maximum > 2^63).
func IsUnsignedInteger(prop apiextensionsv1.JSONSchemaProps) bool {
	return IsUnsignedIntegerFormat(prop.Format) || isUnsignedByBounds(prop)
}

// isUnsignedByBounds detects CUE-generated uint64 schemas by their bounds:
// Minimum >= 0 and Maximum above the signed int64 range.
func isUnsignedByBounds(prop apiextensionsv1.JSONSchemaProps) bool {
	return prop.Minimum != nil && *prop.Minimum >= 0 &&
		prop.Maximum != nil && *prop.Maximum > math.Exp2(63)
}

// stripIntegerOverflow removes Maximum from integer properties whose Maximum
// >= 2^63. kube-openapi internally converts float64 Maximum to int64;
// float64(MaxInt64) rounds to 2^63 and float64(MaxUint64) rounds to 2^64,
// both of which overflow int64. Stripping is safe because the integer range
// is already enforced by ParseInt/ParseUint at the conversion layer.
// User-defined maximums below 2^63 are preserved.
func stripIntegerOverflow(schema *apiextensionsv1.JSONSchemaProps) *apiextensionsv1.JSONSchemaProps {
	if schema == nil {
		return nil
	}
	needsCopy := false
	for _, prop := range schema.Properties {
		if prop.Type == "integer" && prop.Maximum != nil && *prop.Maximum >= math.Exp2(63) {
			needsCopy = true
			break
		}
	}
	if !needsCopy {
		return schema
	}
	out := schema.DeepCopy()
	for k, prop := range out.Properties {
		if prop.Type == "integer" && prop.Maximum != nil && *prop.Maximum >= math.Exp2(63) {
			prop.Maximum = nil
			prop.ExclusiveMaximum = false
			out.Properties[k] = prop
		}
	}
	return out
}

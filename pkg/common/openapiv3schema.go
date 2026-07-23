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
	"math/big"
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
	return validateLargeIntegerBounds(openAPIV3Schema, data)
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

// stripIntegerOverflow removes Maximum from integer schemas whose Maximum
// >= 2^63. kube-openapi internally converts float64 Maximum to int64;
// float64(MaxInt64) rounds to 2^63 and float64(MaxUint64) rounds to 2^64,
// both of which overflow int64. User-declared maximums that were stripped
// are enforced separately by validateLargeIntegerBounds.
func stripIntegerOverflow(schema *apiextensionsv1.JSONSchemaProps) *apiextensionsv1.JSONSchemaProps {
	if schema == nil {
		return nil
	}
	if !hasIntegerOverflowMaximum(schema, false) {
		return schema
	}
	out := schema.DeepCopy()
	stripIntegerOverflowMaximum(out, false)
	return out
}

func hasIntegerOverflowMaximum(schema *apiextensionsv1.JSONSchemaProps, inheritedInteger bool) bool {
	integerContext := hasEffectiveIntegerType(schema, inheritedInteger)
	if integerContext && schema.Maximum != nil && *schema.Maximum >= math.Exp2(63) {
		return true
	}
	for i := range schema.AllOf {
		if hasIntegerOverflowMaximum(&schema.AllOf[i], integerContext) {
			return true
		}
	}
	for k := range schema.Properties {
		prop := schema.Properties[k]
		if hasIntegerOverflowMaximum(&prop, false) {
			return true
		}
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil &&
		hasIntegerOverflowMaximum(schema.AdditionalProperties.Schema, false) {
		return true
	}
	if schema.Items != nil {
		if schema.Items.Schema != nil && hasIntegerOverflowMaximum(schema.Items.Schema, false) {
			return true
		}
		for i := range schema.Items.JSONSchemas {
			if hasIntegerOverflowMaximum(&schema.Items.JSONSchemas[i], false) {
				return true
			}
		}
	}
	return false
}

func stripIntegerOverflowMaximum(schema *apiextensionsv1.JSONSchemaProps, inheritedInteger bool) {
	integerContext := hasEffectiveIntegerType(schema, inheritedInteger)
	if integerContext && schema.Maximum != nil && *schema.Maximum >= math.Exp2(63) {
		schema.Maximum = nil
		schema.ExclusiveMaximum = false
	}
	for i := range schema.AllOf {
		stripIntegerOverflowMaximum(&schema.AllOf[i], integerContext)
	}
	for k := range schema.Properties {
		prop := schema.Properties[k]
		stripIntegerOverflowMaximum(&prop, false)
		schema.Properties[k] = prop
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		stripIntegerOverflowMaximum(schema.AdditionalProperties.Schema, false)
	}
	if schema.Items != nil {
		if schema.Items.Schema != nil {
			stripIntegerOverflowMaximum(schema.Items.Schema, false)
		}
		for i := range schema.Items.JSONSchemas {
			stripIntegerOverflowMaximum(&schema.Items.JSONSchemas[i], false)
		}
	}
}

func hasEffectiveIntegerType(schema *apiextensionsv1.JSONSchemaProps, inheritedInteger bool) bool {
	typeName, conflict := effectiveAllOfType(schema)
	if conflict {
		return false
	}
	if inheritedInteger {
		typeName, conflict = intersectJSONSchemaType("integer", typeName)
	}
	return !conflict && typeName == "integer"
}

func effectiveAllOfType(schema *apiextensionsv1.JSONSchemaProps) (string, bool) {
	if schema == nil {
		return "", false
	}
	typeName := schema.Type
	for i := range schema.AllOf {
		branchType, conflict := effectiveAllOfType(&schema.AllOf[i])
		if conflict {
			return "", true
		}
		typeName, conflict = intersectJSONSchemaType(typeName, branchType)
		if conflict {
			return "", true
		}
	}
	return typeName, false
}

func intersectJSONSchemaType(left, right string) (string, bool) {
	if left == "" {
		return right, false
	}
	if right == "" || left == right {
		return left, false
	}
	if (left == "integer" && right == "number") || (left == "number" && right == "integer") {
		return "integer", false
	}
	return "", true
}

// validateLargeIntegerBounds enforces original Maximum for integer properties
// whose Maximum was >= 2^63 (stripped by stripIntegerOverflow to prevent
// kube-openapi int64 overflow).
func validateLargeIntegerBounds(schema *apiextensionsv1.JSONSchemaProps, data interface{}) error {
	return validateLargeIntegerBoundsAt(schema, data, "", false)
}

func validateLargeIntegerBoundsAt(schema *apiextensionsv1.JSONSchemaProps, data interface{}, path string, inheritedInteger bool) error {
	if schema == nil {
		return nil
	}
	integerContext := hasEffectiveIntegerType(schema, inheritedInteger)
	if integerContext && schema.Maximum != nil && *schema.Maximum >= math.Exp2(63) {
		if cmp, ok := compareNumericValueToFloat64(data, *schema.Maximum); ok {
			if schema.ExclusiveMaximum && cmp >= 0 {
				return fmt.Errorf("%s: must be less than %g", schemaPath(path), *schema.Maximum)
			}
			if !schema.ExclusiveMaximum && cmp > 0 {
				return fmt.Errorf("%s: must be less than or equal to %g", schemaPath(path), *schema.Maximum)
			}
		}
	}
	for i := range schema.AllOf {
		if err := validateLargeIntegerBoundsAt(&schema.AllOf[i], data, path, integerContext); err != nil {
			return err
		}
	}
	dataMap, ok := data.(map[string]interface{})
	if ok {
		for k := range schema.Properties {
			val, exists := dataMap[k]
			if !exists {
				continue
			}
			prop := schema.Properties[k]
			if err := validateLargeIntegerBoundsAt(&prop, val, childSchemaPath(path, k), false); err != nil {
				return err
			}
		}
		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
			for k, val := range dataMap {
				if _, defined := schema.Properties[k]; defined {
					continue
				}
				if err := validateLargeIntegerBoundsAt(schema.AdditionalProperties.Schema, val, childSchemaPath(path, k), false); err != nil {
					return err
				}
			}
		}
	}
	dataSlice, ok := data.([]interface{})
	if !ok || schema.Items == nil {
		return nil
	}
	if schema.Items.Schema != nil {
		for i, val := range dataSlice {
			if err := validateLargeIntegerBoundsAt(schema.Items.Schema, val, fmt.Sprintf("%s[%d]", schemaPath(path), i), false); err != nil {
				return err
			}
		}
		return nil
	}
	for i := range schema.Items.JSONSchemas {
		if i >= len(dataSlice) {
			break
		}
		if err := validateLargeIntegerBoundsAt(&schema.Items.JSONSchemas[i], dataSlice[i], fmt.Sprintf("%s[%d]", schemaPath(path), i), false); err != nil {
			return err
		}
	}
	return nil
}

func childSchemaPath(path, child string) string {
	if path == "" {
		return child
	}
	return path + "." + child
}

func schemaPath(path string) string {
	if path == "" {
		return "value"
	}
	return path
}

func compareNumericValueToFloat64(val interface{}, bound float64) (int, bool) {
	switch v := val.(type) {
	case float64:
		return compareFloat64(v, bound)
	case float32:
		return compareFloat64(float64(v), bound)
	case int64:
		return compareIntegerToFloat64(big.NewInt(v), bound)
	case uint64:
		return compareIntegerToFloat64(new(big.Int).SetUint64(v), bound)
	case int:
		return compareIntegerToFloat64(big.NewInt(int64(v)), bound)
	case int32:
		return compareIntegerToFloat64(big.NewInt(int64(v)), bound)
	case uint:
		return compareIntegerToFloat64(new(big.Int).SetUint64(uint64(v)), bound)
	case uint32:
		return compareIntegerToFloat64(new(big.Int).SetUint64(uint64(v)), bound)
	}
	return 0, false
}

func compareIntegerToFloat64(value *big.Int, bound float64) (int, bool) {
	boundValue := new(big.Rat).SetFloat64(bound)
	if boundValue == nil {
		if math.IsInf(bound, 1) {
			return -1, true
		}
		if math.IsInf(bound, -1) {
			return 1, true
		}
		return 0, false
	}
	return new(big.Rat).SetInt(value).Cmp(boundValue), true
}

func compareFloat64(value, bound float64) (int, bool) {
	if math.IsNaN(value) || math.IsNaN(bound) {
		return 0, false
	}
	if value < bound {
		return -1, true
	}
	if value > bound {
		return 1, true
	}
	return 0, true
}

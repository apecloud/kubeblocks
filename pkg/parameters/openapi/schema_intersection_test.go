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

package openapi

import (
	"os"

	"cuelang.org/go/cue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("CUE schema intersection semantics", func() {
	const schemaType = "combined"

	var (
		cueTemplate string
		runtime     *Runtime
	)

	BeforeEach(func() {
		content, err := os.ReadFile(testdata.SubTestDataPath("cue_testdata/schema_intersection.cue"))
		Expect(err).NotTo(HaveOccurred())
		cueTemplate = string(content)
		runtime, err = NewRuntime(cueTemplate)
		Expect(err).NotTo(HaveOccurred())
	})

	validConfig := func() map[string]interface{} {
		return map[string]interface{}{
			"x":             int64(5),
			"requiredField": "set",
			"nested": map[string]interface{}{
				"value": int64(5),
			},
			"mapField": map[string]interface{}{
				"entry": int64(5),
			},
		}
	}

	validateCue := func(config map[string]interface{}) error {
		definition := runtime.Underlying().LookupPath(cue.MakePath(cue.Def(schemaType)))
		return definition.Unify(runtime.Context().Encode(config)).Validate(cue.Concrete(true))
	}

	validateGeneratedSchema := func(config map[string]interface{}) error {
		schema, err := GenerateOpenAPISchema(cueTemplate, schemaType)
		if err != nil {
			return err
		}
		return common.ValidateDataWithSchema(schema, map[string]interface{}{DefaultSchemaName: config})
	}

	DescribeTable("preserves every intersected constraint",
		func(mutate func(map[string]interface{}), wantValid bool) {
			config := validConfig()
			mutate(config)

			cueErr := validateCue(config)
			schemaErr := validateGeneratedSchema(config)
			if wantValid {
				Expect(cueErr).NotTo(HaveOccurred())
				Expect(schemaErr).NotTo(HaveOccurred())
				return
			}
			Expect(cueErr).To(HaveOccurred())
			Expect(schemaErr).To(HaveOccurred())
		},
		Entry("accepts a value inside both bounds", func(map[string]interface{}) {}, true),
		Entry("rejects a value below the lower bound", func(config map[string]interface{}) {
			config["x"] = int64(0)
		}, false),
		Entry("rejects a value above the upper bound", func(config map[string]interface{}) {
			config["x"] = int64(11)
		}, false),
		Entry("preserves required fields", func(config map[string]interface{}) {
			delete(config, "requiredField")
		}, false),
		Entry("preserves nested reference intersections", func(config map[string]interface{}) {
			config["nested"] = map[string]interface{}{"value": int64(11)}
		}, false),
		Entry("preserves additionalProperties intersections", func(config map[string]interface{}) {
			config["mapField"] = map[string]interface{}{"entry": int64(11)}
		}, false),
	)

	It("preserves intersected leaf constraints when flattening", func() {
		schema, err := GenerateOpenAPISchema(cueTemplate, schemaType)
		Expect(err).NotTo(HaveOccurred())
		specSchema := schema.Properties[DefaultSchemaName]
		flattened := FlattenSchema(specSchema)

		xSchema, ok := flattened.Properties["x"]
		Expect(ok).To(BeTrue())
		leafSchema := schema.DeepCopy()
		leafSchema.Properties = map[string]apiextensionsv1.JSONSchemaProps{"x": xSchema}
		leafSchema.Required = nil

		Expect(common.ValidateDataWithSchema(leafSchema, map[string]interface{}{"x": int64(5)})).To(Succeed())
		Expect(common.ValidateDataWithSchema(leafSchema, map[string]interface{}{"x": int64(0)})).NotTo(Succeed())
		Expect(common.ValidateDataWithSchema(leafSchema, map[string]interface{}{"x": int64(11)})).NotTo(Succeed())

		typedValue, err := common.ConvertStringToInterfaceBySchemaType(leafSchema, map[string]string{"x": "11"})
		Expect(err).NotTo(HaveOccurred())
		Expect(typedValue["x"]).To(Equal(int64(11)))
		Expect(common.ValidateDataWithSchema(leafSchema, typedValue)).NotTo(Succeed())
	})

	DescribeTable("converts compatible numeric intersections independently of branch order",
		func(definition string) {
			const value = "9007199254740993"

			cueDefinition := runtime.Underlying().LookupPath(cue.MakePath(cue.Def(definition)))
			Expect(cueDefinition.Unify(runtime.Context().Encode(map[string]interface{}{
				"x": int64(9007199254740993),
			})).Validate(cue.Concrete(true))).To(Succeed())

			schema, err := GenerateOpenAPISchema(cueTemplate, definition)
			Expect(err).NotTo(HaveOccurred())
			xSchema, ok := FlattenSchema(schema.Properties[DefaultSchemaName]).Properties["x"]
			Expect(ok).To(BeTrue())
			leafSchema := &apiextensionsv1.JSONSchemaProps{
				Type:       SchemaStructType,
				Properties: map[string]apiextensionsv1.JSONSchemaProps{"x": xSchema},
			}

			typedValue, err := common.ConvertStringToInterfaceBySchemaType(leafSchema, map[string]string{"x": value})
			Expect(err).NotTo(HaveOccurred())
			Expect(typedValue["x"]).To(Equal(int64(9007199254740993)))
			Expect(common.ValidateDataWithSchema(leafSchema, typedValue)).To(Succeed())
		},
		Entry("number then integer", "numberThenInteger"),
		Entry("integer then number", "integerThenNumber"),
	)

	DescribeTable("converts unsigned integer intersections independently of branch order",
		func(definition string) {
			const (
				value        = "9223372036854775808"
				integerValue = uint64(9223372036854775808)
			)

			cueDefinition := runtime.Underlying().LookupPath(cue.MakePath(cue.Def(definition)))
			Expect(cueDefinition.Unify(runtime.Context().Encode(map[string]interface{}{
				"x": integerValue,
			})).Validate(cue.Concrete(true))).To(Succeed())

			schema, err := GenerateOpenAPISchema(cueTemplate, definition)
			Expect(err).NotTo(HaveOccurred())
			xSchema, ok := FlattenSchema(schema.Properties[DefaultSchemaName]).Properties["x"]
			Expect(ok).To(BeTrue())
			leafSchema := &apiextensionsv1.JSONSchemaProps{
				Type:       SchemaStructType,
				Properties: map[string]apiextensionsv1.JSONSchemaProps{"x": xSchema},
			}

			typedValue, err := common.ConvertStringToInterfaceBySchemaType(leafSchema, map[string]string{"x": value})
			Expect(err).NotTo(HaveOccurred())
			Expect(typedValue["x"]).To(Equal(integerValue))
			Expect(common.ValidateDataWithSchema(leafSchema, typedValue)).To(Succeed())
		},
		Entry("generic integer then uint64", "genericThenUnsigned"),
		Entry("uint64 then generic integer", "unsignedThenGeneric"),
	)

	It("preserves conflicting leaf constraints when flattening", func() {
		objectSchema := apiextensionsv1.JSONSchemaProps{
			Type: SchemaStructType,
			AllOf: []apiextensionsv1.JSONSchemaProps{
				{
					Type: SchemaStructType,
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"x": {Type: "integer"},
					},
				},
				{
					Type: SchemaStructType,
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"x": {Type: "string"},
					},
				},
			},
		}
		xSchema, ok := FlattenSchema(objectSchema).Properties["x"]
		Expect(ok).To(BeTrue())
		leafSchema := &apiextensionsv1.JSONSchemaProps{
			Type:       SchemaStructType,
			Properties: map[string]apiextensionsv1.JSONSchemaProps{"x": xSchema},
		}

		typedValue, err := common.ConvertStringToInterfaceBySchemaType(leafSchema, map[string]string{"x": "5"})
		Expect(err).NotTo(HaveOccurred())
		Expect(common.ValidateDataWithSchema(leafSchema, typedValue)).NotTo(Succeed())
		Expect(common.ValidateDataWithSchema(leafSchema, map[string]interface{}{"x": int64(5)})).NotTo(Succeed())
		Expect(common.ValidateDataWithSchema(leafSchema, map[string]interface{}{"x": "value"})).NotTo(Succeed())
	})
})

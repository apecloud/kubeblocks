package datachannel

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/apis/datachannel/v1alpha1"
)

var _ = Describe("Utils", func() {
	Context("transformToChannelDefExpressWithRootObj", func() {

		se := v1alpha1.SyncObjEnvExpress{
			Name:                  "ut",
			MetaTypeConnectSymbol: ".",
			MetaObjConnectSymbol:  ",",
			Prefix:                "a_",
			Suffix:                "_b",
			MetaTypeRequired:      nil,
		}

		It("DatabaseAndTable", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.DatabaseSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "table1",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
					},
				},
				{
					Name:  "database2",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "table2",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
						{
							Name:  "table3",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
					},
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal("a_database1.table1_b,a_database2.table2_b,a_database2.table3_b"))
		})

		It("SimpleDatabase", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.DatabaseSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "table1",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
					},
				},
				{
					Name:  "database2",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "table2",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
						{
							Name:  "table3",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
					},
				}, {
					Name:  "database3",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
				},
				{
					Name:  "database4",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: true,
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal("a_database4_b"))
		})

		It("FullTypes", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.DatabaseSyncMeta,
				v1alpha1.SchemaSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "schema1",
							Type:  v1alpha1.SchemaSyncMeta,
							IsAll: false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:  "table1",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: true,
								},
								{
									Name:  "table2",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: false,
								},
							},
						},
					},
				},
				{
					Name:  "database2",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "schema2",
							Type:  v1alpha1.SchemaSyncMeta,
							IsAll: false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:  "table3",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: true,
								},
							},
						},
					},
				}, {
					Name:  "database3",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
				},
				{
					Name:  "database4",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: true,
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal("a_database1.schema1.table1_b,a_database2.schema2.table3_b"))
		})

		It("AllTypeNotExists", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.SchemaSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database3",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
				},
				{
					Name:  "database4",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: true,
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal(""))
		})

		It("MidTargetTypeNotExists", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.DatabaseSyncMeta,
				v1alpha1.SchemaSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "table1",
							Type:  v1alpha1.TableSyncMeta,
							IsAll: true,
						},
					},
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal(""))
		})

		It("MidBaseTypeNotExists", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.DatabaseSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "schema1",
							Type:  v1alpha1.SchemaSyncMeta,
							IsAll: false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:  "table1",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: true,
								},
								{
									Name:  "table2",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: false,
								},
							},
						},
					},
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal("a_database1.table1_b"))
		})

		It("FirstBaseTypeNotExists", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.SchemaSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "schema1",
							Type:  v1alpha1.SchemaSyncMeta,
							IsAll: false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:  "table1",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: true,
								},
								{
									Name:  "table2",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: false,
								},
							},
						},
					},
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, true)
			Expect(result).Should(Equal("a_schema1.table1_b"))
		})

		It("ObjectMappingCheck", func() {
			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.SchemaSyncMeta,
				v1alpha1.TableSyncMeta,
			}
			se.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:        "database1",
					MappingName: "database1_mapping",
					Type:        v1alpha1.DatabaseSyncMeta,
					IsAll:       false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:        "schema1",
							MappingName: "schema1_mapping",
							Type:        v1alpha1.SchemaSyncMeta,
							IsAll:       false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:        "table1",
									MappingName: "table1_mapping",
									Type:        v1alpha1.TableSyncMeta,
									IsAll:       true,
								},
								{
									Name:        "table2",
									MappingName: "table2_mapping",
									Type:        v1alpha1.TableSyncMeta,
									IsAll:       false,
								},
							},
						}, {
							Name:  "schema2",
							Type:  v1alpha1.SchemaSyncMeta,
							IsAll: false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:  "table3",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: true,
								},
							},
						},
					},
				},
			}
			result := transformToChannelDefExpressWithRootObj(se, so, false)
			Expect(result).Should(Equal("a_schema1_mapping.table1_mapping_b,a_schema2.table3_b"))
		})

		It("InvolvedSelectModeCheck", func() {
			newSe := se.DeepCopy()
			newSe.SelectMode = v1alpha1.InvolvedSelectMode

			metaTypeRequired := []v1alpha1.SyncMetaType{
				v1alpha1.DatabaseSyncMeta,
			}
			newSe.MetaTypeRequired = metaTypeRequired
			so := []v1alpha1.SyncMetaObject{
				{
					Name:  "database1",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: false,
					Child: []v1alpha1.SyncMetaObject{
						{
							Name:  "schema1",
							Type:  v1alpha1.SchemaSyncMeta,
							IsAll: false,
							Child: []v1alpha1.SyncMetaObject{
								{
									Name:  "table1",
									Type:  v1alpha1.TableSyncMeta,
									IsAll: true,
								},
								{
									Name:        "table2",
									MappingName: "table2_mapping",
									Type:        v1alpha1.TableSyncMeta,
									IsAll:       false,
								},
							},
						},
					},
				}, {
					Name:  "database2",
					Type:  v1alpha1.DatabaseSyncMeta,
					IsAll: true,
				},
			}
			result := transformToChannelDefExpressWithRootObj(*newSe, so, true)
			Expect(result).Should(Equal("a_database1_b,a_database2_b"))
		})

	})
})

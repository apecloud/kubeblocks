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

package trace

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("util test", func() {
	Context("objectTypeToGVK", func() {
		It("should work well", func() {
			objType := &tracev1.ObjectType{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}
			gvk, err := objectTypeToGVK(objType)
			Expect(err).Should(BeNil())
			Expect(*gvk).Should(Equal(schema.GroupVersionKind{
				Group:   tracev1.GroupVersion.Group,
				Version: tracev1.GroupVersion.Version,
				Kind:    tracev1.Kind,
			}))

			objType = nil
			gvk, err = objectTypeToGVK(objType)
			Expect(err).Should(BeNil())
			Expect(gvk).Should(BeNil())
		})
	})

	Context("objectReferenceToType", func() {
		It("should work well", func() {
			ref := &corev1.ObjectReference{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}
			objType := objectReferenceToType(ref)
			Expect(objType).ShouldNot(BeNil())
			Expect(*objType).Should(Equal(tracev1.ObjectType{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}))

			ref = nil
			objType = objectReferenceToType(ref)
			Expect(objType).Should(BeNil())
		})
	})

	Context("objectReferenceToRef", func() {
		It("should work well", func() {
			ref := &corev1.ObjectReference{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
				Namespace:  "foo",
				Name:       "bar",
			}
			objRef := objectReferenceToRef(ref)
			Expect(objRef).ShouldNot(BeNil())
			Expect(*objRef).Should(Equal(model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   tracev1.GroupVersion.Group,
					Version: tracev1.GroupVersion.Version,
					Kind:    tracev1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}))

			ref = nil
			objRef = objectReferenceToRef(ref)
			Expect(objRef).Should(BeNil())
		})
	})

	Context("objectRefToReference", func() {
		It("should work well", func() {
			objectRef := model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   tracev1.GroupVersion.Group,
					Version: tracev1.GroupVersion.Version,
					Kind:    tracev1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}
			uid := uuid.NewUUID()
			resourceVersion := "123456"
			ref := objectRefToReference(objectRef, uid, resourceVersion)
			Expect(ref).ShouldNot(BeNil())
			Expect(*ref).Should(Equal(corev1.ObjectReference{
				APIVersion:      tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:            tracev1.Kind,
				Namespace:       "foo",
				Name:            "bar",
				UID:             uid,
				ResourceVersion: resourceVersion,
			}))
		})
	})

	Context("objectRefToType", func() {
		It("should work well", func() {
			objectRef := &model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   tracev1.GroupVersion.Group,
					Version: tracev1.GroupVersion.Version,
					Kind:    tracev1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}
			t := objectRefToType(objectRef)
			Expect(t).ShouldNot(BeNil())
			Expect(*t).Should(Equal(tracev1.ObjectType{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}))
			objectRef = nil
			t = objectRefToType(objectRef)
			Expect(t).Should(BeNil())
		})
	})

	Context("getObjectRef", func() {
		It("should work well", func() {
			obj := builder.NewClusterBuilder(namespace, name).GetObject()
			objectRef, err := getObjectRef(obj, scheme.Scheme)
			Expect(err).Should(BeNil())
			Expect(*objectRef).Should(Equal(model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   kbappsv1.GroupVersion.Group,
					Version: kbappsv1.GroupVersion.Version,
					Kind:    kbappsv1.ClusterKind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: namespace,
					Name:      name,
				},
			}))
		})
	})

	Context("getObjectReference", func() {
		It("should work well", func() {
			obj := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			ref, err := getObjectReference(obj, scheme.Scheme)
			Expect(err).Should(BeNil())
			Expect(*ref).Should(Equal(corev1.ObjectReference{
				APIVersion:      kbappsv1.APIVersion,
				Kind:            kbappsv1.ClusterKind,
				Namespace:       namespace,
				Name:            name,
				UID:             uid,
				ResourceVersion: resourceVersion,
			}))
		})
	})

	Context("matchOwnerOf", func() {
		It("should work well", func() {
			owner := builder.NewClusterBuilder(namespace, name).SetUID(uid).GetObject()
			compName := "hello"
			fullCompName := fmt.Sprintf("%s-%s", owner.Name, compName)
			owned := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, owner).
				GetObject()
			matchOwner := &matchOwner{
				controller: true,
				ownerUID:   uid,
			}
			Expect(matchOwnerOf(matchOwner, owned)).Should(BeTrue())
		})
	})

	Context("parseRevision", func() {
		It("should work well", func() {
			rev := parseRevision(resourceVersion)
			Expect(rev).Should(Equal(int64(612345)))
		})
	})

	Context("parseQueryOptions", func() {
		It("should work well", func() {
			By("selector criteria")
			labels := map[string]string{
				"label1": "value1",
				"label2": "value2",
			}
			primary := builder.NewInstanceSetBuilder(namespace, name).AddLabelsInMap(labels).AddMatchLabelsInMap(labels).GetObject()
			criteria := &OwnershipCriteria{
				SelectorCriteria: &FieldPath{
					Path: "spec.selector.matchLabels",
				},
			}
			opt, err := parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			Expect(opt.matchLabels).Should(BeEquivalentTo(labels))

			By("label criteria")
			labels = map[string]string{
				"label1": "$(primary)",
				"label2": "$(primary.name)",
				"label3": "value3",
			}
			criteria = &OwnershipCriteria{
				LabelCriteria: labels,
			}
			opt, err = parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			expectedLabels := map[string]string{
				"label1": primary.Labels["label1"],
				"label2": primary.Name,
				"label3": "value3",
			}
			Expect(opt.matchLabels).Should(BeEquivalentTo(expectedLabels))

			By("specified name criteria")
			criteria = &OwnershipCriteria{
				SpecifiedNameCriteria: &FieldPath{
					Path: "metadata.name",
				},
			}
			opt, err = parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			Expect(opt.matchFields).Should(BeEquivalentTo(map[string]string{"metadata.name": primary.Name}))

			By("validation type")
			criteria = &OwnershipCriteria{
				Validation: ControllerValidation,
			}
			opt, err = parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			Expect(opt.matchOwner).ShouldNot(BeNil())
			Expect(*opt.matchOwner).Should(Equal(matchOwner{ownerUID: primary.UID, controller: true}))
		})
	})

	Context("getObjectsByGVK", func() {
		It("should work well", func() {
			gvk := &schema.GroupVersionKind{
				Group:   kbappsv1.GroupVersion.Group,
				Version: kbappsv1.GroupVersion.Version,
				Kind:    kbappsv1.ComponentKind,
			}
			opt := &queryOptions{
				matchOwner: &matchOwner{
					controller: true,
					ownerUID:   uid,
				},
			}
			owner := builder.NewClusterBuilder(namespace, name).SetUID(uid).GetObject()
			compName := "hello"
			fullCompName := fmt.Sprintf("%s-%s", owner.Name, compName)
			owned := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, owner).
				GetObject()
			k8sMock.EXPECT().Scheme().Return(scheme.Scheme).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ComponentList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentList, _ ...client.ListOption) error {
					list.Items = []kbappsv1.Component{*owned}
					return nil
				}).Times(1)

			objects, err := getObjectsByGVK(ctx, k8sMock, gvk, opt)
			Expect(err).Should(BeNil())
			Expect(objects).Should(HaveLen(1))
			Expect(objects[0]).Should(Equal(owned))
		})
	})

	Context("get objects from cache", func() {
		var (
			primary     *kbappsv1.Cluster
			secondaries []kbappsv1.Component
		)
		BeforeEach(func() {
			primary = builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			compNames := []string{"hello", "world"}
			secondaries = nil
			for _, compName := range compNames {
				fullCompName := fmt.Sprintf("%s-%s", primary.Name, compName)
				secondary := builder.NewComponentBuilder(namespace, fullCompName, "").
					SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, primary).
					SetUID(uid).
					GetObject()
				secondary.ResourceVersion = resourceVersion
				secondaries = append(secondaries, *secondary)
			}
			k8sMock.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ComponentList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentList, _ ...client.ListOption) error {
					list.Items = secondaries
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.SecretList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			componentSecondaries := []client.ObjectList{
				&workloads.InstanceSetList{},
				&corev1.ServiceList{},
				&corev1.SecretList{},
				&corev1.ConfigMapList{},
				&corev1.PersistentVolumeClaimList{},
				&rbacv1.ClusterRoleBindingList{},
				&rbacv1.RoleBindingList{},
				&corev1.ServiceAccountList{},
				&batchv1.JobList{},
				&dpv1alpha1.BackupList{},
				&dpv1alpha1.RestoreList{},
				&appsv1alpha1.ConfigurationList{},
			}
			for _, secondary := range componentSecondaries {
				k8sMock.EXPECT().
					List(gomock.Any(), secondary, gomock.Any()).
					DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						return nil
					}).Times(2)
			}
		})

		Context("getObjectTreeFromCache", func() {
			It("should work well", func() {
				tree, err := getObjectTreeFromCache(ctx, k8sMock, primary, getKBOwnershipRules())
				Expect(err).Should(BeNil())
				Expect(tree).ShouldNot(BeNil())
				Expect(tree.Primary).Should(Equal(corev1.ObjectReference{
					APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
					Kind:            kbappsv1.ClusterKind,
					Namespace:       primary.Namespace,
					Name:            primary.Name,
					UID:             primary.UID,
					ResourceVersion: primary.ResourceVersion,
				}))
				Expect(tree.Secondaries).Should(HaveLen(2))
				for i := 0; i < len(secondaries); i++ {
					Expect(tree.Secondaries[i].Primary).Should(Equal(corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ComponentKind,
						Namespace:       secondaries[i].Namespace,
						Name:            secondaries[i].Name,
						UID:             secondaries[i].UID,
						ResourceVersion: secondaries[i].ResourceVersion,
					}))
				}
			})
		})

		Context("getObjectsFromCache", func() {
			It("should work well", func() {
				objects, err := getObjectsFromCache(ctx, k8sMock, primary, getKBOwnershipRules())
				Expect(err).Should(BeNil())
				Expect(objects).Should(HaveLen(3))
				expectedObjects := make(map[model.GVKNObjKey]client.Object, len(objects))
				for _, object := range []client.Object{primary, &secondaries[0], &secondaries[1]} {
					objectRef, err := getObjectRef(object, k8sMock.Scheme())
					Expect(err).Should(BeNil())
					expectedObjects[*objectRef] = object
				}
				for key, object := range expectedObjects {
					v, ok := objects[key]
					Expect(ok).Should(BeTrue())
					Expect(v).Should(Equal(object))
				}
			})
		})
	})

	Context("Changes And Summary", func() {
		var initialObjectMap, newObjectMap map[model.GVKNObjKey]client.Object
		var objectList []client.Object

		BeforeEach(func() {
			initialObjectList := []client.Object{
				builder.NewComponentBuilder(namespace, name+"-0", "").GetObject(),
				builder.NewComponentBuilder(namespace, name+"-1", "").GetObject(),
			}
			newObjectList := []client.Object{
				builder.NewComponentBuilder(namespace, name+"-0", "").GetObject(),
				builder.NewComponentBuilder(namespace, name+"-2", "").GetObject(),
			}
			newObjectList[0].SetResourceVersion(resourceVersion)
			objectList = []client.Object{newObjectList[1], newObjectList[0], initialObjectList[1]}

			initialObjectMap = make(map[model.GVKNObjKey]client.Object, len(initialObjectList))
			newObjectMap = make(map[model.GVKNObjKey]client.Object, len(newObjectList))
			for _, object := range initialObjectList {
				objectRef, err := getObjectRef(object, scheme.Scheme)
				Expect(err).Should(BeNil())
				initialObjectMap[*objectRef] = object
			}
			for _, object := range newObjectList {
				objectRef, err := getObjectRef(object, scheme.Scheme)
				Expect(err).Should(BeNil())
				newObjectMap[*objectRef] = object
			}
		})

		Context("buildObjectSummaries", func() {
			It("should work well", func() {
				summary := buildObjectSummaries(initialObjectMap, newObjectMap)
				Expect(summary).Should(HaveLen(1))
				Expect(summary[0].ObjectType).Should(Equal(tracev1.ObjectType{
					APIVersion: kbappsv1.SchemeBuilder.GroupVersion.String(),
					Kind:       kbappsv1.ComponentKind,
				}))
				Expect(summary[0].Total).Should(BeEquivalentTo(2))
				Expect(summary[0].ChangeSummary).ShouldNot(BeNil())
				Expect(summary[0].ChangeSummary.Added).ShouldNot(BeNil())
				Expect(*summary[0].ChangeSummary.Added).Should(BeEquivalentTo(1))
				Expect(summary[0].ChangeSummary.Updated).ShouldNot(BeNil())
				Expect(*summary[0].ChangeSummary.Updated).Should(BeEquivalentTo(1))
				Expect(summary[0].ChangeSummary.Deleted).ShouldNot(BeNil())
				Expect(*summary[0].ChangeSummary.Deleted).Should(BeEquivalentTo(1))
			})
		})

		Context("buildChanges", func() {
			It("should work well", func() {
				i18n := builder.NewConfigMapBuilder(namespace, name).SetData(
					map[string]string{"en": "apps.kubeblocks.io/v1/Component/Creation=Component %s/%s is created."},
				).GetObject()
				changes := buildChanges(initialObjectMap, newObjectMap, buildDescriptionFormatter(i18n, defaultLocale, nil))
				Expect(changes).Should(HaveLen(3))
				Expect(changes[0]).Should(Equal(tracev1.ObjectChange{
					ObjectReference: corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ComponentKind,
						Namespace:       objectList[0].GetNamespace(),
						Name:            objectList[0].GetName(),
						UID:             objectList[0].GetUID(),
						ResourceVersion: objectList[0].GetResourceVersion(),
					},
					ChangeType:  tracev1.ObjectCreationType,
					Revision:    parseRevision(objectList[0].GetResourceVersion()),
					Timestamp:   changes[0].Timestamp,
					Description: fmt.Sprintf("Component %s/%s is created.", objectList[0].GetNamespace(), objectList[0].GetName()),
				}))
				Expect(changes[1]).Should(Equal(tracev1.ObjectChange{
					ObjectReference: corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ComponentKind,
						Namespace:       objectList[1].GetNamespace(),
						Name:            objectList[1].GetName(),
						UID:             objectList[1].GetUID(),
						ResourceVersion: objectList[1].GetResourceVersion(),
					},
					ChangeType:  tracev1.ObjectUpdateType,
					Revision:    parseRevision(objectList[1].GetResourceVersion()),
					Timestamp:   changes[1].Timestamp,
					Description: "Update",
				}))
				Expect(changes[2]).Should(Equal(tracev1.ObjectChange{
					ObjectReference: corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ComponentKind,
						Namespace:       objectList[2].GetNamespace(),
						Name:            objectList[2].GetName(),
						UID:             objectList[2].GetUID(),
						ResourceVersion: objectList[2].GetResourceVersion(),
					},
					ChangeType:  tracev1.ObjectDeletionType,
					Revision:    parseRevision(objectList[2].GetResourceVersion()),
					Timestamp:   changes[2].Timestamp,
					Description: "Deletion",
				}))
			})
		})
	})

	Context("get objects from store", func() {
		var (
			primary     *kbappsv1.Cluster
			secondaries []kbappsv1.Component
			store       ObjectRevisionStore
			reference   client.Object
		)
		BeforeEach(func() {
			primary = builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			compNames := []string{"hello", "world"}
			secondaries = nil
			for _, compName := range compNames {
				fullCompName := fmt.Sprintf("%s-%s", primary.Name, compName)
				secondary := builder.NewComponentBuilder(namespace, fullCompName, "").
					SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, primary).
					AddLabels(constant.AppManagedByLabelKey, constant.AppName).
					AddLabels(constant.AppInstanceLabelKey, primary.Name).
					SetUID(uid).
					GetObject()
				secondary.ResourceVersion = resourceVersion
				secondaries = append(secondaries, *secondary)
			}

			store = NewObjectStore(scheme.Scheme)
			reference = &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			Expect(store.Insert(primary, reference)).Should(Succeed())
			Expect(store.Insert(&secondaries[0], reference)).Should(Succeed())
			Expect(store.Insert(&secondaries[1], reference)).Should(Succeed())
		})

		Context("getObjectsFromTree", func() {
			It("should work well", func() {
				tree := &tracev1.ObjectTreeNode{
					Primary: corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ClusterKind,
						Namespace:       primary.Namespace,
						Name:            primary.Name,
						UID:             primary.UID,
						ResourceVersion: primary.ResourceVersion,
					},
					Secondaries: []*tracev1.ObjectTreeNode{
						{
							Primary: corev1.ObjectReference{
								APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
								Kind:            kbappsv1.ComponentKind,
								Namespace:       secondaries[0].Namespace,
								Name:            secondaries[0].Name,
								UID:             secondaries[0].UID,
								ResourceVersion: secondaries[0].ResourceVersion,
							},
						},
						{
							Primary: corev1.ObjectReference{
								APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
								Kind:            kbappsv1.ComponentKind,
								Namespace:       secondaries[1].Namespace,
								Name:            secondaries[1].Name,
								UID:             secondaries[1].UID,
								ResourceVersion: secondaries[1].ResourceVersion,
							},
						},
					},
				}

				objects, err := getObjectsFromTree(tree, store, scheme.Scheme)
				Expect(err).Should(BeNil())
				Expect(objects).Should(HaveLen(3))
				expectedObjects := make(map[model.GVKNObjKey]client.Object, len(objects))
				for _, object := range []client.Object{primary, &secondaries[0], &secondaries[1]} {
					objectRef, err := getObjectRef(object, k8sMock.Scheme())
					Expect(err).Should(BeNil())
					expectedObjects[*objectRef] = object
				}
				for key, object := range expectedObjects {
					v, ok := objects[key]
					Expect(ok).Should(BeTrue())
					Expect(v).Should(Equal(object))
				}
			})
		})

		Context("getObjectTreeWithRevision", func() {
			It("should work well", func() {
				revision := parseRevision(resourceVersion)
				tree, err := getObjectTreeWithRevision(primary, getKBOwnershipRules(), store, revision, scheme.Scheme)
				Expect(err).Should(BeNil())
				Expect(tree).ShouldNot(BeNil())
				Expect(tree.Primary).Should(Equal(corev1.ObjectReference{
					APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
					Kind:            kbappsv1.ClusterKind,
					Namespace:       primary.Namespace,
					Name:            primary.Name,
					UID:             primary.UID,
					ResourceVersion: primary.ResourceVersion,
				}))
				Expect(tree.Secondaries).Should(HaveLen(2))
				for i := 0; i < len(secondaries); i++ {
					Expect(tree.Secondaries[i].Primary).Should(Equal(corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ComponentKind,
						Namespace:       secondaries[i].Namespace,
						Name:            secondaries[i].Name,
						UID:             secondaries[i].UID,
						ResourceVersion: secondaries[i].ResourceVersion,
					}))
				}
			})
		})

		Context("deleteUnusedRevisions", func() {
			It("should work well", func() {
				changes := []tracev1.ObjectChange{{
					ObjectReference: corev1.ObjectReference{
						APIVersion:      kbappsv1.SchemeBuilder.GroupVersion.String(),
						Kind:            kbappsv1.ClusterKind,
						Namespace:       primary.GetNamespace(),
						Name:            primary.GetName(),
						UID:             primary.GetUID(),
						ResourceVersion: primary.GetResourceVersion(),
					},
					Revision: parseRevision(primary.GetResourceVersion()),
				}}
				deleteUnusedRevisions(store, changes, reference)
				Expect(store.List(&schema.GroupVersionKind{
					Group:   kbappsv1.GroupVersion.Group,
					Version: kbappsv1.GroupVersion.Version,
					Kind:    kbappsv1.ClusterKind,
				})).Should(HaveLen(0))
			})
		})
	})
})

/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ComponentParameterLister helps list ComponentParameters.
// All objects returned here must be treated as read-only.
type ComponentParameterLister interface {
	// List lists all ComponentParameters in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ComponentParameter, err error)
	// ComponentParameters returns an object that can list and get ComponentParameters.
	ComponentParameters(namespace string) ComponentParameterNamespaceLister
	ComponentParameterListerExpansion
}

// componentParameterLister implements the ComponentParameterLister interface.
type componentParameterLister struct {
	indexer cache.Indexer
}

// NewComponentParameterLister returns a new ComponentParameterLister.
func NewComponentParameterLister(indexer cache.Indexer) ComponentParameterLister {
	return &componentParameterLister{indexer: indexer}
}

// List lists all ComponentParameters in the indexer.
func (s *componentParameterLister) List(selector labels.Selector) (ret []*v1alpha1.ComponentParameter, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ComponentParameter))
	})
	return ret, err
}

// ComponentParameters returns an object that can list and get ComponentParameters.
func (s *componentParameterLister) ComponentParameters(namespace string) ComponentParameterNamespaceLister {
	return componentParameterNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ComponentParameterNamespaceLister helps list and get ComponentParameters.
// All objects returned here must be treated as read-only.
type ComponentParameterNamespaceLister interface {
	// List lists all ComponentParameters in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ComponentParameter, err error)
	// Get retrieves the ComponentParameter from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ComponentParameter, error)
	ComponentParameterNamespaceListerExpansion
}

// componentParameterNamespaceLister implements the ComponentParameterNamespaceLister
// interface.
type componentParameterNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all ComponentParameters in the indexer for a given namespace.
func (s componentParameterNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.ComponentParameter, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ComponentParameter))
	})
	return ret, err
}

// Get retrieves the ComponentParameter from the indexer for a given namespace and name.
func (s componentParameterNamespaceLister) Get(name string) (*v1alpha1.ComponentParameter, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("componentparameter"), name)
	}
	return obj.(*v1alpha1.ComponentParameter), nil
}

/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	v1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ParametersDefinitionLister helps list ParametersDefinitions.
// All objects returned here must be treated as read-only.
type ParametersDefinitionLister interface {
	// List lists all ParametersDefinitions in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ParametersDefinition, err error)
	// Get retrieves the ParametersDefinition from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ParametersDefinition, error)
	ParametersDefinitionListerExpansion
}

// parametersDefinitionLister implements the ParametersDefinitionLister interface.
type parametersDefinitionLister struct {
	indexer cache.Indexer
}

// NewParametersDefinitionLister returns a new ParametersDefinitionLister.
func NewParametersDefinitionLister(indexer cache.Indexer) ParametersDefinitionLister {
	return &parametersDefinitionLister{indexer: indexer}
}

// List lists all ParametersDefinitions in the indexer.
func (s *parametersDefinitionLister) List(selector labels.Selector) (ret []*v1alpha1.ParametersDefinition, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ParametersDefinition))
	})
	return ret, err
}

// Get retrieves the ParametersDefinition from the index for a given name.
func (s *parametersDefinitionLister) Get(name string) (*v1alpha1.ParametersDefinition, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("parametersdefinition"), name)
	}
	return obj.(*v1alpha1.ParametersDefinition), nil
}
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
	v1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ParameterDrivenConfigRenderLister helps list ParameterDrivenConfigRenders.
// All objects returned here must be treated as read-only.
type ParameterDrivenConfigRenderLister interface {
	// List lists all ParameterDrivenConfigRenders in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ParameterDrivenConfigRender, err error)
	// Get retrieves the ParameterDrivenConfigRender from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ParameterDrivenConfigRender, error)
	ParameterDrivenConfigRenderListerExpansion
}

// parameterDrivenConfigRenderLister implements the ParameterDrivenConfigRenderLister interface.
type parameterDrivenConfigRenderLister struct {
	indexer cache.Indexer
}

// NewParameterDrivenConfigRenderLister returns a new ParameterDrivenConfigRenderLister.
func NewParameterDrivenConfigRenderLister(indexer cache.Indexer) ParameterDrivenConfigRenderLister {
	return &parameterDrivenConfigRenderLister{indexer: indexer}
}

// List lists all ParameterDrivenConfigRenders in the indexer.
func (s *parameterDrivenConfigRenderLister) List(selector labels.Selector) (ret []*v1alpha1.ParameterDrivenConfigRender, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ParameterDrivenConfigRender))
	})
	return ret, err
}

// Get retrieves the ParameterDrivenConfigRender from the index for a given name.
func (s *parameterDrivenConfigRenderLister) Get(name string) (*v1alpha1.ParameterDrivenConfigRender, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("parameterdrivenconfigrender"), name)
	}
	return obj.(*v1alpha1.ParameterDrivenConfigRender), nil
}
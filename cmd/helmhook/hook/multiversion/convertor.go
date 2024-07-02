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

package multiversion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type sourceVersion interface {
	// list all resources in the source version of kind
	list(ctx context.Context, cli *versioned.Clientset, namespace string) ([]client.Object, error)

	// check if the resource is used by any other resources
	used(ctx context.Context, cli *versioned.Clientset, namespace, name string) (bool, error)
}

type targetVersion interface {
	// get a resource in the target version of kind
	get(ctx context.Context, cli *versioned.Clientset, namespace, name string) (client.Object, error)

	// convert a resource in the source version of kind to the target version
	convert(source client.Object) []client.Object
}

type convertor struct {
	kind       string
	source     sourceVersion
	target     targetVersion
	namespaces []string

	stats   conversionStats
	objects map[string]client.Object
}

func (c *convertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	namespace := []string{""}
	if len(c.namespaces) > 0 {
		namespace = c.namespaces
	}
	if err := c.precheck(ctx, cli, namespace); err != nil {
		return nil, err
	}
	c.stats.dump(c.kind)

	targets := make([]client.Object, 0)
	for name := range c.stats.toBeConverted {
		targets = append(targets, c.convert(c.objects[name])...)
	}
	return targets, nil
}

func (c *convertor) precheck(ctx context.Context, cli hook.CRClient, namespaces []string) error {
	for _, namespace := range namespaces {
		sources, err := c.source.list(ctx, cli.KBClient, namespace)
		if err != nil {
			return err
		}
		for _, obj := range sources {
			name := obj.GetName()
			if len(obj.GetNamespace()) > 0 {
				name = fmt.Sprintf("%s@%s", name, obj.GetNamespace())
			}
			c.objects[name] = obj

			used, err1 := c.source.used(ctx, cli.KBClient, obj.GetNamespace(), obj.GetName())
			if err1 != nil || !used {
				if err1 != nil {
					c.stats.error(name, err1)
				} else {
					c.stats.unused(name)
				}
				continue
			}

			exist, converted, err2 := c.checkExistedNConverted(ctx, cli.KBClient, obj)
			switch {
			case err2 != nil:
				c.stats.error(name, err2)
			case !exist:
				c.stats.convert(name)
			case converted:
				c.stats.converted(name)
			default:
				c.stats.native(name)
			}
		}
	}
	return nil
}

func (c *convertor) checkExistedNConverted(ctx context.Context, cli *versioned.Clientset, source client.Object) (bool, bool, error) {
	obj, err := c.target.get(ctx, cli, source.GetNamespace(), source.GetName())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}
	if obj.GetAnnotations() != nil {
		_, ok := obj.GetAnnotations()[convertedFromAnnotationKey]
		return true, ok, nil
	}
	return true, false, nil
}

func (c *convertor) convert(source client.Object) []client.Object {
	targets := c.target.convert(source)
	for i := range targets {
		// TODO: filter labels & annotations
		labels := func() map[string]string {
			return source.GetLabels()
		}
		annotations := func() map[string]string {
			m := map[string]string{}
			maps.Copy(m, source.GetAnnotations())
			b, _ := json.Marshal(source)
			m[convertedFromAnnotationKey] = string(b)
			return m
		}
		targets[i].SetNamespace(source.GetNamespace())
		targets[i].SetName(source.GetName())
		targets[i].SetLabels(labels())
		targets[i].SetAnnotations(annotations())
	}
	return targets
}

type conversionStats struct {
	errors        map[string]error
	unuseds       sets.Set[string]
	natives       sets.Set[string]
	beenConverted sets.Set[string]
	toBeConverted sets.Set[string]
}

func (s *conversionStats) error(name string, err error) {
	s.errors[name] = err
}

func (s *conversionStats) unused(name string) {
	s.unuseds.Insert(name)
}

func (s *conversionStats) native(name string) {
	s.natives.Insert(name)
}

func (s *conversionStats) converted(name string) {
	s.beenConverted.Insert(name)
}

func (s *conversionStats) convert(name string) {
	s.toBeConverted.Insert(name)
}

func (s *conversionStats) dump(kind string) {
	hook.Log("%s conversion to v1 status", kind)
	hook.Log("\tunused %ss: %d", kind, len(sets.List(s.unuseds)))
	hook.Log(s.doubleTableFormat(sets.List(s.unuseds)))
	hook.Log("\thas native %ss defined: %d", kind, len(sets.List(s.natives)))
	hook.Log(s.doubleTableFormat(sets.List(s.natives)))
	hook.Log("\thas been converted %ss: %d", kind, len(sets.List(s.beenConverted)))
	hook.Log(s.doubleTableFormat(sets.List(s.beenConverted)))
	hook.Log("\tto be converted %ss: %d", kind, len(sets.List(s.toBeConverted)))
	hook.Log(s.doubleTableFormat(sets.List(s.toBeConverted)))
	hook.Log("\terror occurred when perform pre-check: %d", len(s.errors))
	hook.Log(s.doubleTableFormat(maps.Keys(s.errors), s.errors))
}

func (s *conversionStats) doubleTableFormat(items []string, errors ...map[string]error) string {
	formattedErr := func(key string) string {
		if len(errors) == 0 {
			return ""
		}
		if err, ok := errors[0][key]; ok {
			return fmt.Sprintf(": %s", err.Error())
		}
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString("\t\t" + item + formattedErr(item) + "\n")
	}
	return sb.String()
}

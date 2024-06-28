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
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
)

// func used(ctx context.Context, cli hook.CRClient, cdName string, namespaces []string) (bool, error) {
//	selectors := []string{
//		fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName),
//		fmt.Sprintf("%s=%s", constant.AppNameLabelKey, cdName),
//	}
//	opts := metav1.ListOptions{
//		LabelSelector: strings.Join(selectors, ","),
//	}
//
//	used := false
//	for _, namespace := range namespaces {
//		compList, err := cli.KBClient.AppsV1alpha1().Clusters(namespace).List(ctx, opts)
//		if err != nil {
//			return false, err
//		}
//		used = used || (len(compList.Items) > 0)
//	}
//	return used, nil
// }

func checkExistedNConverted(getter func() (client.Object, error)) (bool, bool, error) {
	obj, err := getter()
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

type stat struct {
	errors        map[string]error
	unuseds       sets.Set[string]
	natives       sets.Set[string]
	beenConverted sets.Set[string]
	toBeConverted sets.Set[string]
}

func (s *stat) error(name string, err error) {
	s.errors[name] = err
}

// func (s *stat) unused(name string) {
//	s.unuseds.Insert(name)
// }

func (s *stat) native(name string) {
	s.natives.Insert(name)
}

func (s *stat) converted(name string) {
	s.beenConverted.Insert(name)
}

func (s *stat) convert(name string) {
	s.toBeConverted.Insert(name)
}

func (s *stat) dump(kind string) {
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

func (s *stat) doubleTableFormat(items []string, errors ...map[string]error) string {
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

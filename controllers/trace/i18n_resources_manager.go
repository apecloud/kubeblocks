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
	"fmt"
	"strings"
	"sync"

	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type I18nResourcesManager interface {
	ParseRaw(*corev1.ConfigMap) error
	GetFormatString(key string, locale string) string
}

type i18nResourcesManager struct {
	resources       map[string]map[string]string
	resourceMapLock sync.RWMutex

	parsedRawSet sets.Set[client.ObjectKey]
	rawSetLock   sync.Mutex
}

var defaultResourcesManager = &i18nResourcesManager{
	resources:    make(map[string]map[string]string),
	parsedRawSet: sets.New[client.ObjectKey](),
}

func (m *i18nResourcesManager) ParseRaw(cm *corev1.ConfigMap) error {
	if cm == nil {
		return nil
	}

	m.rawSetLock.Lock()
	defer m.rawSetLock.Unlock()

	if m.parsedRawSet.Has(client.ObjectKeyFromObject(cm)) {
		return nil
	}

	m.resourceMapLock.Lock()
	defer m.resourceMapLock.Unlock()

	for key, value := range cm.Data {
		_, err := language.Parse(key)
		if err != nil {
			return err
		}
		locale := strings.ToLower(key)
		resourceLocaleMap, ok := m.resources[locale]
		if !ok {
			resourceLocaleMap = make(map[string]string)
		}
		formatedStrings := strings.Split(value, "\n")
		for _, formatedString := range formatedStrings {
			if len(formatedString) == 0 {
				continue
			}
			index := strings.Index(formatedString, "=")
			if index <= 0 {
				return fmt.Errorf("can't parse string %s as a key=value pair", formatedString)
			}
			resourceKey := formatedString[:index]
			resourceValue := formatedString[index+1:]
			if len(resourceValue) == 0 {
				return fmt.Errorf("can't parse string %s as a key=value pair", formatedString)
			}
			resourceLocaleMap[resourceKey] = resourceValue
		}
		m.resources[locale] = resourceLocaleMap
	}

	m.parsedRawSet.Insert(client.ObjectKeyFromObject(cm))

	return nil
}

func (m *i18nResourcesManager) GetFormatString(key string, locale string) string {
	m.resourceMapLock.RLock()
	defer m.resourceMapLock.RUnlock()

	resourceLocaleMap, ok := m.resources[strings.ToLower(locale)]
	if !ok {
		return ""
	}
	return resourceLocaleMap[key]
}

var _ I18nResourcesManager = &i18nResourcesManager{}

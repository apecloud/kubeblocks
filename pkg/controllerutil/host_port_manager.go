/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package controllerutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	portManager *PortManager
)

func InitHostPortManager(cli client.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      viper.GetString(constant.CfgHostPortConfigMapName),
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
		},
		Data: make(map[string]string),
	}
	parsePortRange := func(item string) (int64, int64, error) {
		parts := strings.Split(item, "-")
		var (
			from int64
			to   int64
			err  error
		)
		switch len(parts) {
		case 2:
			from, err = strconv.ParseInt(parts[0], 10, 32)
			if err != nil {
				return from, to, err
			}
			to, err = strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				return from, to, err
			}
			if from > to {
				return from, to, fmt.Errorf("invalid port range %s", item)
			}
		case 1:
			from, err = strconv.ParseInt(parts[0], 10, 32)
			if err != nil {
				return from, to, err
			}
			to = from
		default:
			return from, to, fmt.Errorf("invalid port range %s", item)
		}
		return from, to, nil
	}
	parsePortRanges := func(portRanges string) ([]PortRange, error) {
		var ranges []PortRange
		for _, item := range strings.Split(portRanges, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			from, to, err := parsePortRange(item)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, PortRange{
				Min: int32(from),
				Max: int32(to),
			})
		}
		return ranges, nil
	}
	var err error
	if err = cli.Create(context.Background(), cm); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	includes, err := parsePortRanges(viper.GetString(constant.CfgHostPortIncludeRanges))
	if err != nil {
		return err
	}
	excludes, err := parsePortRanges(viper.GetString(constant.CfgHostPortExcludeRanges))
	if err != nil {
		return err
	}
	portManager, err = NewPortManager(includes, excludes, cli)
	return err
}

func GetPortManager() *PortManager {
	return portManager
}

func BuildHostPortName(clusterName, compName, containerName, portName string) string {
	return fmt.Sprintf("%s-%s-%s-%s", clusterName, compName, containerName, portName)
}

type PortManager struct {
	sync.Mutex
	cli      client.Client
	from     int32
	to       int32
	cursor   int32
	includes []PortRange
	excludes []PortRange
	used     map[int32]string
	cm       *corev1.ConfigMap
}

type PortRange struct {
	Min int32
	Max int32
}

// NewPortManager creates a new PortManager
// TODO[ziang] Putting all the port information in one configmap may have performance issues and is not secure enough.
// There is a risk of accidental deletion leading to the loss of cluster port information.
func NewPortManager(includes []PortRange, excludes []PortRange, cli client.Client) (*PortManager, error) {
	var (
		from int32
		to   int32
	)
	for _, item := range includes {
		if item.Min < from || from == 0 {
			from = item.Min
		}
		if item.Max > to {
			to = item.Max
		}
	}
	pm := &PortManager{
		cli:      cli,
		from:     from,
		to:       to,
		cursor:   from,
		includes: includes,
		excludes: excludes,
		used:     make(map[int32]string),
	}
	if err := pm.sync(); err != nil {
		return nil, err
	}
	return pm, nil
}

func (pm *PortManager) parsePort(port string) (int32, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		return 0, fmt.Errorf("port is empty")
	}
	p, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(p), nil
}

func (pm *PortManager) sync() error {
	cm := &corev1.ConfigMap{}
	objKey := types.NamespacedName{
		Name:      viper.GetString(constant.CfgHostPortConfigMapName),
		Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
	}
	if err := pm.cli.Get(context.Background(), objKey, cm); err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	used := make(map[int32]string)
	for key, item := range cm.Data {
		port, err := pm.parsePort(item)
		if err != nil {
			continue
		}
		used[port] = key
	}
	for _, item := range pm.excludes {
		for port := item.Min; port <= item.Max; port++ {
			used[port] = ""
		}
	}

	pm.cm = cm
	pm.used = used
	return nil
}

func (pm *PortManager) update(key string, port int32) error {
	var err error
	defer func() {
		if apierrors.IsConflict(err) {
			_ = pm.sync()
		}
	}()
	cm := pm.cm.DeepCopy()
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = fmt.Sprintf("%d", port)
	err = pm.cli.Update(context.Background(), cm)
	if err != nil {
		return err
	}

	pm.cm = cm
	pm.used[port] = key
	return nil
}

func (pm *PortManager) delete(keys []string) error {
	if pm.cm.Data == nil {
		return nil
	}

	var err error
	defer func() {
		if apierrors.IsConflict(err) {
			_ = pm.sync()
		}
	}()

	cm := pm.cm.DeepCopy()
	var ports []int32
	for _, key := range keys {
		value, ok := cm.Data[key]
		if !ok {
			continue
		}
		port, err := pm.parsePort(value)
		if err != nil {
			return err
		}
		ports = append(ports, port)
		delete(cm.Data, key)
	}
	err = pm.cli.Update(context.Background(), cm)
	if err != nil {
		return err
	}
	pm.cm = cm
	for _, port := range ports {
		delete(pm.used, port)
	}
	return nil
}

func (pm *PortManager) GetPort(key string) (int32, error) {
	pm.Lock()
	defer pm.Unlock()

	if value, ok := pm.cm.Data[key]; ok {
		port, err := pm.parsePort(value)
		if err != nil {
			return 0, err
		}
		return port, nil
	}
	return 0, nil
}

func (pm *PortManager) UsePort(key string, port int32) error {
	pm.Lock()
	defer pm.Unlock()
	if k, ok := pm.used[port]; ok && k != key {
		return fmt.Errorf("port %d is used by %s", port, k)
	}
	if err := pm.update(key, port); err != nil {
		return err
	}
	return nil
}

func (pm *PortManager) AllocatePort(key string) (int32, error) {
	pm.Lock()
	defer pm.Unlock()

	if value, ok := pm.cm.Data[key]; ok {
		port, err := pm.parsePort(value)
		if err != nil {
			return 0, err
		}
		return port, nil
	}

	if len(pm.used) >= int(pm.to-pm.from)+1 {
		return 0, fmt.Errorf("no available port")
	}

	for {
		if _, ok := pm.used[pm.cursor]; !ok {
			break
		}
		pm.cursor++
		if pm.cursor > pm.to {
			pm.cursor = pm.from
		}
	}
	if err := pm.update(key, pm.cursor); err != nil {
		return 0, err
	}
	return pm.cursor, nil
}

func (pm *PortManager) ReleaseByPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	pm.Lock()
	defer pm.Unlock()

	var keys []string
	for key := range pm.cm.Data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	if err := pm.delete(keys); err != nil {
		return err
	}
	return nil
}

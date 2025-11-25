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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type PortManager interface {
	GetPort(key string) (int32, error)
	UsePort(key string, port int32) error
	AllocatePort(key string) (int32, error)
	ReleaseByPrefix(prefix string) error
	PortKey(clusterName, compName, containerName, portName string) string
}

var (
	defaultPortManager PortManager
)

func GetPortManager(network *appsv1.ComponentNetwork) PortManager {
	if network == nil || !network.HostNetwork || len(network.HostPorts) == 0 {
		return defaultPortManager
	}
	return newDefinedPortManager(network.HostPorts)
}

func InitDefaultHostPortManager(cli client.Client) error {
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
	parsePortRanges := func(portRanges string) ([]portRange, error) {
		var ranges []portRange
		for _, item := range strings.Split(portRanges, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			from, to, err := parsePortRange(item)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, portRange{
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
	defaultPortManager, err = newDefaultPortManager(includes, excludes, cli)
	return err
}

type portManager struct {
	sync.Mutex
	cli      client.Client
	from     int32
	to       int32
	cursor   int32
	includes []portRange
	excludes []portRange
	used     map[int32]string
	cm       *corev1.ConfigMap
}

type portRange struct {
	Min int32
	Max int32
}

// newDefaultPortManager creates a new default port manager
// TODO[ziang] Putting all the port information in one configmap may have performance issues and is not secure enough.
// There is a risk of accidental deletion leading to the loss of cluster port information.
func newDefaultPortManager(includes []portRange, excludes []portRange, cli client.Client) (*portManager, error) {
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
	pm := &portManager{
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

func (m *portManager) PortKey(clusterName, compName, containerName, portName string) string {
	return fmt.Sprintf("%s-%s-%s-%s", clusterName, compName, containerName, portName)
}

func (m *portManager) parsePort(port string) (int32, error) {
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

func (m *portManager) sync() error {
	cm := &corev1.ConfigMap{}
	objKey := types.NamespacedName{
		Name:      viper.GetString(constant.CfgHostPortConfigMapName),
		Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
	}
	if err := m.cli.Get(context.Background(), objKey, cm); err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	used := make(map[int32]string)
	for key, item := range cm.Data {
		port, err := m.parsePort(item)
		if err != nil {
			continue
		}
		used[port] = key
	}
	for _, item := range m.excludes {
		for port := item.Min; port <= item.Max; port++ {
			used[port] = ""
		}
	}

	m.cm = cm
	m.used = used
	return nil
}

func (m *portManager) update(key string, port int32) error {
	var err error
	defer func() {
		if apierrors.IsConflict(err) {
			_ = m.sync()
		}
	}()
	cm := m.cm.DeepCopy()
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = fmt.Sprintf("%d", port)
	err = m.cli.Update(context.Background(), cm)
	if err != nil {
		return err
	}

	m.cm = cm
	m.used[port] = key
	return nil
}

func (m *portManager) delete(keys []string) error {
	if m.cm.Data == nil {
		return nil
	}

	var err error
	defer func() {
		if apierrors.IsConflict(err) {
			_ = m.sync()
		}
	}()

	cm := m.cm.DeepCopy()
	var ports []int32
	for _, key := range keys {
		value, ok := cm.Data[key]
		if !ok {
			continue
		}
		port, err := m.parsePort(value)
		if err != nil {
			return err
		}
		ports = append(ports, port)
		delete(cm.Data, key)
	}
	err = m.cli.Update(context.Background(), cm)
	if err != nil {
		return err
	}
	m.cm = cm
	for _, port := range ports {
		delete(m.used, port)
	}
	return nil
}

func (m *portManager) GetPort(key string) (int32, error) {
	m.Lock()
	defer m.Unlock()

	if value, ok := m.cm.Data[key]; ok {
		port, err := m.parsePort(value)
		if err != nil {
			return 0, err
		}
		return port, nil
	}
	return 0, nil
}

func (m *portManager) UsePort(key string, port int32) error {
	m.Lock()
	defer m.Unlock()
	if k, ok := m.used[port]; ok && k != key {
		return fmt.Errorf("port %d is used by %s", port, k)
	}
	if err := m.update(key, port); err != nil {
		return err
	}
	return nil
}

func (m *portManager) AllocatePort(key string) (int32, error) {
	m.Lock()
	defer m.Unlock()

	if value, ok := m.cm.Data[key]; ok {
		port, err := m.parsePort(value)
		if err != nil {
			return 0, err
		}
		return port, nil
	}

	if len(m.used) >= int(m.to-m.from)+1 {
		return 0, fmt.Errorf("no available port")
	}

	for {
		if _, ok := m.used[m.cursor]; !ok {
			break
		}
		m.cursor++
		if m.cursor > m.to {
			m.cursor = m.from
		}
	}
	if err := m.update(key, m.cursor); err != nil {
		return 0, err
	}
	return m.cursor, nil
}

func (m *portManager) ReleaseByPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	m.Lock()
	defer m.Unlock()

	var keys []string
	for key := range m.cm.Data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	if err := m.delete(keys); err != nil {
		return err
	}
	return nil
}

type definedPortManager struct {
	hostPorts map[string]int32
}

func (m *definedPortManager) PortKey(_, _, _, portName string) string {
	return portName
}

func (m *definedPortManager) GetPort(key string) (int32, error) {
	return m.hostPorts[key], nil
}

func (m *definedPortManager) UsePort(_ string, _ int32) error {
	return nil
}

func (m *definedPortManager) AllocatePort(key string) (int32, error) {
	port, ok := m.hostPorts[key]
	if ok {
		return port, nil
	}
	return 0, fmt.Errorf("no available port")

}

func (m *definedPortManager) ReleaseByPrefix(_ string) error {
	return nil
}

func newDefinedPortManager(hostPorts []appsv1.HostPort) *definedPortManager {
	hostPortsMap := make(map[string]int32)
	for _, hp := range hostPorts {
		hostPortsMap[hp.Name] = hp.Port
	}
	return &definedPortManager{hostPorts: hostPortsMap}
}

/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
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

// GetPortManager returns the port manager for a component. componentPortNames
// holds the port names declared by the component's own (non-kbagent)
// containers; the legacy kbagent port-name aliases are disabled for names that
// belong to a real component port.
func GetPortManager(network *appsv1.ComponentNetwork, componentPortNames sets.Set[string]) PortManager {
	if network == nil || !network.HostNetwork || len(network.HostPorts) == 0 {
		return defaultPortManager
	}
	if defaultPortManager == nil {
		return newDefinedPortManager(nil, network.HostPorts, componentPortNames)
	}
	return newDefinedPortManager(defaultPortManager.(*portManager), network.HostPorts, componentPortNames)
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

	// adopt an allocation recorded under the legacy kbagent port-name key, so
	// that workloads created before the port rename keep their numeric ports
	if legacyKey := legacyKBAgentPortKey(key); legacyKey != "" {
		if value, ok := m.cm.Data[legacyKey]; ok {
			port, err := m.parsePort(value)
			if err != nil {
				return 0, err
			}
			if err := m.update(key, port); err != nil {
				return 0, err
			}
			return port, nil
		}
	}

	if len(m.used) >= int(m.to-m.from)+1 {
		return 0, fmt.Errorf("no available port: %s", key)
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
	if len(keys) > 0 {
		return m.delete(keys)
	}
	return nil
}

type definedPortManager struct {
	defaultPortManager *portManager
	hostPorts          map[string]int32
	// componentPortNames are port names owned by the component's own
	// containers; a legacy kbagent alias must not consume their mappings.
	componentPortNames sets.Set[string]
}

func (m *definedPortManager) PortKey(clusterName, compName, containerName, portName string) string {
	if m.isKBAgentPortNNotDefined(containerName, portName) {
		return m.defaultPortManager.PortKey(clusterName, compName, containerName, portName)
	}
	return portName
}

func (m *definedPortManager) GetPort(key string) (int32, error) {
	if m.isKBAgentPortNNotDefinedInKey(key) {
		return m.defaultPortManager.GetPort(key)
	}
	if port, ok := m.definedKBAgentPort(key); ok {
		return port, nil
	}
	return m.hostPorts[key], nil
}

func (m *definedPortManager) UsePort(key string, port int32) error {
	if m.isKBAgentPortNNotDefinedInKey(key) {
		return m.defaultPortManager.UsePort(key, port)
	}
	return nil
}

func (m *definedPortManager) AllocatePort(key string) (int32, error) {
	if port, ok := m.definedKBAgentPort(key); ok {
		return port, nil
	}
	if m.isKBAgentPortNNotDefinedInKey(key) {
		return m.defaultPortManager.AllocatePort(key)
	}
	return 0, fmt.Errorf("no available port: %s", key)

}

func (m *definedPortManager) ReleaseByPrefix(prefix string) error {
	// Release must not depend on legacy-alias resolution: the deletion path
	// builds this manager without the component-port context, so
	// hasKBAgentPortDefined() can resolve aliases differently from allocation
	// and skip entries the default manager actually allocated (leaking host
	// ports). Dynamic allocations live only in the default manager and are
	// keyed by the component prefix, so releasing unconditionally is safe and
	// idempotent: fully user-defined setups simply have no entries there.
	if m.defaultPortManager == nil {
		return nil
	}
	return m.defaultPortManager.ReleaseByPrefix(prefix)
}

// kbagentLegacyPortNames maps the current kbagent port names to the names used
// before the rename, which existing specs may still reference.
var kbagentLegacyPortNames = map[string]string{
	kbagent.DefaultHTTPPortName:      kbagent.LegacyHTTPPortName,
	kbagent.DefaultStreamingPortName: kbagent.LegacyStreamingPortName,
}

// legacyKBAgentPortKey rewrites an allocation key that refers to a kbagent
// port by its current name into the key used before the port rename, or
// returns "" when the key does not refer to a kbagent port.
func legacyKBAgentPortKey(key string) string {
	for portName, legacy := range kbagentLegacyPortNames {
		suffix := fmt.Sprintf("-%s-%s", kbagent.ContainerName, portName)
		if strings.HasSuffix(key, suffix) {
			return strings.TrimSuffix(key, suffix) + fmt.Sprintf("-%s-%s", kbagent.ContainerName, legacy)
		}
	}
	return ""
}

func (m *definedPortManager) isKBAgentPort(containerName, portName string) bool {
	return containerName == kbagent.ContainerName && (portName == kbagent.DefaultHTTPPortName || portName == kbagent.DefaultStreamingPortName)
}

// definedKBAgentPort resolves a user-defined host port for a kbagent port
// name, accepting the legacy name as an alias for specs written before the
// kbagent ports were renamed.
func (m *definedPortManager) definedKBAgentPort(portName string) (int32, bool) {
	if port, ok := m.hostPorts[portName]; ok {
		return port, true
	}
	if legacy, ok := kbagentLegacyPortNames[portName]; ok {
		// a mapping under the legacy name is only a kbagent alias when the
		// name cannot refer to a real component port
		if m.componentPortNames.Has(legacy) {
			return 0, false
		}
		if port, ok := m.hostPorts[legacy]; ok {
			return port, true
		}
	}
	return 0, false
}

func (m *definedPortManager) hasKBAgentPortDefined() bool {
	_, http := m.definedKBAgentPort(kbagent.DefaultHTTPPortName)
	_, stream := m.definedKBAgentPort(kbagent.DefaultStreamingPortName)
	return http && stream
}

func (m *definedPortManager) isKBAgentPortNNotDefined(containerName, portName string) bool {
	if !m.isKBAgentPort(containerName, portName) {
		return false
	}
	_, defined := m.definedKBAgentPort(portName)
	return !defined
}

func (m *definedPortManager) isKBAgentPortNNotDefinedInKey(key string) bool {
	// the port names contain dashes, so match the "<container>-<port>" key
	// suffix instead of splitting the key
	for _, portName := range []string{kbagent.DefaultHTTPPortName, kbagent.DefaultStreamingPortName} {
		if strings.HasSuffix(key, fmt.Sprintf("-%s-%s", kbagent.ContainerName, portName)) {
			return m.isKBAgentPortNNotDefined(kbagent.ContainerName, portName)
		}
	}
	return false
}

func newDefinedPortManager(defaultPortManager *portManager, hostPorts []appsv1.HostPort, componentPortNames sets.Set[string]) *definedPortManager {
	hostPortsMap := make(map[string]int32)
	for _, hp := range hostPorts {
		hostPortsMap[hp.Name] = hp.Port
	}
	return &definedPortManager{
		defaultPortManager: defaultPortManager,
		hostPorts:          hostPortsMap,
		componentPortNames: componentPortNames,
	}
}

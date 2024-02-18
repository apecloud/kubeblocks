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

package util

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	probing "github.com/prometheus-community/pro-bing"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var pingerLogger = ctlruntime.Log.WithName("pinger")

// IsDNSReady checks if dns and ip is ready, it can successfully resolve dns
func IsDNSReady(dns string) (bool, error) {
	if dns == "" {
		return false, errors.New("the dns cannot have a length of zero")
	}
	pinger, err := probing.NewPinger(dns)
	if err != nil {
		err = errors.Wrap(err, "new pinger failed")
		return false, err
	}

	pinger.Count = 3
	pinger.Timeout = 3 * time.Second
	pinger.Interval = 500 * time.Millisecond
	err = pinger.Run()
	if err != nil {
		// For container runtimes like Containerd, unprivileged users can't send icmp echo packets.
		// As a temporary workaround, special handling is being implemented to bypass this limitation.
		if strings.Contains(err.Error(), "socket: permission denied") {
			pingerLogger.Info("ping failed, socket: permission denied, but temporarily return true")
			time.Sleep(10 * time.Second)
			return true, nil
		}
		err = errors.Wrapf(err, "ping dns:%s failed", dns)
		return false, err
	}

	return true, nil
}

func CheckDNSReadyWithRetry(dns string, times int) (bool, error) {
	var ready bool
	var err error
	for i := 0; i < times; i++ {
		ready, err = IsDNSReady(dns)
		if err == nil && ready {
			return true, nil
		}
	}
	pingerLogger.Info("dns resolution is ready", "dns", dns)
	return ready, err
}

// WaitForDNSReady checks if dns is ready
func WaitForDNSReady(dns string) {
	pingerLogger.Info("Waiting for dns resolution to be ready")
	isPodReady, err := IsDNSReady(dns)
	for err != nil || !isPodReady {
		if err != nil {
			pingerLogger.Info("dns check failed", "error", err.Error())
		}
		time.Sleep(3 * time.Second)
		isPodReady, err = IsDNSReady(dns)
	}
	pingerLogger.Info("dns resolution is ready", "dns", dns)
}

// WaitForPodReady checks if pod is ready
func WaitForPodReady(checkPodHeadless bool) {
	domain := viper.GetString(constant.KBEnvPodFQDN)
	WaitForDNSReady(domain)
	if checkPodHeadless {
		domain := viper.GetString(constant.KBEnvPodName)
		WaitForDNSReady(domain)
	}
}

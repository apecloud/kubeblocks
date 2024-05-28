/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"net"
	"time"

	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var pingerLogger = ctlruntime.Log.WithName("pinger")

// IsDNSReady checks if dns and ip is ready, it can successfully resolve dns.
// Since the vast majority of container runtimes currently prohibit the NET_RAW capability,
// we can't rely on ICMP protocol to detect DNS resolution.
// Instead, we directly depend on TCP for port detection.
func IsDNSReady(dns string) (bool, error) {
	// get the port where the Lorry HTTP service is listening
	port := viper.GetString("port")
	return IsTCPReady(dns, port)
}

func IsTCPReady(host, port string) (bool, error) {
	address := net.JoinHostPort(host, port)
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = conn.Close()
	}()

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

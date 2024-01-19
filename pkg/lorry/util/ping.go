package util

import (
	"strings"
	"time"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	"github.com/pkg/errors"
	probing "github.com/prometheus-community/pro-bing"
	ctlruntime "sigs.k8s.io/controller-runtime"
)

var pingerLogger = ctlruntime.Log.WithName("pinger")

// IsDNSReady checks if dns and ip is ready, it can successfully resolve dns
func IsDNSReady(dns string) (bool, error) {
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
	return ready, err
}

// WaitForPodReady checks if pod is ready
func WaitForPodReady() {
	domain := viper.GetString("KB_POD_FQDN")
	isPodReady, err := IsDNSReady(domain)
	for err != nil || !isPodReady {
		pingerLogger.Info("Waiting for dns resolution to be ready")
		time.Sleep(3 * time.Second)
		isPodReady, err = IsDNSReady(domain)
	}
	pingerLogger.Info("dns resolution is ready")
}

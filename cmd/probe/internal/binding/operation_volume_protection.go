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

package binding

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

const (
	kubeletStatsSummaryURL = "https://%s:%s/stats/summary"

	certFile  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	envNamespace      = "KB_NAMESPACE"
	envHostIP         = "KB_HOST_IP"
	envNodeName       = "KB_NODENAME"
	envPodName        = "KB_POD_NAME"
	envPodUID         = "KB_POD_UID"
	envVolumesToProbe = "KB_VOLUME_PROTECTION_SPEC"

	reasonLock   = "HighVolumeWatermark"
	reasonUnlock = "LowVolumeWatermark" // TODO
)

type operationVolumeProtection struct {
	Logger               logger.Logger
	Client               *http.Client
	Request              *http.Request
	Pod                  string
	VolumeProtectionSpec appsv1alpha1.VolumeProtectionSpec
	VolumeStats          map[string]statsv1alpha1.VolumeStats
	Readonly             bool

	// TODO: hack it here, remove it later
	BaseOperation *BaseOperations
}

func (o *operationVolumeProtection) Kind() bindings.OperationKind {
	return VolumeProtection
}

func (o *operationVolumeProtection) Init(metadata bindings.Metadata) error {
	var err error
	if o.Client, err = httpClient(); err != nil {
		o.Logger.Warnf("build HTTP client error at setup: %s", err.Error())
		return err
	}

	if o.Request, err = httpRequest(context.Background()); err != nil {
		o.Logger.Warnf("build HTTP request error at setup, will try it later: %s", err.Error())
	}

	o.Pod = os.Getenv(envPodName)
	if err = o.initVolumes(); err != nil {
		o.Logger.Warnf("init volumes to monitor error: %s", err.Error())
		return err
	}
	o.Logger.Infof("succeed to init %s operation, pod: %s, low watermark: %d, high watermark: %d, volumes: %s",
		o.Kind(), o.Pod, o.VolumeProtectionSpec.LowWatermark, o.VolumeProtectionSpec.HighWatermark,
		strings.Join(o.VolumeProtectionSpec.Volumes, ","))
	return nil
}

func (o *operationVolumeProtection) initVolumes() error {
	spec := &o.VolumeProtectionSpec
	raw := os.Getenv(envVolumesToProbe)
	if err := json.Unmarshal([]byte(raw), spec); err != nil {
		o.Logger.Warnf("unmarshal volume protection spec error: %s, raw spec: %s", err.Error(), raw)
		return err
	}
	normalizeWatermarks(&o.VolumeProtectionSpec.LowWatermark, &o.VolumeProtectionSpec.HighWatermark)

	if o.VolumeStats == nil {
		o.VolumeStats = make(map[string]statsv1alpha1.VolumeStats)
	}
	for _, name := range spec.Volumes {
		o.VolumeStats[name] = statsv1alpha1.VolumeStats{
			Name: name,
		}
	}
	return nil
}

func (o *operationVolumeProtection) Invoke(ctx context.Context, req *bindings.InvokeRequest, rsp *bindings.InvokeResponse) error {
	if o.disabled() {
		o.Logger.Infof("The operation %s is disabled", o.Kind())
		return nil
	}
	if o.Client == nil {
		return fmt.Errorf("HTTP client for kubelet is unavailable")
	}
	if o.Request == nil {
		// try to build http request again
		var err error
		o.Request, err = httpRequest(ctx)
		if err != nil {
			o.Logger.Warnf("build HTTP request to query kubelet error: %s", err.Error())
			return err
		}
	}

	summary, err := o.request(ctx)
	if err != nil {
		o.Logger.Warnf("request stats summary from kubelet error: %s", err.Error())
		return err
	}

	if err = o.updateVolumeStats(summary); err != nil {
		return err
	}

	msg, err := o.checkUsage(ctx)
	if err == nil {
		rsp.Data = []byte(msg)
	}
	return err
}

func (o *operationVolumeProtection) disabled() bool {
	skip := func(watermark int) bool {
		return watermark <= 0 || watermark >= 100
	}
	// TODO: check the role and skip secondary instances.
	return len(o.Pod) == 0 || len(o.VolumeProtectionSpec.Volumes) == 0 ||
		skip(o.VolumeProtectionSpec.LowWatermark) || skip(o.VolumeProtectionSpec.HighWatermark)
}

func (o *operationVolumeProtection) request(ctx context.Context) ([]byte, error) {
	req := o.Request.WithContext(ctx)
	rsp, err := o.Client.Do(req)
	if err != nil {
		o.Logger.Warnf("issue request to kubelet error: %s", err.Error())
		return nil, err
	}
	if rsp.StatusCode != 200 {
		o.Logger.Warnf("HTTP response from kubelet error: %s", rsp.Status)
		return nil, fmt.Errorf(rsp.Status)
	}

	defer rsp.Body.Close()
	return io.ReadAll(rsp.Body)
}

func (o *operationVolumeProtection) updateVolumeStats(payload []byte) error {
	summary := &statsv1alpha1.Summary{}
	if err := json.Unmarshal(payload, summary); err != nil {
		o.Logger.Warnf("stats summary obtained from kubelet error: %s", err.Error())
		return err
	}
	for _, pod := range summary.Pods {
		if pod.PodRef.Name == o.Pod {
			for _, stats := range pod.VolumeStats {
				if _, ok := o.VolumeStats[stats.Name]; !ok {
					continue
				}
				o.VolumeStats[stats.Name] = stats
			}
			break
		}
	}
	return nil
}

func (o *operationVolumeProtection) checkUsage(ctx context.Context) (string, error) {
	lower := make([]string, 0)
	higher := make([]string, 0)
	for name, stats := range o.VolumeStats {
		ret := o.checkVolumeWatermark(stats)
		if ret == 0 {
			continue
		}
		if ret < 0 {
			lower = append(lower, name)
		} else {
			higher = append(higher, name)
		}
	}

	var msg string
	readonly := o.Readonly
	// the instance is running normally and there have volume(s) over the space usage threshold.
	if !readonly && len(higher) > 0 {
		msg = o.buildEventMsg(higher)
		if err := o.highWatermark(ctx, msg); err != nil {
			return "", err
		}
	}
	// the instance is protected in RO mode, and all volumes' space usage are under the threshold.
	if readonly && len(lower) == len(o.VolumeStats) {
		msg = o.buildEventMsg(lower)
		if err := o.lowWatermark(ctx, msg); err != nil {
			return "", err
		}
	}

	if len(msg) == 0 {
		msg = o.buildEventMsg(o.VolumeProtectionSpec.Volumes)
	}
	return msg, nil
}

func (o *operationVolumeProtection) checkVolumeWatermark(stats statsv1alpha1.VolumeStats) int {
	if stats.AvailableBytes == nil || stats.UsedBytes == nil {
		return 0
	}

	lowThresholdBytes := *stats.AvailableBytes / 100 * uint64(o.VolumeProtectionSpec.LowWatermark)
	if *stats.UsedBytes < lowThresholdBytes {
		return -1
	}
	highThresholdBytes := *stats.AvailableBytes / 100 * uint64(o.VolumeProtectionSpec.HighWatermark)
	if *stats.UsedBytes > highThresholdBytes {
		return 1
	}
	return 0
}

func (o *operationVolumeProtection) highWatermark(ctx context.Context, msg string) error {
	if o.Readonly { // double check
		return nil
	}
	if err := o.lockInstance(ctx); err != nil {
		o.Logger.Warnf("set instance to read-only error: %s, volumes: %s", err.Error(), msg)
		return err
	}
	o.Logger.Infof("set instance to read-only OK: %s", msg)
	if err := o.sendEvent(ctx, reasonLock, msg); err != nil {
		o.Logger.Warnf("send volume protection (lock) event error: %s, volumes: %s", err.Error(), msg)
		return err
	}
	o.Readonly = true
	return nil
}

func (o *operationVolumeProtection) lowWatermark(ctx context.Context, msg string) error {
	if !o.Readonly { // double check
		return nil
	}
	if err := o.unlockInstance(ctx); err != nil {
		o.Logger.Warnf("reset instance to read-write error: %s, volumes: %s", err.Error(), msg)
		return err
	}
	o.Logger.Infof("reset instance to read-write OK: %s", msg)
	if err := o.sendEvent(ctx, reasonUnlock, msg); err != nil {
		o.Logger.Warnf("send volume protection (unlock) event error: %s, volumes: %s", err.Error(), msg)
		return err
	}
	o.Readonly = false
	return nil
}

func (o *operationVolumeProtection) lockInstance(ctx context.Context) error {
	return o.BaseOperation.LockInstance(ctx)
}

func (o *operationVolumeProtection) unlockInstance(ctx context.Context) error {
	return o.BaseOperation.UnlockInstance(ctx)
}

func (o *operationVolumeProtection) buildEventMsg(volumes []string) string {
	usages := make(map[string]string)
	for _, v := range volumes {
		stats := o.VolumeStats[v]
		if stats.UsedBytes != nil && stats.AvailableBytes != nil {
			usages[v] = fmt.Sprintf("%d%%", int(*stats.UsedBytes*100 / *stats.AvailableBytes))
		} else {
			usages[v] = "<nil>"
		}
	}
	msg, _ := json.Marshal(usages)
	return string(msg)
}

func (o *operationVolumeProtection) sendEvent(ctx context.Context, reason, msg string) error {
	return sendEvent(ctx, o.Logger, o.createEvent(reason, msg))
}

func (o *operationVolumeProtection) createEvent(reason, msg string) *corev1.Event {
	return &corev1.Event{
		//Name: "",
		//Namespace: ","
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: os.Getenv(envNamespace),
			Name:      os.Getenv(envPodName),
			UID:       types.UID(os.Getenv(envPodUID)),
			FieldPath: "spec.containers{sqlchannel}",
		},
		Reason:  reason,
		Message: msg,
		Source: corev1.EventSource{
			Component: "sqlchannel",
			Host:      os.Getenv(envNodeName),
		},
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		Type:           "Normal",
	}
}

func httpClient() (*http.Client, error) {
	cert, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			},
		},
	}, nil
}

func httpRequest(ctx context.Context) (*http.Request, error) {
	host, err := kubeletEndpointHost(ctx)
	if err != nil {
		return nil, err
	}
	port, err := kubeletEndpointPort(ctx)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(kubeletStatsSummaryURL, host, port)

	accessToken, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if len(accessToken) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}
	return req, nil
}

func kubeletEndpointHost(ctx context.Context) (string, error) {
	return os.Getenv(envHostIP), nil
}

func kubeletEndpointPort(ctx context.Context) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	cliset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	node, err := cliset.CoreV1().Nodes().Get(ctx, os.Getenv(envNodeName), metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return strconv.Itoa(int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)), nil
}

func normalizeWatermarks(low, high *int) {
	if *low < 0 || *low > 100 {
		*low = 0
	}
	if *high < 0 || *high > 100 {
		*high = 0
	}
	if *low == 0 && *high != 0 {
		*low = *high
	}
	if *low != 0 && *high == 0 {
		*high = *low
	}
	if *low > *high {
		*low = *high
	}
}

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

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/lorry/component"
	. "github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	kubeletStatsSummaryURL = "https://%s:%s/stats/summary"

	certFile  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	reasonLock   = "HighVolumeWatermark"
	reasonUnlock = "LowVolumeWatermark" // TODO
)

type volumeStatsRequester interface {
	init(ctx context.Context) error
	request(ctx context.Context) ([]byte, error)
}

type volumeExt struct {
	Name          string
	HighWatermark int
	Stats         statsv1alpha1.VolumeStats
}

type operationVolumeProtection struct {
	Logger        logr.Logger
	Requester     volumeStatsRequester
	Pod           string
	HighWatermark int
	Volumes       map[string]volumeExt
	Readonly      bool
	SendEvent     bool // to disable event for testing
}

var (
	logVolProt logr.Logger
	optVolProt *operationVolumeProtection
)

func init() {
	production, _ := zap.NewProduction()
	logVolProt = zapr.NewLogger(production)
	optVolProt = &operationVolumeProtection{
		Logger: logVolProt,
		Requester: &httpsVolumeStatsRequester{
			logger: logVolProt,
		},
		SendEvent: true,
	}

	if viper.GetBool("VOLUME_PROTECT_LOG_DISABLED") {
		optVolProt.Logger = zapr.NewLogger(zap.New(zapcore.NewNopCore()))
	}

	if err := optVolProt.Requester.init(context.Background()); err != nil {
		optVolProt.Logger.Info("init requester failed", "error", err)
		return
	}

	optVolProt.Pod = os.Getenv(constant.KBEnvPodName)
	if err := optVolProt.initVolumes(); err != nil {
		optVolProt.Logger.Info("init volumes to monitor failed", "error", err)
	}
	optVolProt.Logger.Info(fmt.Sprintf("succeed to init volume protection, pod: %s, spec: %s", optVolProt.Pod, optVolProt.buildVolumesMsg()))
}

func (ops *BaseOperations) VolumeProtectionOps(ctx context.Context, req *ProbeRequest, rsp *ProbeResponse) (OpsResult, error) {
	if optVolProt.disabled() {
		ops.Logger.Info("The volume protection operation is disabled")
		return nil, nil
	}

	summary, err := optVolProt.Requester.request(ctx)
	if err != nil {
		ops.Logger.Error(err, "request stats summary from kubelet error")
		return nil, err
	}

	if err = optVolProt.updateVolumeStats(summary); err != nil {
		return nil, err
	}

	msg, err := optVolProt.checkUsage(ctx)
	if err == nil {
		rsp.Data = []byte(msg)
	}
	return nil, err
}

func (ops *BaseOperations) LockOps(ctx context.Context, req *ProbeRequest, rsp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	manager, err := component.GetDefaultManager()
	if err != nil || manager == nil {
		msg := fmt.Sprintf("Get DB manager failed: %v", err)
		ops.Logger.Info(msg)
		opsRes["event"] = OperationFailed
		opsRes["message"] = msg
		return opsRes, nil
	}

	err = manager.Lock(ctx, "disk full")
	if err != nil {
		msg := fmt.Sprintf("Lock DB failed: %v", err)
		ops.Logger.Info(msg)
		opsRes["event"] = OperationFailed
		opsRes["message"] = msg
		return opsRes, nil
	}
	opsRes["event"] = OperationSuccess
	return opsRes, nil
}

func (ops *BaseOperations) UnlockOps(ctx context.Context, req *ProbeRequest, rsp *ProbeResponse) (OpsResult, error) {
	opsRes := OpsResult{}
	manager, err := component.GetDefaultManager()
	if err != nil || manager == nil {
		msg := fmt.Sprintf("Get DB manager failed: %v", err)
		ops.Logger.Info(msg)
		opsRes["event"] = OperationFailed
		opsRes["message"] = msg
		return opsRes, nil
	}

	err = manager.Unlock(ctx)
	if err != nil {
		msg := fmt.Sprintf("Unlock DB failed: %v", err)
		ops.Logger.Info(msg)
		opsRes["event"] = OperationFailed
		opsRes["message"] = msg
		return opsRes, nil
	}
	opsRes["event"] = OperationSuccess
	return opsRes, nil
}

func (o *operationVolumeProtection) initVolumes() error {
	spec := &appsv1alpha1.VolumeProtectionSpec{}
	raw := os.Getenv(constant.KBEnvVolumeProtectionSpec)
	if err := json.Unmarshal([]byte(raw), spec); err != nil {
		o.Logger.Error(err, "unmarshal volume protection spec error", "raw spec", raw)
		return err
	}

	o.HighWatermark = normalizeVolumeWatermark(&spec.HighWatermark, 0)

	if o.Volumes == nil {
		o.Volumes = make(map[string]volumeExt)
	}
	for _, v := range spec.Volumes {
		o.Volumes[v.Name] = volumeExt{
			Name:          v.Name,
			HighWatermark: normalizeVolumeWatermark(v.HighWatermark, o.HighWatermark),
			Stats: statsv1alpha1.VolumeStats{
				Name: v.Name,
			},
		}
	}
	return nil
}

func (o *operationVolumeProtection) disabled() bool {
	// TODO: check the role and skip secondary instances.
	if len(o.Pod) == 0 || len(o.Volumes) == 0 {
		return true
	}
	for _, v := range o.Volumes {
		// take (0, 100] as enabled
		if v.HighWatermark > 0 && v.HighWatermark <= 100 {
			return false
		}
	}
	return true
}

func (o *operationVolumeProtection) updateVolumeStats(payload []byte) error {
	summary := &statsv1alpha1.Summary{}
	if err := json.Unmarshal(payload, summary); err != nil {
		o.Logger.Error(err, "stats summary obtained from kubelet error")
		return err
	}
	for _, pod := range summary.Pods {
		if pod.PodRef.Name == o.Pod {
			for _, stats := range pod.VolumeStats {
				if _, ok := o.Volumes[stats.Name]; !ok {
					continue
				}
				v := o.Volumes[stats.Name]
				v.Stats = stats
				o.Volumes[stats.Name] = v
			}
			break
		}
	}
	return nil
}

func (o *operationVolumeProtection) checkUsage(ctx context.Context) (string, error) {
	lower := make([]string, 0)
	higher := make([]string, 0)
	for name, v := range o.Volumes {
		ret := o.checkVolumeWatermark(v)
		if ret == 0 {
			lower = append(lower, name)
		} else {
			higher = append(higher, name)
		}
	}

	msg := o.buildVolumesMsg()
	readonly := o.Readonly
	// the instance is running normally and there have volume(s) over the space usage threshold.
	if !readonly && len(higher) > 0 {
		if err := o.highWatermark(ctx, msg); err != nil {
			return "", err
		}
	}
	// the instance is protected in RO mode, and all volumes' space usage are under the threshold.
	if readonly && len(lower) == len(o.Volumes) {
		if err := o.lowWatermark(ctx, msg); err != nil {
			return "", err
		}
	}
	return msg, nil
}

// checkVolumeWatermark checks whether the volume's space usage is over the threshold.
//
//	returns 0 if the volume will not be taken in account or its space usage is under the threshold
//	returns non-zero if the volume space usage is over the threshold
func (o *operationVolumeProtection) checkVolumeWatermark(v volumeExt) int {
	if v.HighWatermark == 0 { // disabled
		return 0
	}
	if v.Stats.CapacityBytes == nil || v.Stats.UsedBytes == nil {
		return 0
	}
	thresholdBytes := *v.Stats.CapacityBytes / 100 * uint64(v.HighWatermark)
	if *v.Stats.UsedBytes < thresholdBytes {
		return 0
	}
	return 1
}

func (o *operationVolumeProtection) highWatermark(ctx context.Context, msg string) error {
	if o.Readonly { // double check
		return nil
	}
	if err := o.lockInstance(ctx); err != nil {
		o.Logger.Error(err, "set instance to read-only error", "volumes", msg)
		return err
	}

	o.Logger.Info("set instance to read-only OK", "msg", msg)
	o.Readonly = true

	if err := o.sendEvent(ctx, reasonLock, msg); err != nil {
		o.Logger.Error(err, "send volume protection (lock) event error", "volumes", msg)
		return err
	}
	return nil
}

func (o *operationVolumeProtection) lowWatermark(ctx context.Context, msg string) error {
	if !o.Readonly { // double check
		return nil
	}
	if err := o.unlockInstance(ctx); err != nil {
		o.Logger.Error(err, "reset instance to read-write error", "volumes", msg)
		return err
	}

	o.Logger.Info("reset instance to read-write OK", "msg", msg)
	o.Readonly = false

	if err := o.sendEvent(ctx, reasonUnlock, msg); err != nil {
		o.Logger.Error(err, "send volume protection (unlock) event error", "volumes", msg)
		return err
	}
	return nil
}

func (o *operationVolumeProtection) lockInstance(ctx context.Context) error {
	manager, err := component.GetDefaultManager()
	if err != nil || manager == nil {
		o.Logger.Error(err, "Get DB manager failed")
	}
	return manager.Lock(ctx, "disk full")
}

func (o *operationVolumeProtection) unlockInstance(ctx context.Context) error {
	manager, err := component.GetDefaultManager()
	if err != nil || manager == nil {
		o.Logger.Error(err, "Get DB manager failed")
	}
	return manager.Unlock(ctx)
}

func (o *operationVolumeProtection) buildVolumesMsg() string {
	volumes := make([]map[string]string, 0)
	for _, v := range o.Volumes {
		usage := make(map[string]string)
		if v.HighWatermark != o.HighWatermark {
			usage["highWatermark"] = fmt.Sprintf("%d", v.HighWatermark)
		}
		stats := v.Stats
		if stats.UsedBytes == nil || stats.CapacityBytes == nil {
			usage[v.Name] = "<nil>"
		} else {
			usage[v.Name] = fmt.Sprintf("%d%%", int(*stats.UsedBytes*100 / *stats.CapacityBytes))
		}
		volumes = append(volumes, usage)
	}
	usages := map[string]any{
		"highWatermark": fmt.Sprintf("%d", o.HighWatermark),
		"volumes":       volumes,
	}
	msg, _ := json.Marshal(usages)
	return string(msg)
}

func (o *operationVolumeProtection) sendEvent(ctx context.Context, reason, msg string) error {
	if o.SendEvent {
		return sendEvent(ctx, o.Logger, o.createEvent(reason, msg))
	}
	return nil
}

func (o *operationVolumeProtection) createEvent(reason, msg string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%s", os.Getenv(constant.KBEnvPodName), rand.String(16)),
			Namespace: os.Getenv(constant.KBEnvNamespace),
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: os.Getenv(constant.KBEnvNamespace),
			Name:      os.Getenv(constant.KBEnvPodName),
			UID:       types.UID(os.Getenv(constant.KBEnvPodUID)),
			FieldPath: "spec.containers{sqlchannel}",
		},
		Reason:  reason,
		Message: msg,
		Source: corev1.EventSource{
			Component: "sqlchannel",
			Host:      os.Getenv(constant.KBEnvNodeName),
		},
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		Type:           "Normal",
	}
}

type httpsVolumeStatsRequester struct {
	logger logr.Logger
	cli    *http.Client
	req    *http.Request
}

var _ volumeStatsRequester = &httpsVolumeStatsRequester{}

func (r *httpsVolumeStatsRequester) init(ctx context.Context) error {
	var err error
	if r.cli, err = httpClient(); err != nil {
		// r.logger.Error(err, "build HTTP client error at setup")
		return err
	}
	// if r.req, err = httpRequest(ctx); err != nil {
	// 	r.logger.Error(err, "build HTTP request error at setup, will try it later")
	// }
	return nil
}

func (r *httpsVolumeStatsRequester) request(ctx context.Context) ([]byte, error) {
	if r.cli == nil {
		return nil, fmt.Errorf("HTTP client for kubelet is unavailable")
	}
	if r.req == nil {
		// try to build http request again
		var err error
		r.req, err = httpRequest(ctx)
		if err != nil {
			r.logger.Error(err, "build HTTP request to query kubelet error")
			return nil, err
		}
	}

	req := r.req.WithContext(ctx)
	rsp, err := r.cli.Do(req)
	if err != nil {
		r.logger.Error(err, "issue request to kubelet error")
		return nil, err
	}
	if rsp.StatusCode != 200 {
		r.logger.Error(nil, fmt.Sprintf("HTTP response from kubelet error: %s", rsp.Status))
		return nil, fmt.Errorf(rsp.Status)
	}

	defer rsp.Body.Close()
	return io.ReadAll(rsp.Body)
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
				RootCAs: certPool,
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
	return os.Getenv(constant.KBEnvHostIP), nil
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
	node, err := cliset.CoreV1().Nodes().Get(ctx, os.Getenv(constant.KBEnvNodeName), metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return strconv.Itoa(int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)), nil
}

func normalizeVolumeWatermark(watermark *int, defaultVal int) int {
	if watermark == nil || *watermark < 0 || *watermark > 100 {
		return defaultVal
	}
	return *watermark
}

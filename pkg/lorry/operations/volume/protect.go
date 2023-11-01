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

package volume

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
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
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

type Protection struct {
	operations.Base
	dbManager     engines.DBManager
	Requester     volumeStatsRequester
	Pod           string
	HighWatermark int
	Volumes       map[string]volumeExt
	Readonly      bool
	SendEvent     bool // to disable event for testing
	Logger        logr.Logger
}

func (p *Protection) Init(ctx context.Context) error {
	p.Logger = ctrl.Log.WithName("Volume-Protection")
	if p.Requester == nil {
		p.Requester = &httpsVolumeStatsRequester{
			logger: p.Logger,
		}
	}
	p.SendEvent = true

	dbManager, err := register.GetDBManager()
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	p.dbManager = dbManager

	if err := p.Requester.init(ctx); err != nil {
		return err
	}

	p.Pod = viper.GetString(constant.KBEnvPodName)
	if err := p.initVolumes(); err != nil {
		p.Logger.Error(err, "init volumes to monitor error")
	}
	p.Logger.Info("succeed to init volume protection", "pod", p.Pod, "spec", p.buildVolumesMsg())
	return nil
}

func (p *Protection) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	return nil
}

func (p *Protection) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	if p.disabled() {
		p.Logger.Info("the volume protection operation is disabled")
		return nil, nil
	}

	summary, err := p.Requester.request(ctx)
	if err != nil {
		p.Logger.Error(err, "request stats summary from kubelet error")
		return nil, err
	}

	if err = p.updateVolumeStats(summary); err != nil {
		return nil, err
	}

	volumeUsages, err := p.checkUsage(ctx)
	resp := &operations.OpsResponse{
		Data: map[string]any{},
	}
	if err == nil {
		resp.Data["protect"] = volumeUsages
	}
	return resp, err
}

func (p *Protection) initVolumes() error {
	spec := &appsv1alpha1.VolumeProtectionSpec{}
	raw := viper.GetString(constant.KBEnvVolumeProtectionSpec)
	if err := json.Unmarshal([]byte(raw), spec); err != nil {
		p.Logger.Error(err, "unmarshal volume protection spec error", "raw spec", raw)
		return err
	}

	p.HighWatermark = normalizeVolumeWatermark(&spec.HighWatermark, 0)

	if p.Volumes == nil {
		p.Volumes = make(map[string]volumeExt)
	}
	for _, v := range spec.Volumes {
		p.Volumes[v.Name] = volumeExt{
			Name:          v.Name,
			HighWatermark: normalizeVolumeWatermark(v.HighWatermark, p.HighWatermark),
			Stats: statsv1alpha1.VolumeStats{
				Name: v.Name,
			},
		}
	}
	return nil
}

func (p *Protection) disabled() bool {
	// TODO: check the role and skip secondary instances.
	if len(p.Pod) == 0 || len(p.Volumes) == 0 {
		return true
	}
	for _, v := range p.Volumes {
		// take (0, 100] as enabled
		if v.HighWatermark > 0 && v.HighWatermark <= 100 {
			return false
		}
	}
	return true
}

func (p *Protection) updateVolumeStats(payload []byte) error {
	summary := &statsv1alpha1.Summary{}
	if err := json.Unmarshal(payload, summary); err != nil {
		p.Logger.Error(err, "stats summary obtained from kubelet error")
		return err
	}
	for _, pod := range summary.Pods {
		if pod.PodRef.Name == p.Pod {
			for _, stats := range pod.VolumeStats {
				if _, ok := p.Volumes[stats.Name]; !ok {
					continue
				}
				v := p.Volumes[stats.Name]
				v.Stats = stats
				p.Volumes[stats.Name] = v
			}
			break
		}
	}
	return nil
}

func (p *Protection) checkUsage(ctx context.Context) (map[string]any, error) {
	lower := make([]string, 0)
	higher := make([]string, 0)
	for name, v := range p.Volumes {
		ret := p.checkVolumeWatermark(v)
		if ret == 0 {
			lower = append(lower, name)
		} else {
			higher = append(higher, name)
		}
	}

	volumeUsages := p.buildVolumesMsg()
	readonly := p.Readonly
	// the instance is running normally and there have volume(s) over the space usage threshold.
	if !readonly && len(higher) > 0 {
		if err := p.highWatermark(ctx, volumeUsages); err != nil {
			return volumeUsages, err
		}
	}
	// the instance is protected in RO mode, and all volumes' space usage are under the threshold.
	if readonly && len(lower) == len(p.Volumes) {
		if err := p.lowWatermark(ctx, volumeUsages); err != nil {
			return volumeUsages, err
		}
	}
	return volumeUsages, nil
}

// checkVolumeWatermark checks whether the volume's space usage is over the threshold.
//
//	returns 0 if the volume will not be taken in account or its space usage is under the threshold
//	returns non-zero if the volume space usage is over the threshold
func (p *Protection) checkVolumeWatermark(v volumeExt) int {
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

func (p *Protection) highWatermark(ctx context.Context, volumeUsages map[string]any) error {
	if p.Readonly { // double check
		return nil
	}
	if err := p.lockInstance(ctx); err != nil {
		p.Logger.Error(err, "set instance to read-only error", "volumes", volumeUsages)
		return err
	}

	p.Logger.Info("set instance to read-only OK", "msg", volumeUsages)
	p.Readonly = true

	if err := p.sendEvent(ctx, reasonLock, volumeUsages); err != nil {
		p.Logger.Error(err, "send volume protection (lock) event error", "volumes", volumeUsages)
		return err
	}
	return nil
}

func (p *Protection) lowWatermark(ctx context.Context, volumeUsages map[string]any) error {
	if !p.Readonly { // double check
		return nil
	}
	if err := p.unlockInstance(ctx); err != nil {
		p.Logger.Error(err, "reset instance to read-write error", "volumes", volumeUsages)
		return err
	}

	p.Logger.Info("reset instance to read-write OK", "msg", volumeUsages)
	p.Readonly = false

	if err := p.sendEvent(ctx, reasonUnlock, volumeUsages); err != nil {
		p.Logger.Error(err, "send volume protection (unlock) event error", "volumes", volumeUsages)
		return err
	}
	return nil
}

func (p *Protection) lockInstance(ctx context.Context) error {
	return p.dbManager.Lock(ctx, "disk full")
}

func (p *Protection) unlockInstance(ctx context.Context) error {
	return p.dbManager.Unlock(ctx)
}

func (p *Protection) buildVolumesMsg() map[string]any {
	volumes := make([]map[string]string, 0)
	for _, v := range p.Volumes {
		usage := make(map[string]string)
		if v.HighWatermark != p.HighWatermark {
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
		"highWatermark": fmt.Sprintf("%d", p.HighWatermark),
		"volumes":       volumes,
	}
	return usages
}

func (p *Protection) sendEvent(ctx context.Context, reason string, volumeUsages map[string]any) error {
	if p.SendEvent {
		event, err := util.CreateEvent(reason, volumeUsages)
		if err != nil {
			return errors.Wrap(err, "create volume protection event failed")
		}
		return util.SendEvent(ctx, event)
	}
	return nil
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
		r.logger.Error(err, "build HTTP client error at setup")
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
	return viper.GetString(constant.KBEnvHostIP), nil
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
	node, err := cliset.CoreV1().Nodes().Get(ctx, viper.GetString(constant.KBEnvNodeName), metav1.GetOptions{})
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

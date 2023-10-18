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
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-logr/zapr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/rand"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type mockVolumeStatsRequester struct {
	summary []byte
}

var _ volumeStatsRequester = &mockVolumeStatsRequester{}

func (r *mockVolumeStatsRequester) init(ctx context.Context) error {
	return nil
}

func (r *mockVolumeStatsRequester) request(ctx context.Context) ([]byte, error) {
	return r.summary, nil
}

type mockErrorVolumeStatsRequester struct {
	initErr    bool
	requestErr bool
}

var _ volumeStatsRequester = &mockErrorVolumeStatsRequester{}

func (r *mockErrorVolumeStatsRequester) init(ctx context.Context) error {
	if r.initErr {
		return fmt.Errorf("error")
	}
	return nil
}

func (r *mockErrorVolumeStatsRequester) request(ctx context.Context) ([]byte, error) {
	if r.requestErr {
		return nil, fmt.Errorf("error")
	}
	return nil, nil
}

type DBManagerWithLock struct {
	component.FakeManager
	instanceLocked bool
	lockTimes      int
	unlockTimes    int
}

func (mgr *DBManagerWithLock) Lock(ctx context.Context, reason string) error {
	fmt.Fprintf(GinkgoWriter, "set readonly")
	mgr.instanceLocked = true
	mgr.lockTimes += 1
	return nil
}

func (mgr *DBManagerWithLock) Unlock(ctx context.Context) error {
	mgr.instanceLocked = false
	mgr.unlockTimes += 1
	return nil
}

var _ = Describe("Volume Protection Operation", func() {
	var (
		podName                      = rand.String(8)
		DBWithLockCharacterType      = "DBWithLock"
		DBWithoutLockCharacterType   = "DBWithoutLock"
		workloadTypeForTest          = "workloadTypeForTest"
		volumeName                   = rand.String(8)
		defaultThreshold             = 90
		zeroThreshold                = 0
		fullThreshold                = 100
		invalidThresholdLower        = -1
		invalidThresholdHigher       = 101
		capacityBytes                = uint64(10 * 1024 * 1024 * 1024)
		usedBytesUnderThreshold      = capacityBytes * uint64(defaultThreshold-3) / 100
		usedBytesOverThreshold       = capacityBytes * uint64(defaultThreshold+3) / 100
		usedBytesOverThresholdHigher = capacityBytes * uint64(defaultThreshold+4) / 100
		volumeProtectionSpec         = &appsv1alpha1.VolumeProtectionSpec{
			HighWatermark: defaultThreshold,
			Volumes: []appsv1alpha1.ProtectedVolume{
				{
					Name:          volumeName,
					HighWatermark: &defaultThreshold,
				},
			},
		}
		metadata = map[string]string{"sql": ""}
		req      = &ProbeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: util.VolumeProtection,
		}
		d, _ = zap.NewDevelopment()
		log  = zapr.NewLogger(d)
		p    = BaseOperations{Logger: log}

		dbManagerWithLock    = &DBManagerWithLock{}
		dbManagerWithoutLock = &component.FakeManager{}
	)

	component.RegisterManager(DBWithLockCharacterType, workloadTypeForTest, dbManagerWithLock)
	component.RegisterManager(DBWithoutLockCharacterType, workloadTypeForTest, dbManagerWithoutLock)

	p.Init(component.Properties{})

	setup := func() {
		os.Setenv(constant.KBEnvPodName, podName)
		os.Setenv(constant.KBEnvCharacterType, DBWithLockCharacterType)
		os.Setenv(constant.KBEnvWorkloadType, workloadTypeForTest)
		raw, _ := json.Marshal(volumeProtectionSpec)
		os.Setenv(constant.KBEnvVolumeProtectionSpec, string(raw))
		dbManagerWithLock.instanceLocked = false
		dbManagerWithLock.lockTimes = 0
		dbManagerWithLock.unlockTimes = 0
		development, _ := zap.NewDevelopment()
		optVolProt.Logger = zapr.NewLogger(development)
		optVolProt.Requester = &mockVolumeStatsRequester{}
		optVolProt.SendEvent = false
		optVolProt.Readonly = false
		optVolProt.Pod = os.Getenv(constant.KBEnvPodName)
		Expect(optVolProt.initVolumes()).ShouldNot(HaveOccurred())
	}

	cleanAll := func() {
		os.Unsetenv(constant.KBEnvPodName)
		os.Unsetenv(constant.KBEnvCharacterType)
		os.Unsetenv(constant.KBEnvVolumeProtectionSpec)
		dbManagerWithLock.instanceLocked = false
		dbManagerWithLock.lockTimes = 0
		dbManagerWithLock.unlockTimes = 0
	}

	BeforeEach(setup)

	AfterEach(cleanAll)

	resetVolumeProtectionSpecEnv := func(spec appsv1alpha1.VolumeProtectionSpec) {
		raw, _ := json.Marshal(spec)
		os.Setenv(constant.KBEnvVolumeProtectionSpec, string(raw))
	}

	newVolumeProtectionObj := func() *operationVolumeProtection {
		development, _ := zap.NewDevelopment()
		return &operationVolumeProtection{
			Logger:    zapr.NewLogger(development),
			Requester: &mockVolumeStatsRequester{},
			SendEvent: false,
		}
	}

	Context("Volume Protection", func() {
		It("init - succeed", func() {
			obj := optVolProt
			Expect(obj.Pod).Should(Equal(podName))
			Expect(obj.HighWatermark).Should(Equal(volumeProtectionSpec.HighWatermark))
			Expect(len(obj.Volumes)).Should(Equal(len(volumeProtectionSpec.Volumes)))
		})

		It("init - invalid volume protection spec env", func() {
			os.Setenv(constant.KBEnvVolumeProtectionSpec, "")
			obj := newVolumeProtectionObj()
			Expect(obj.initVolumes()).Should(HaveOccurred())
		})

		It("init - init requester error", func() {
			obj := optVolProt
			obj.Requester = &mockErrorVolumeStatsRequester{initErr: true}
			metadata := map[string]string{"sql": ""}
			req := &ProbeRequest{
				Data:      nil,
				Metadata:  metadata,
				Operation: util.VolumeProtection,
			}

			_, err := p.Invoke(context.Background(), req)
			Expect(err).Should(HaveOccurred())
		})

		It("init - normalize watermark", func() {
			By("normalize global watermark")
			for _, val := range []int{-1, 0, 101} {
				resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
					HighWatermark: val,
				})
				obj := newVolumeProtectionObj()
				Expect(obj.initVolumes()).Should(Succeed())
				Expect(obj.HighWatermark).Should(Equal(0))
			}

			By("normalize volume watermark")
			spec := appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: defaultThreshold,
				Volumes: []appsv1alpha1.ProtectedVolume{
					{
						Name:          "01",
						HighWatermark: &invalidThresholdLower,
					},
					{
						Name:          "02",
						HighWatermark: &invalidThresholdHigher,
					},
					{
						Name:          "03",
						HighWatermark: &zeroThreshold,
					},
					{
						Name:          "04",
						HighWatermark: &fullThreshold,
					},
				},
			}
			resetVolumeProtectionSpecEnv(spec)
			obj := newVolumeProtectionObj()
			Expect(obj.initVolumes()).Should(Succeed())
			Expect(obj.HighWatermark).Should(Equal(spec.HighWatermark))
			for _, v := range spec.Volumes {
				if *v.HighWatermark >= 0 && *v.HighWatermark <= 100 {
					Expect(obj.Volumes[v.Name].HighWatermark).Should(Equal(*v.HighWatermark))
				} else {
					Expect(obj.Volumes[v.Name].HighWatermark).Should(Equal(obj.HighWatermark))
				}
			}
		})

		It("disabled - empty pod name", func() {
			os.Setenv(constant.KBEnvPodName, "")
			obj := optVolProt
			obj.Pod = os.Getenv(constant.KBEnvPodName)
			Expect(obj.disabled()).Should(BeTrue())
			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("disabled - empty volume", func() {
			resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: defaultThreshold,
				Volumes:       []appsv1alpha1.ProtectedVolume{},
			})
			obj := optVolProt
			obj.Volumes = nil
			Expect(obj.initVolumes()).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("disabled - volumes with zero watermark", func() {
			resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: defaultThreshold,
				Volumes: []appsv1alpha1.ProtectedVolume{
					{
						Name:          "data",
						HighWatermark: &zeroThreshold,
					},
				},
			})
			obj := optVolProt
			obj.Volumes = nil
			Expect(obj.initVolumes()).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())

		})

		It("query stats summary - request error", func() {
			obj := optVolProt
			obj.Requester = &mockErrorVolumeStatsRequester{requestErr: true}
			_, err := p.Invoke(context.Background(), req)
			Expect(err).Should(HaveOccurred())
		})

		It("query stats summary - format error", func() {
			// default summary is empty string
			_, err := p.Invoke(context.Background(), req)
			Expect(err).Should(HaveOccurred())
		})

		It("query stats summary - ok", func() {
			obj := optVolProt
			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{}
			mock.summary, _ = json.Marshal(stats)

			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("update volume stats summary", func() {
			obj := optVolProt
			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{
				Pods: []statsv1alpha1.PodStats{
					{
						PodRef: statsv1alpha1.PodReference{
							Name: podName,
						},
						VolumeStats: []statsv1alpha1.VolumeStats{
							{
								Name: volumeName,
								FsStats: statsv1alpha1.FsStats{
									CapacityBytes: &capacityBytes,
									UsedBytes:     &usedBytesUnderThreshold,
								},
							},
						},
					},
				},
			}
			mock.summary, _ = json.Marshal(stats)

			// nil capacity and used bytes
			stats.Pods[0].VolumeStats[0].CapacityBytes = nil
			stats.Pods[0].VolumeStats[0].UsedBytes = nil

			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())

			stats.Pods[0].VolumeStats[0].CapacityBytes = &capacityBytes
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			_, err = p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Volumes[volumeName].Stats).Should(Equal(stats.Pods[0].VolumeStats[0]))
			Expect(obj.Readonly).Should(BeFalse())
		})

		It("volume over high watermark", func() {
			obj := optVolProt
			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{
				Pods: []statsv1alpha1.PodStats{
					{
						PodRef: statsv1alpha1.PodReference{
							Name: podName,
						},
						VolumeStats: []statsv1alpha1.VolumeStats{
							{
								Name: volumeName,
								FsStats: statsv1alpha1.FsStats{
									CapacityBytes: &capacityBytes,
									UsedBytes:     &usedBytesOverThreshold,
								},
							},
						},
					},
				},
			}
			mock.summary, _ = json.Marshal(stats)

			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue())
			Expect(dbManagerWithLock.instanceLocked).Should(BeTrue())
			Expect(dbManagerWithLock.lockTimes).Should(Equal(1))

			// check again and the usage is higher, no lock action triggered again
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesOverThresholdHigher
			mock.summary, _ = json.Marshal(stats)
			_, err = p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue())
			Expect(dbManagerWithLock.instanceLocked).Should(BeTrue())
			Expect(dbManagerWithLock.lockTimes).Should(Equal(1))
		})

		It("volume under high watermark", func() {
			obj := optVolProt
			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{
				Pods: []statsv1alpha1.PodStats{
					{
						PodRef: statsv1alpha1.PodReference{
							Name: podName,
						},
						VolumeStats: []statsv1alpha1.VolumeStats{
							{
								Name: volumeName,
								FsStats: statsv1alpha1.FsStats{
									CapacityBytes: &capacityBytes,
									UsedBytes:     &usedBytesOverThreshold,
								},
							},
						},
					},
				},
			}
			mock.summary, _ = json.Marshal(stats)
			_, err := p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue())
			Expect(dbManagerWithLock.instanceLocked).Should(BeTrue())
			Expect(dbManagerWithLock.lockTimes).Should(Equal(1))

			// drops down the usage, and trigger unlock action
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			mock.summary, _ = json.Marshal(stats)
			_, err = p.Invoke(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeFalse())
			Expect(dbManagerWithLock.instanceLocked).Should(BeFalse())
			Expect(dbManagerWithLock.unlockTimes).Should(Equal(1))
		})

		It("lock/unlock error", func() {
			os.Setenv(constant.KBEnvCharacterType, DBWithoutLockCharacterType)
			obj := optVolProt
			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{
				Pods: []statsv1alpha1.PodStats{
					{
						PodRef: statsv1alpha1.PodReference{
							Name: podName,
						},
						VolumeStats: []statsv1alpha1.VolumeStats{
							{
								Name: volumeName,
								FsStats: statsv1alpha1.FsStats{
									CapacityBytes: &capacityBytes,
									UsedBytes:     &usedBytesOverThreshold,
								},
							},
						},
					},
				},
			}
			mock.summary, _ = json.Marshal(stats)
			_, err := p.Invoke(context.Background(), req)
			Expect(err).Should(HaveOccurred())
			Expect(obj.Readonly).Should(BeFalse()) // unchanged

			// drops down the usage, and trigger unlock action
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			mock.summary, _ = json.Marshal(stats)
			obj.Readonly = true // hack it as locked
			_, err = p.Invoke(context.Background(), req)
			Expect(err).Should(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue()) // unchanged
		})
	})
})

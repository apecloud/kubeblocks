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
	"encoding/json"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/rand"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
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

var _ = Describe("Volume Protection Operation", func() {
	var (
		podName                      = rand.String(8)
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
	)

	setup := func() {
		raw, _ := json.Marshal(volumeProtectionSpec)
		viper.SetDefault(constant.KBEnvVolumeProtectionSpec, string(raw))
		viper.SetDefault(constant.KBEnvPodName, podName)
	}

	cleanAll := func() {
	}

	BeforeEach(setup)

	AfterEach(cleanAll)

	resetVolumeProtectionSpecEnv := func(spec appsv1alpha1.VolumeProtectionSpec) {
		raw, _ := json.Marshal(spec)
		viper.SetDefault(constant.KBEnvVolumeProtectionSpec, string(raw))
	}

	newProtection := func() *Protection {
		protection := &Protection{
			Requester: &mockVolumeStatsRequester{},
		}
		Expect(protection.Init(context.Background())).Should(Succeed())
		protection.SendEvent = false
		return protection
	}

	Context("Volume Protection", func() {
		It("init - succeed", func() {
			protection := &Protection{
				Requester: &mockVolumeStatsRequester{},
			}
			Expect(protection.Init(context.Background())).Should(Succeed())
			Expect(protection.Pod).Should(Equal(podName))
			Expect(protection.HighWatermark).Should(Equal(volumeProtectionSpec.HighWatermark))
			Expect(len(protection.Volumes)).Should(Equal(len(volumeProtectionSpec.Volumes)))
		})

		It("init - invalid volume protection spec env", func() {
			viper.SetDefault(constant.KBEnvVolumeProtectionSpec, "")
			protection := &Protection{
				Requester: &mockVolumeStatsRequester{},
			}
			Expect(protection.initVolumes()).Should(HaveOccurred())
		})

		It("init - init requester error", func() {
			protection := &Protection{
				Requester: &mockErrorVolumeStatsRequester{initErr: true},
			}

			Expect(protection.Init(context.Background())).Should(HaveOccurred())
		})

		It("init - normalize watermark", func() {
			By("normalize global watermark")
			for _, val := range []int{-1, 0, 101} {
				resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
					HighWatermark: val,
				})
				obj := newProtection()
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
			obj := newProtection()
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
			viper.SetDefault(constant.KBEnvPodName, "")
			obj := newProtection()
			obj.Pod = viper.GetString(constant.KBEnvPodName)
			Expect(obj.disabled()).Should(BeTrue())
			_, err := obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("disabled - empty volume", func() {
			resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: defaultThreshold,
				Volumes:       []appsv1alpha1.ProtectedVolume{},
			})
			obj := newProtection()
			obj.Volumes = nil
			Expect(obj.initVolumes()).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			_, err := obj.Do(context.Background(), nil)
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
			obj := newProtection()
			obj.Volumes = nil
			Expect(obj.initVolumes()).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			_, err := obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())

		})

		It("query stats summary - request error", func() {
			obj := newProtection()
			obj.Requester = &mockErrorVolumeStatsRequester{requestErr: true}
			_, err := obj.Do(context.Background(), nil)
			Expect(err).Should(HaveOccurred())
		})

		It("query stats summary - format error", func() {
			// default summary is empty string
			obj := newProtection()
			_, err := obj.Do(context.Background(), nil)
			Expect(err).Should(HaveOccurred())
		})

		It("query stats summary - ok", func() {
			obj := newProtection()
			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{}
			mock.summary, _ = json.Marshal(stats)
			Expect(obj.Init(context.Background())).Should(Succeed())

			_, err := obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("update volume stats summary", func() {
			obj := newProtection()
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

			_, err := obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())

			stats.Pods[0].VolumeStats[0].CapacityBytes = &capacityBytes
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			_, err = obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Volumes[volumeName].Stats).Should(Equal(stats.Pods[0].VolumeStats[0]))
			Expect(obj.Readonly).Should(BeFalse())
		})

		It("volume over high watermark", func() {
			ctrl := gomock.NewController(GinkgoT())
			mockDBManager := engines.NewMockDBManager(ctrl)
			mockDBManager.EXPECT().Lock(gomock.Any(), gomock.Any()).Return(nil)
			register.SetDBManager(mockDBManager)

			obj := newProtection()
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

			_, err := obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue())

			// check again and the usage is higher, no lock action triggered again
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesOverThresholdHigher
			mock.summary, _ = json.Marshal(stats)
			_, err = obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue())
		})

		It("volume under high watermark", func() {
			ctrl := gomock.NewController(GinkgoT())
			mockDBManager := engines.NewMockDBManager(ctrl)
			mockDBManager.EXPECT().Lock(gomock.Any(), gomock.Any()).Return(nil)
			mockDBManager.EXPECT().Unlock(gomock.Any()).Return(nil)
			register.SetDBManager(mockDBManager)

			obj := newProtection()
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
			_, err := obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue())

			// drops down the usage, and trigger unlock action
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			mock.summary, _ = json.Marshal(stats)
			_, err = obj.Do(context.Background(), nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(obj.Readonly).Should(BeFalse())
		})

		It("lock/unlock error", func() {
			ctrl := gomock.NewController(GinkgoT())
			mockDBManager := engines.NewMockDBManager(ctrl)
			mockDBManager.EXPECT().Lock(gomock.Any(), gomock.Any()).Return(fmt.Errorf("test"))
			mockDBManager.EXPECT().Unlock(gomock.Any()).Return(fmt.Errorf("test"))
			register.SetDBManager(mockDBManager)

			obj := newProtection()
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
			_, err := obj.Do(context.Background(), nil)
			Expect(err).Should(HaveOccurred())
			Expect(obj.Readonly).Should(BeFalse()) // unchanged

			// drops down the usage, and trigger unlock action
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			mock.summary, _ = json.Marshal(stats)
			obj.Readonly = true // hack it as locked
			_, err = obj.Do(context.Background(), nil)
			Expect(err).Should(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue()) // unchanged
		})
	})
})

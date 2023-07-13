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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"k8s.io/apimachinery/pkg/util/rand"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
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
		instanceLocked = false
		lockTimes      = 0
		unlockTimes    = 0
	)
	setup := func() {
		os.Setenv(constant.KBEnvPodName, podName)
		raw, _ := json.Marshal(volumeProtectionSpec)
		os.Setenv(constant.KBEnvVolumeProtectionSpec, string(raw))
		instanceLocked = false
		lockTimes = 0
		unlockTimes = 0
	}

	cleanAll := func() {
		os.Unsetenv(constant.KBEnvPodName)
		os.Unsetenv(constant.KBEnvVolumeProtectionSpec)
		instanceLocked = false
		lockTimes = 0
		unlockTimes = 0
	}

	BeforeEach(setup)

	AfterEach(cleanAll)

	lockInstance := func(ctx context.Context) error {
		instanceLocked = true
		lockTimes += 1
		return nil
	}

	unlockInstance := func(ctx context.Context) error {
		instanceLocked = false
		unlockTimes += 1
		return nil
	}

	lockInstanceErr := func(ctx context.Context) error {
		return fmt.Errorf("error")
	}

	unlockInstanceErr := func(ctx context.Context) error {
		return fmt.Errorf("error")
	}

	resetVolumeProtectionSpecEnv := func(spec appsv1alpha1.VolumeProtectionSpec) {
		raw, _ := json.Marshal(spec)
		os.Setenv(constant.KBEnvVolumeProtectionSpec, string(raw))
	}

	newVolumeProtectionObj := func() *operationVolumeProtection {
		return &operationVolumeProtection{
			Logger:    logger.NewLogger("volume-protection-test"),
			Requester: &mockVolumeStatsRequester{},
			SendEvent: false,
			BaseOperation: &BaseOperations{
				LockInstance:   lockInstance,
				UnlockInstance: unlockInstance,
			},
		}
	}

	Context("Volume Protection", func() {
		It("init - succeed", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
			Expect(obj.Pod).Should(Equal(podName))
			Expect(obj.HighWatermark).Should(Equal(volumeProtectionSpec.HighWatermark))
			Expect(len(obj.Volumes)).Should(Equal(len(volumeProtectionSpec.Volumes)))
		})

		It("init - invalid volume protection spec env", func() {
			os.Setenv(constant.KBEnvVolumeProtectionSpec, "")
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(HaveOccurred())
		})

		It("init - init requester error", func() {
			obj := newVolumeProtectionObj()
			obj.Requester = &mockErrorVolumeStatsRequester{initErr: true}
			Expect(obj.Init(bindings.Metadata{})).Should(HaveOccurred())
		})

		It("init - normalize watermark", func() {
			By("normalize global watermark")
			for _, val := range []int{-1, 0, 101} {
				resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
					HighWatermark: val,
				})
				obj := newVolumeProtectionObj()
				Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
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
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
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
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			Expect(obj.Invoke(ctx, nil, nil)).Should(BeNil())
		})

		It("disabled - empty volume", func() {
			resetVolumeProtectionSpecEnv(appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: defaultThreshold,
				Volumes:       []appsv1alpha1.ProtectedVolume{},
			})
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			Expect(obj.Invoke(ctx, nil, nil)).Should(BeNil())
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
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
			Expect(obj.disabled()).Should(BeTrue())
			Expect(obj.Invoke(ctx, nil, nil)).Should(BeNil())

		})

		It("query stats summary - request error", func() {
			obj := newVolumeProtectionObj()
			obj.Requester = &mockErrorVolumeStatsRequester{requestErr: true}
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
			Expect(obj.Invoke(ctx, nil, nil)).Should(HaveOccurred())
		})

		It("query stats summary - format error", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())
			// default summary is empty string
			Expect(obj.Invoke(ctx, nil, nil)).Should(HaveOccurred())
		})

		It("query stats summary - ok", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())

			mock := obj.Requester.(*mockVolumeStatsRequester)
			stats := statsv1alpha1.Summary{}
			mock.summary, _ = json.Marshal(stats)

			rsp := &bindings.InvokeResponse{}
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())
		})

		It("update volume stats summary", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())

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
			rsp := &bindings.InvokeResponse{}
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())

			stats.Pods[0].VolumeStats[0].CapacityBytes = &capacityBytes
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())
			Expect(obj.Volumes[volumeName].Stats).Should(Equal(stats.Pods[0].VolumeStats[0]))
			Expect(obj.Readonly).Should(BeFalse())
		})

		It("volume over high watermark", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())

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

			rsp := &bindings.InvokeResponse{}
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())
			Expect(obj.Readonly).Should(BeTrue())
			Expect(instanceLocked).Should(BeTrue())
			Expect(lockTimes).Should(Equal(1))

			// check again and the usage is higher, no lock action triggered again
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesOverThresholdHigher
			mock.summary, _ = json.Marshal(stats)
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())
			Expect(obj.Readonly).Should(BeTrue())
			Expect(instanceLocked).Should(BeTrue())
			Expect(lockTimes).Should(Equal(1))
		})

		It("volume under high watermark", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())

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

			rsp := &bindings.InvokeResponse{}
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())
			Expect(obj.Readonly).Should(BeTrue())
			Expect(instanceLocked).Should(BeTrue())
			Expect(lockTimes).Should(Equal(1))

			// drops down the usage, and trigger unlock action
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			mock.summary, _ = json.Marshal(stats)
			Expect(obj.Invoke(ctx, nil, rsp)).Should(Succeed())
			Expect(obj.Readonly).Should(BeFalse())
			Expect(instanceLocked).Should(BeFalse())
			Expect(unlockTimes).Should(Equal(1))
		})

		It("lock/unlock error", func() {
			obj := newVolumeProtectionObj()
			Expect(obj.Init(bindings.Metadata{})).Should(Succeed())

			obj.BaseOperation.LockInstance = lockInstanceErr
			obj.BaseOperation.UnlockInstance = unlockInstanceErr

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

			rsp := &bindings.InvokeResponse{}
			Expect(obj.Invoke(ctx, nil, rsp)).Should(HaveOccurred())
			Expect(obj.Readonly).Should(BeFalse()) // unchanged

			// drops down the usage, and trigger unlock action
			stats.Pods[0].VolumeStats[0].UsedBytes = &usedBytesUnderThreshold
			mock.summary, _ = json.Marshal(stats)
			obj.Readonly = true // hack it as locked
			Expect(obj.Invoke(ctx, nil, rsp)).Should(HaveOccurred())
			Expect(obj.Readonly).Should(BeTrue()) // unchanged
		})
	})
})

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

package utils

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

var _ = Describe("Periodical Enqueue Source", func() {
	const (
		backupName = "test-backup"
	)

	var (
		ctx        context.Context
		cancelFunc context.CancelFunc
		cli        client.Client
		queue      workqueue.RateLimitingInterface
	)

	createBackup := func(name string) {
		Expect(cli.Create(ctx, &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		})).Should(Succeed())
	}

	BeforeEach(func() {
		Expect(dpv1alpha1.AddToScheme(scheme.Scheme)).Should(Succeed())
		ctx, cancelFunc = context.WithCancel(context.TODO())
		cli = (&fake.ClientBuilder{}).Build()
		queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	})

	Context("source", func() {
		var source *PeriodicalEnqueueSource

		BeforeEach(func() {
			By("create source")
			source = NewPeriodicalEnqueueSource(cli, &dpv1alpha1.BackupList{}, 1*time.Second, PeriodicalEnqueueSourceOption{})
		})

		It("should start success", func() {
			By("start source")
			Expect(source.Start(ctx, nil, queue)).Should(Succeed())

			By("wait and there is no resources")
			time.Sleep(1 * time.Second)
			Expect(queue.Len()).Should(Equal(0))

			By("create a resource")
			createBackup(backupName)

			By("wait and there is one resource")
			time.Sleep(2 * time.Second)
			Expect(queue.Len()).Should(Equal(1))

			By("cancel context, the queue source shouldn't run anymore")
			item, _ := queue.Get()
			queue.Forget(item)
			Expect(queue.Len()).Should(Equal(0))
			cancelFunc()
			time.Sleep(2 * time.Second)
			Expect(queue.Len()).Should(Equal(0))
		})

		It("predicate should work", func() {
			By("start source")
			Expect(source.Start(ctx, nil, queue, predicate.Funcs{
				GenericFunc: func(event event.GenericEvent) bool {
					return event.Object.GetName() == backupName
				},
			}))

			By("create a resource match predicate")
			createBackup(backupName)

			By("create another resource that does not match predicate")
			createBackup(backupName + "-1")

			By("wait and there is one resource")
			time.Sleep(2 * time.Second)
			Expect(queue.Len()).Should(Equal(1))

			cancelFunc()
		})

		It("order function should work", func() {
			By("set source order func")
			source.option.OrderFunc = func(objList client.ObjectList) client.ObjectList {
				backupList := &dpv1alpha1.BackupList{}
				objArray := make([]runtime.Object, 0)
				backups, _ := meta.ExtractList(objList)
				objArray = append(objArray, backups[1], backups[0])
				_ = meta.SetList(backupList, objArray)
				return backupList
			}

			By("create a resource")
			createBackup(backupName + "-1")

			By("create another resource")
			createBackup(backupName + "-2")

			By("start source")
			Expect(source.Start(ctx, nil, queue)).Should(Succeed())

			time.Sleep(2 * time.Second)
			Expect(queue.Len()).Should(Equal(2))
			first, _ := queue.Get()
			Expect(first.(reconcile.Request).Name).Should(Equal(backupName + "-2"))

			cancelFunc()
		})
	})
})

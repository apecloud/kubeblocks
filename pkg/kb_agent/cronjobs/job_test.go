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

package cronjobs

import (
	"fmt"
	"testing"
	"time"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/stretchr/testify/assert"
)

func TestNewJob(t *testing.T) {
	t.Run("NewJob", func(t *testing.T) {
		cronJob := &util.CronJob{
			PeriodSeconds:    1,
			SuccessThreshold: 2,
			FailureThreshold: 2,
			ReportFrequency:  2,
		}
		name := constant.RoleProbeAction
		job, err := NewJob(name, cronJob)
		assert.NotNil(t, job)
		assert.Nil(t, err)
	})

	t.Run("NewJob", func(t *testing.T) {
		cronJob := &util.CronJob{
			PeriodSeconds:    0,
			SuccessThreshold: 0,
			FailureThreshold: 0,
			ReportFrequency:  0,
		}
		name := "test"
		job, err := NewJob(name, cronJob)
		assert.Nil(t, job)
		assert.NotNil(t, err)
		assert.Error(t, err, fmt.Errorf("%s not implemented", name))
	})

}

func TestCommonJob_Start(t *testing.T) {
	t.Run("No errors after startup", func(t *testing.T) {
		job := &CommonJob{
			Name:            "test",
			PeriodSeconds:   1,
			ReportFrequency: 1,
			FailedCount:     0,
			Do: func() error {
				return nil
			},
		}
		go func() {
			job.Start()
		}()
		time.Sleep(3 * time.Second)
		job.Ticker.Stop()
		assert.Equal(t, 0, job.FailedCount)
	})

	t.Run("Error after startup", func(t *testing.T) {
		job := &CommonJob{
			Name:            "test",
			PeriodSeconds:   2,
			ReportFrequency: 1,
			FailedCount:     0,
			Do: func() error {
				return fmt.Errorf("test error")
			},
		}
		go func() {
			job.Start()
		}()
		time.Sleep(3 * time.Second)
		job.Stop()
		assert.Equal(t, 1, job.FailedCount)
	})
}

package cronjobs

import (
	"fmt"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

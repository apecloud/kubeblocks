package cronjobs

import (
	"encoding/json"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewManager(t *testing.T) {
	handlers.ResetHandlerSpecs()
	actionHandlerSpecs := map[string]util.HandlerSpec{
		constant.RoleProbeAction: {
			CronJob: &util.CronJob{
				PeriodSeconds:    1,
				SuccessThreshold: 2,
				FailureThreshold: 2,
				ReportFrequency:  2,
			},
		},
		"test": {},
	}
	actionJson, _ := json.Marshal(actionHandlerSpecs)
	viper.Set(constant.KBEnvActionHandlers, string(actionJson))
	assert.Nil(t, handlers.InitHandlers())
	t.Run("NewManager", func(t *testing.T) {
		manager, err := NewManager()
		assert.NotNil(t, manager)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(manager.Jobs))
	})
}

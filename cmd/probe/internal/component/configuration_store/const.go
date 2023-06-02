package configuration_store

const (
	SysID              = "sys_id"
	TTL                = "ttl"
	MaxLagOnSwitchover = "max_lag_on_switchover"
	AcquireTime        = "acquire_time"
	RenewTime          = "renew_time"
	OpTime             = "op_time"
	ScheduledAt        = "scheduled_at"
	Extra              = "extra"
)

const (
	ConfigSuffix     = "-config"
	SwitchoverSuffix = "-switchover"
	LeaderSuffix     = "-leader"
	ExtraSuffix      = "-extra"
)

const (
	KbRoleLabel = "kubeblocks.io/role"
)

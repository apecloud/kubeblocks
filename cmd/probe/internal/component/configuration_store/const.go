package configuration_store

const (
	SysID              = "sys_id"
	State              = "state"
	ProbeUrl           = "probe_url"
	TTL                = "ttl"
	MaxLagOnSwitchover = "max_lag_on_switchover"
	ReplicationMode    = "replication_mode"
	AcquireTime        = "acquire_time"
	RenewTime          = "renew_time"
	ScheduledAt        = "scheduled_at"
	SyncStandby        = "sync_standby"
	Extra              = "extra"
)

const (
	ConfigSuffix     = "-config"
	SwitchoverSuffix = "-Switchover"
	ExtraSuffix      = "-extra"
)

const (
	KbRoleLabel = "kubeblocks.io/role"
)

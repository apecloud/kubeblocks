package configuration_store

const (
	SysID            = "sys_id"
	State            = "state"
	ProbeUrl         = "probe_url"
	TTL              = "ttl"
	MaxLagOnFailover = "max_lag_on_failover"
	ReplicationMode  = "replication_mode"
	AcquireTime      = "acquire_time"
	LeaderName       = "leader_name"
	RenewTime        = "renew_time"
	Candidate        = "candidate"
	ScheduledAt      = "scheduled_at"
	SyncStandby      = "sync_standby"
	Extra            = "extra"
)

const (
	ConfigSuffix   = "-config"
	FailoverSuffix = "-failover"
	ExtraSuffix    = "-extra"
)

const (
	KbRoleLabel = "kubeblocks.io/role"
)

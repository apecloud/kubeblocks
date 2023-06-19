package dcs

type DCS interface {
	Initialize()
	GetCluster() (*Cluster, error)
	GetMembers() ([]Member, error)
	ResetCluser()
	DeleteCluser()
	AttempAcquireLock()
	HasLock()
	ReleaseLock()
	GetHaConfig() (*HaConfig, error)
	GetSwitchover() (*Switchover, error)
	SetSwitchover()
	AddThisMember()
}

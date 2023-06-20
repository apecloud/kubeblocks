package dcs

type DCS interface {
	Initialize() error
	GetCluster() (*Cluster, error)
	GetMembers() ([]Member, error)
	ResetCluser()
	DeleteCluser()
	GetHaConfig() (*HaConfig, error)
	GetSwitchover() (*Switchover, error)
	SetSwitchover()
	AddThisMember()

	AttempAcquireLock() error
	CreateLock() error
	IsLockExist() (bool, error)
	HasLock() bool
	ReleaseLock() error
	UpdateLock() error

	DeleteCluser()
	GetLeader() (*Leader, error)
	ResetCluser()
}

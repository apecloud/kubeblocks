package dcs

type DCS interface {
	Initialize()
	GetCluster() (*Cluster, error)
	ResetCluser()
	DeleteCluser()
	AttempAcquireLock()
	HasLock()
	ReleaseLock()
	GetSwitchover()
	SetSwitchover()
	AddThisMember()
}

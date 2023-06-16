package dcs

type DCS interface {
	Initialize()
	GetCluser() Cluster
	ResetCluser()
	DeleteCluser()
	AttempAcquireLock()
	HasLock()
	ReleaseLock()
	GetSwitchover()
	SetSwitchover()
	AddThisMember()
}

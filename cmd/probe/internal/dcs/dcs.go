package dcs

type DCS interface {
	GetCluser() Cluster
}

type Cluster interface {
	HasLock()
	AttempAcquireLock()
	ReleaseLock()
}

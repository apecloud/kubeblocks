package component

type DBManager interface {
	Initialize()
	IsInitialized()
	IsRunning()
	IsHealthy()
	IsLeader() bool
	Recover()
	AddToCluster()
	Premote()
	Demote()
	GetHealthiestMember()
	HasOtherHealthtyLeader()
}

type dbManagerBase struct {
	MemberName string
}

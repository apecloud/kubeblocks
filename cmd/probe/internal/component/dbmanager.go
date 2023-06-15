package component

type DBManager interface {
	IsRunning()
	IsHealthy()
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

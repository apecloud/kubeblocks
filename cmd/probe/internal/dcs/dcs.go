package dcs

import "github.com/spf13/viper"

type DCS interface {
	Initialize() error

	// cluster manage functions
	GetClusterName() string
	GetCluster() (*Cluster, error)
	ResetCluser()
	DeleteCluser()

	// cluster scole ha config
	GetHaConfig() (*HaConfig, error)

	// member manager funtions
	GetMembers() ([]Member, error)
	AddCurrentMember() error

	// manual switchover
	GetSwitchover() (*Switchover, error)
	CreateSwitchover(string, string) error
	DeleteSwitchover() error

	// cluster scope leader lock
	AttempAcquireLock() error
	CreateLock() error
	IsLockExist() (bool, error)
	HasLock() bool
	ReleaseLock() error
	UpdateLock() error

	GetLeader() (*Leader, error)
}

var dcs DCS

func init() {
	viper.SetDefault("KB_TTL", 30)
}

func GetStore() DCS {
	return dcs
}

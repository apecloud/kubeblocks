package dcs

import "github.com/spf13/viper"

type DCS interface {
	Initialize() error

	GetClusterName() string
	GetCluster() (*Cluster, error)
	ResetCluser()
	DeleteCluser()

	GetMembers() ([]Member, error)
	AddCurrentMember() error

	GetHaConfig() (*HaConfig, error)

	GetSwitchover() (*Switchover, error)
	CreateSwitchover(string, string) error
	DeleteSwitchover() error

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

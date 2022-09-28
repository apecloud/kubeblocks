package iptables

import (
	"github.com/coreos/go-iptables/iptables"
)

type iptablesWrapper struct {
	*iptables.IPTables
}

func NewIPTables() (IPTables, error) {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	return iptablesWrapper{ipt}, err
}

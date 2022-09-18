package iptables

import (
	"github.com/coreos/go-iptables/iptables"
)

type iptablesWrapper struct {
	ipt *iptables.IPTables
}

func (i iptablesWrapper) Exists(table, chain string, ruleSpec ...string) (bool, error) {
	return i.ipt.Exists(table, chain, ruleSpec...)
}

func (i iptablesWrapper) Append(table, chain string, ruleSpec ...string) error {
	return i.ipt.Append(table, chain, ruleSpec...)
}

func (i iptablesWrapper) Delete(table, chain string, ruleSpec ...string) error {
	return i.ipt.Delete(table, chain, ruleSpec...)
}

func NewIPTables() (IPTables, error) {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	return iptablesWrapper{ipt: ipt}, err
}

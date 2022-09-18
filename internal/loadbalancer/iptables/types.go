package iptables

type IPTables interface {
	Exists(table, chain string, rulespec ...string) (bool, error)

	Append(table, chain string, rulespec ...string) error

	Delete(table, chain string, rulespec ...string) error
}

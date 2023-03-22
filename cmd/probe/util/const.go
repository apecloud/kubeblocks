package util

import "github.com/dapr/components-contrib/bindings"

const (
	CheckRunningOperation bindings.OperationKind = "checkRunning"
	CheckStatusOperation  bindings.OperationKind = "checkStatus"
	CheckRoleOperation    bindings.OperationKind = "checkRole"
	GetRoleOperation      bindings.OperationKind = "getRole"
	GetLagOperation       bindings.OperationKind = "getLag"
	ExecOperation         bindings.OperationKind = "exec"
	QueryOperation        bindings.OperationKind = "query"
	CloseOperation        bindings.OperationKind = "close"

	OperationNotImplemented = "NotImplemented"
	OperationInvalid        = "Invalid"
	OperationSuccess        = "Success"
	OperationFailed         = "Failed"
)

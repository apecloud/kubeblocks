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

	// actions for cluster accounts management
	ListUsersOp      bindings.OperationKind = "listUsers"
	CreateUserOp     bindings.OperationKind = "createUser"
	DeleteUserOp     bindings.OperationKind = "deleteUser"
	DescribeUserOp   bindings.OperationKind = "describeUser"
	GrantUserRoleOp  bindings.OperationKind = "grantUserRole"
	RevokeUserRoleOp bindings.OperationKind = "revokeUserRole"
	// actions for cluster roles management

	OperationNotImplemented = "NotImplemented"
	OperationInvalid        = "Invalid"
	OperationSuccess        = "Success"
	OperationFailed         = "Failed"
)

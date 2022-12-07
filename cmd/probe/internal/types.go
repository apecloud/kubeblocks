package internal

type ProbeMessage struct {
	Event        string `json:"event,omitempty"`
	OriginalRole string `json:"originalRole,omitempty"`
	Role         string `json:"role,omitempty"`
	Message      string `json:"message,omitempty"`
}

// AccessMode define SVC access mode enums.
// +enum
type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

const (
	// keys from response's metadata.
	RespOpKey           = "operation"
	RespSQLKey          = "sql"
	RespStartTimeKey    = "start-time"
	RespRowsAffectedKey = "rows-affected"
	RespEndTimeKey      = "end-time"
	RespDurationKey     = "duration"
	StatusCode          = "status-code"
	//451 Unavailable For Legal Reasons, used to indicate check failed and trigger kubelet events
	CheckFailedHTTPCode = "451"
)

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

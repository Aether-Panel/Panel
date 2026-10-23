package skypanel

type Transmission struct {
	Message interface{}      `json:"data"`
	Type    TransmissionType `json:"type"`
}

type TransmissionType string

const (
	MessageTypeLog    = "console"
	MessageTypeStats  = "stat"
	MessageTypeStatus = "status"
	// MessageTypeCrash is sent on the status channel when a server dies on its own
	MessageTypeCrash = "crash"
)

// ServerCrash is the payload of a crash message; the full report is fetched separately
type ServerCrash struct {
	ID        string `json:"id"`
	ExitCode  int    `json:"exitCode"`
	Restarted bool   `json:"restarted"`
}

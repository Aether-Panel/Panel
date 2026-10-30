package skypanel

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
)

type ServerIDResponse struct {
	ID string `json:"id"`
} // @name ServerId

type ServerStats struct {
	// CPU is in percent of one core (200 = two cores busy); MaxCPU is what the server may use,
	// in the same unit: its CPU limit, or every core of the node when it has none
	CPU        float64         `json:"cpu"`
	MaxCPU     float64         `json:"maxCpu"`
	Memory     float64         `json:"memory"`
	MaxMemory  float64         `json:"maxMemory"`
	Disk       float64         `json:"storage"`
	MaxStorage float64         `json:"maxStorage"`
	NetworkRx  float64         `json:"networkRx"`
	NetworkTx  float64         `json:"networkTx"`
	Jvm        *utils.JvmStats `json:"jvm,omitempty"`
	Running    bool            `json:"running"`
} // @name ServerStats

// CPUPercent is the CPU use as a share of what the server may use, 0 to 100. Every chart,
// card and alert reads this one number, so a two-core server at full load is 100% everywhere.
func (s *ServerStats) CPUPercent() float64 {
	if s.MaxCPU <= 0 {
		return max(s.CPU, 0)
	}
	return min(max(s.CPU/s.MaxCPU*100, 0), 100)
}

type ServerLogs struct {
	Epoch int64  `json:"epoch"`
	Logs  []byte `json:"logs"`
} // @name ServerLogs

type ServerRunning struct {
	Running    bool `json:"running"`
	Installing bool `json:"installing"`
} // @name ServerRunning

type ServerData struct {
	Variables map[string]Variable `json:"data"`
	Groups    []Group             `json:"groups,omitempty"`
} // @name ServerData

type ServerDataAdmin struct {
	*Server
}

type DaemonRunning struct {
	Message string `json:"message"`
} // @name DaemonRunning

type ServerTasks struct {
	Tasks map[string]ServerTask `json:"tasks"`
} // @name ServerTasks

type ServerTask struct {
	// IsRunning bool `json:"isRunning"`
	Task
} // @name ServerTask

type ErrorResponse struct {
	Error *Error `json:"error"`
} // @name ErrorResponse

type Metadata struct {
	Paging *Paging `json:"paging"`
} // @name Metadata

type Paging struct {
	Page    uint  `json:"page"`
	Size    uint  `json:"pageSize"`
	MaxSize uint  `json:"maxSize"`
	Total   int64 `json:"total"`
} // @name Paging

type ServerFlags struct {
	AutoStart             *bool `json:"autoStart,omitempty"`
	AutoRestartOnCrash    *bool `json:"autoRestartOnCrash,omitempty"`
	AutoRestartOnGraceful *bool `json:"autoRestartOnGraceful,omitempty"`
} // @name ServerFlags

type ServerBackupResponse struct {
	BackupFileName string `json:"backupFileName"`
} // @name ServerBackup

// BackupFile is a finished backup archive in a server's backup folder on the node
type BackupFile struct {
	FileName string    `json:"fileName"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
	// Location is "local", "offsite" or "both"
	Location string `json:"location"`
} // @name BackupFile

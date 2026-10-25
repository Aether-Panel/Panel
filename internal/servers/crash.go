package servers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// A crash report keeps what the console showed just before a server died on its own, so
// the owner can see (and have the AI explain) what happened after the fact.

const (
	// crashReportLines is how many console lines a report keeps
	crashReportLines = 60
	// crashReportLineLength trims runaway lines (stack traces with huge payloads)
	crashReportLineLength = 500
	// maxCrashReports is how many reports a server keeps, newest first
	maxCrashReports = 10
)

type CrashReport struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"time"`
	ExitCode int       `json:"exitCode"`
	// Lines are the last console lines before the crash, oldest first
	Lines []string `json:"lines"`
	// Restarted is true when the server was started again by itself
	Restarted bool `json:"restarted"`
	// Attempt is which crash in a row this was, and Limit how many restarts are allowed
	Attempt int `json:"attempt"`
	Limit   int `json:"limit"`
} // @name CrashReport

// crashFilesLock serialises reads and writes of the .crashes files
var crashFilesLock sync.Mutex

func crashFile(serverID string) string {
	return filepath.Join(config.ServersFolder.Value(), serverID+".crashes")
}

// lastConsoleLines returns the last n non-empty lines of the server's console buffer
func (p *Server) lastConsoleLines(n int) []string {
	if p.RunningEnvironment == nil || p.RunningEnvironment.ConsoleBuffer == nil {
		return []string{}
	}
	raw, _ := p.RunningEnvironment.ConsoleBuffer.Read()
	all := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")

	lines := make([]string, 0, n)
	for i := len(all) - 1; i >= 0 && len(lines) < n; i-- {
		line := strings.TrimRight(all[i], "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > crashReportLineLength {
			line = line[:crashReportLineLength] + "…"
		}
		lines = append(lines, line)
	}
	// collected newest first; reports read top to bottom
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines
}

// recordCrash saves a report and tells anyone watching the server's status
func (p *Server) recordCrash(exitCode int, restarted bool) CrashReport {
	now := time.Now()
	report := CrashReport{
		ID:        now.UTC().Format("20060102T150405.000000000"),
		Time:      now,
		ExitCode:  exitCode,
		Lines:     p.lastConsoleLines(crashReportLines),
		Restarted: restarted,
		Attempt:   p.CrashCounter,
		Limit:     config.CrashLimit.Value(),
	}

	crashFilesLock.Lock()
	reports, _ := readCrashReports(p.ID())
	reports = append([]CrashReport{report}, reports...)
	if len(reports) > maxCrashReports {
		reports = reports[:maxCrashReports]
	}
	if err := writeCrashReports(p.ID(), reports); err != nil {
		p.Log(logging.Error, "Error saving crash report: %s", err)
	}
	crashFilesLock.Unlock()

	if p.RunningEnvironment != nil && p.RunningEnvironment.StatusTracker != nil {
		_ = p.RunningEnvironment.StatusTracker.WriteMessage(skypanel.Transmission{
			Type: skypanel.MessageTypeCrash,
			Message: skypanel.ServerCrash{
				ID:        report.ID,
				ExitCode:  exitCode,
				Restarted: restarted,
			},
		})
	}
	return report
}

// CrashReports returns the server's saved reports, newest first
func (p *Server) CrashReports() ([]CrashReport, error) {
	crashFilesLock.Lock()
	defer crashFilesLock.Unlock()
	return readCrashReports(p.ID())
}

func readCrashReports(serverID string) ([]CrashReport, error) {
	data, err := os.ReadFile(crashFile(serverID))
	if os.IsNotExist(err) {
		return []CrashReport{}, nil
	}
	if err != nil {
		return nil, err
	}
	var reports []CrashReport
	if err := json.Unmarshal(data, &reports); err != nil {
		return nil, err
	}
	return reports, nil
}

func writeCrashReports(serverID string, reports []CrashReport) error {
	data, err := json.Marshal(reports)
	if err != nil {
		return err
	}
	// console output can hold player names and IPs, so only the panel's user may read it
	return os.WriteFile(crashFile(serverID), data, 0600)
}

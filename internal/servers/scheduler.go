package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/go-co-op/gocron/v2"
	"github.com/robfig/cron/v3"
)

// MaxTasks caps how many scheduled tasks one server can hold
const MaxTasks = 50

// ErrInvalidTask marks a task the scheduler refused because of what was sent, not because
// something failed on the node
var ErrInvalidTask = errors.New("invalid task")

var taskIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// cronParser reads schedules the way gocron does for CronJob(spec, true): an optional
// seconds field and an optional CRON_TZ= or TZ= prefix
var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// ValidateTask checks a task before it replaces anything, so a bad schedule is reported to
// the caller instead of leaving the server without the task it had
func ValidateTask(id string, task skypanel.Task) error {
	if !taskIDPattern.MatchString(id) {
		return fmt.Errorf("%w: the task id may only use letters, digits, - and _", ErrInvalidTask)
	}
	if task.Name == "" {
		return fmt.Errorf("%w: the task needs a name", ErrInvalidTask)
	}
	if task.CronSchedule != "" {
		if _, err := cronParser.Parse(task.CronSchedule); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidTask, err.Error())
		}
	}
	return nil
}

type Scheduler struct {
	scheduler gocron.Scheduler
	serverID  string
	// lock guards Tasks, which HTTP handlers edit while scheduled jobs read it
	lock sync.RWMutex

	Tasks           map[string]skypanel.Task `json:"tasks"`
	Timezone        string                   `json:"timezone,omitempty"`
	ConcurrentLimit uint                     `json:"concurrentLimit"`
	LimitMode       string                   `json:"limitMode"`
}

// LoadScheduler Loads the scheduler from the serverid.cron file, or defaults
// This file is a JSON file, but it hooks into everything
func LoadScheduler(serverID string) (*Scheduler, error) {
	safeID, err := validateServerID(serverID)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(config.ServersFolder.Value(), safeID+".cron"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	defer utils.Close(file)

	scheduler := NewDefaultScheduler(serverID)
	if file != nil {
		err = json.NewDecoder(file).Decode(&scheduler)
		if err != nil {
			return nil, err
		}
	}

	err = scheduler.Init()
	return scheduler, err
}

func NewDefaultScheduler(serverID string) *Scheduler {
	return &Scheduler{
		Tasks:           make(map[string]skypanel.Task),
		Timezone:        "Local",
		ConcurrentLimit: 5,
		LimitMode:       "wait",
		serverID:        serverID,
	}
}

func (s *Scheduler) Save() error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.save()
}

func (s *Scheduler) save() error {
	file, err := os.OpenFile(filepath.Join(config.ServersFolder.Value(), s.serverID+".cron"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer utils.Close(file)

	err = json.NewEncoder(file).Encode(s)
	return err
}

func (s *Scheduler) Init() error {
	if s.scheduler != nil {
		err := s.scheduler.StopJobs()
		if err != nil {
			return err
		}
	}

	var timezone *time.Location
	var err error
	if s.Timezone != "" {
		timezone, err = time.LoadLocation(s.Timezone)
		if err != nil {
			return err
		}
	} else {
		timezone = time.Local
	}

	opts := []gocron.SchedulerOption{
		gocron.WithLocation(timezone),
	}

	if s.ConcurrentLimit > 0 {
		if s.LimitMode == "reschedule" {
			opts = append(opts, gocron.WithLimitConcurrentJobs(s.ConcurrentLimit, gocron.LimitModeReschedule))
		} else {
			opts = append(opts, gocron.WithLimitConcurrentJobs(s.ConcurrentLimit, gocron.LimitModeWait))
		}
	}

	gs, err := gocron.NewScheduler(opts...)
	if err != nil {
		return err
	}

	s.scheduler = gs

	for k, v := range s.Tasks {
		err = s.addTask(k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Scheduler) Stop() {
	if s.scheduler == nil {
		return
	}
	if s.scheduler.Shutdown() == nil {
		s.scheduler = nil
	}
}

func (s *Scheduler) Start() {
	s.scheduler.Start()
}

func (s *Scheduler) IsRunning() bool {
	return s.scheduler != nil
}

// SetTask creates the task or replaces the one with the same id
func (s *Scheduler) SetTask(id string, task skypanel.Task) error {
	if err := ValidateTask(id, task); err != nil {
		return err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.Tasks[id]; !exists && len(s.Tasks) >= MaxTasks {
		return fmt.Errorf("%w: a server can have at most %d tasks", ErrInvalidTask, MaxTasks)
	}

	s.removeJob(id)
	if err := s.addTask(id, task); err != nil {
		return err
	}
	return s.save()
}

func (s *Scheduler) addTask(id string, task skypanel.Task) error {
	var opt gocron.JobDefinition

	if task.CronSchedule != "" {
		opt = gocron.CronJob(task.CronSchedule, true)
	} else {
		opt = gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)))
	}

	_, err := s.scheduler.NewJob(opt, gocron.NewTask(runScheduledTask, s.serverID, id), gocron.WithName(id))
	if err != nil {
		return err
	}

	s.Tasks[id] = task
	return nil
}

// RemoveTask deletes the task, or returns gocron.ErrJobNotFound when there is none
func (s *Scheduler) RemoveTask(id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.Tasks[id]; !exists {
		return gocron.ErrJobNotFound
	}
	s.removeJob(id)
	delete(s.Tasks, id)
	return s.save()
}

func (s *Scheduler) removeJob(id string) {
	for _, v := range s.scheduler.Jobs() {
		if v.Name() == id {
			_ = s.scheduler.RemoveJob(v.ID())
		}
	}
}

func (s *Scheduler) RunTask(id string) error {
	jobs := s.scheduler.Jobs()
	for _, v := range jobs {
		if v.Name() == id {
			return v.RunNow()
		}
	}
	return gocron.ErrJobNotFound
}

// GetTasks returns a copy of the tasks, safe to read while tasks are being edited
func (s *Scheduler) GetTasks() map[string]skypanel.Task {
	s.lock.RLock()
	defer s.lock.RUnlock()

	result := make(map[string]skypanel.Task, len(s.Tasks))
	for k, v := range s.Tasks {
		result[k] = v
	}
	return result
}

func (s *Scheduler) GetTask(id string) (skypanel.Task, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	task, ok := s.Tasks[id]
	return task, ok
}

// runScheduledTask is what gocron calls when one of a server's tasks is due
func runScheduledTask(serverID, id string) {
	p := GetFromCache(serverID)
	if p == nil || p.Scheduler == nil {
		return
	}
	var err error

	task, ok := p.Scheduler.GetTask(id)
	if !ok {
		return
	}

	ops := task.Operations
	if len(ops) > 0 {
		p.RunningEnvironment.DisplayToConsole(true, "Running task %s\n", task.Name)
		var process OperationProcess
		process, err = GenerateProcess(ops, p.GetEnvironment(), p.DataToMap(), p.Execution.EnvironmentVariables)
		if err != nil {
			logging.Error.Printf("Error setting up tasks: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to setup tasks\n")
			p.RunningEnvironment.DisplayToConsole(true, "%s\n", err.Error())
			return
		}

		err = process.Run(p)
		if err != nil {
			logging.Error.Printf("Error setting up tasks: %s", err)
			p.RunningEnvironment.DisplayToConsole(true, "Failed to setup tasks\n")
			p.RunningEnvironment.DisplayToConsole(true, "%s\n", err.Error())
			return
		}
		p.RunningEnvironment.DisplayToConsole(true, "Task %s finished\n", task.Name)
	}
}

func (s *Scheduler) GetExecutor() gocron.Scheduler {
	return s.scheduler
}

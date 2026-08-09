package tty

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/creack/pty"
	"github.com/shirou/gopsutil/mem"
	psnet "github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cast"
)

type tty struct {
	mainProcess   *exec.Cmd
	processLocker sync.RWMutex
	statLocker    sync.Mutex
	lastStats     *skypanel.ServerStats
	lastStatTime  time.Time
	lastNetworkRx uint64
	lastNetworkTx uint64
	lastNetTime   time.Time
	// disableStdin        bool
	disableSpecialStats bool

	DisableUnshare bool     `json:"disableUnshare"`
	Mounts         []string `json:"mounts"`

	dirSize     int64
	dirSizeTime time.Time
}

func (t *tty) ExecuteAsyncImpl(environment *skypanel.Environment, steps skypanel.ExecutionData) (err error) {
	environment.Wait.Add(1)

	pr, err := t.createCmd(environment.GetRootDirectory(), steps.Command)
	if err != nil {
		return err
	}

	var envVars = make(map[string]string)

	for _, v := range os.Environ() {
		key, value, valid := strings.Cut(v, "=")
		if !valid {
			continue
		}
		if strings.HasPrefix(key, "PUFFER_") {
			continue
		}
		envVars[key] = value
	}
	envVars["HOME"] = environment.GetRootDirectory()
	envVars["TERM"] = "xterm-256color"
	for k, v := range steps.Environment {
		envVars[k] = v
	}

	// Ensure binaries folder is in PATH
	binDir := config.BinariesFolder.Value()
	jailBinDir := binDir
	// If the binaries folder is relative and we're using unshare, it needs to be absolute within the jail
	if !filepath.IsAbs(jailBinDir) && !config.SecurityDisableUnshare.Value() && !t.DisableUnshare {
		jailBinDir = "/" + jailBinDir
	}

	if currentPath, ok := envVars["PATH"]; ok {
		if !strings.Contains(currentPath, jailBinDir) {
			envVars["PATH"] = jailBinDir + ":" + currentPath
		}
	} else {
		envVars["PATH"] = jailBinDir + ":/usr/local/bin:/usr/bin:/bin"
	}

	for k, v := range envVars {
		pr.Env = append(pr.Env, fmt.Sprintf("%s=%s", k, v))
	}

	environment.DisplayToConsole(true, "Starting process: %s", steps.Command)
	environment.Log(logging.Info, "Starting process in directory [%s]: %s", pr.Dir, strings.Join(pr.Args, " "))

	_ = environment.StatusTracker.WriteMessage(skypanel.Transmission{
		Message: skypanel.ServerRunning{
			Running:    true,
			Installing: environment.IsInstalling(),
		},
		Type: skypanel.MessageTypeStatus,
	})

	t.disableSpecialStats = steps.DisableStats
	// t.disableStdin = steps.DisableStdin

	processTty, err := pty.Start(pr)
	if err != nil {
		environment.Wait.Done()
		if strings.Contains(err.Error(), "permission denied") && !config.SecurityDisableUnshare.Value() && !t.DisableUnshare {
			environment.DisplayToConsole(true, "------------------------------------------------------------")
			environment.DisplayToConsole(true, "ERROR: Permission denied while starting the security jail.")
			environment.DisplayToConsole(true, "Your OS (e.g. Ubuntu 24.04) might be restricting unprivileged user namespaces.")
			environment.DisplayToConsole(true, "FIX 1 (Recommended): Run 'sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0'")
			environment.DisplayToConsole(true, "FIX 2: Add '\"security\": {\"disableUnshare\": true}' to your config.json and restart.")
			environment.DisplayToConsole(true, "------------------------------------------------------------")
		}
		environment.DisplayToConsole(true, "Failed to start process: %s", err)
		return
	}

	// Publish the process only after pty.Start has fully initialized it
	// (pty.Start -> Cmd.Start writes cmd.Process). Publishing earlier lets a
	// concurrent Stop/IsRunning observe a half-initialized command, which the
	// race detector flags (data race between Cmd.Start and IsRunningImpl).
	t.processLocker.Lock()
	t.mainProcess = pr
	t.processLocker.Unlock()

	// if !t.disableStdin {
	//	environment.CreateConsoleStdinProxy(steps.StdInConfig, processTty)
	//}
	environment.CreateConsoleStdinProxy(steps.StdInConfig, processTty)

	environment.Console.Start()

	go func(proxy io.Writer) {
		_, _ = io.Copy(proxy, processTty)
	}(environment.Wrapper)

	go t.handleClose(environment, steps.Callback)
	return
}

func (t *tty) KillImpl(environment *skypanel.Environment) (err error) {
	running, err := environment.IsRunning()
	if err != nil {
		return
	}
	if !running {
		return
	}
	t.processLocker.RLock()
	mainProcess := t.mainProcess
	t.processLocker.RUnlock()
	return mainProcess.Process.Kill()
}

func (t *tty) GetStatsImpl(environment *skypanel.Environment) (*skypanel.ServerStats, error) {
	running, err := environment.IsRunning()
	if err != nil {
		return nil, err
	}
	if !running {
		stats := &skypanel.ServerStats{
			CPU:     0,
			Memory:  0,
			Running: false,
		}

		if environment.Server.Stats.Type == "jcmd" {
			stats.Jvm = &utils.JvmStats{}
		}

		return stats, nil
	}

	t.statLocker.Lock()
	defer t.statLocker.Unlock()

	// only fetch stats once every 5 seconds, to avoid excessive spam
	if t.lastStatTime.Add(5 * time.Second).After(time.Now()) {
		return t.lastStats, nil
	}

	t.processLocker.RLock()
	mainProcess := t.mainProcess
	t.processLocker.RUnlock()
	if mainProcess == nil || mainProcess.Process == nil {
		return nil, errors.New("process is not running")
	}

	pr, err := process.NewProcess(int32(mainProcess.Process.Pid))
	if err != nil {
		return nil, err
	}

	memMap, _ := pr.MemoryInfo()
	cpu, _ := pr.Percent(time.Second * 1)

	memInfo, _ := mem.VirtualMemory()

	now := time.Now()
	var rxRate, txRate float64
	if netStats, err := psnet.IOCounters(false); err == nil && len(netStats) > 0 {
		totalRx := netStats[0].BytesRecv
		totalTx := netStats[0].BytesSent
		if !t.lastNetTime.IsZero() {
			elapsed := now.Sub(t.lastNetTime).Seconds()
			if elapsed > 0 {
				rxRate = (float64(totalRx-t.lastNetworkRx) / elapsed) / 1024
				txRate = (float64(totalTx-t.lastNetworkTx) / elapsed) / 1024
			}
		}
		t.lastNetworkRx = totalRx
		t.lastNetworkTx = totalTx
	}
	t.lastNetTime = now

	if t.dirSizeTime.IsZero() || time.Since(t.dirSizeTime) > 30*time.Second {
		t.dirSize = getDirSize(environment.GetRootDirectory())
		t.dirSizeTime = time.Now()
	}

	var maxStorage float64
	if diskVar, ok := environment.Server.Variables["disk"]; ok {
		if limit, err := cast.ToInt64E(diskVar.Value); err == nil && limit > 0 {
			maxStorage = float64(limit) * 1024 * 1024
		}
	}

	stats := &skypanel.ServerStats{
		CPU:        cpu,
		Memory:     cast.ToFloat64(memMap.RSS),
		MaxMemory:  cast.ToFloat64(memInfo.Total),
		Disk:       float64(t.dirSize),
		MaxStorage: maxStorage,
		NetworkRx:  rxRate,
		NetworkTx:  txRate,
		Running:    true,
	}

	if !t.disableSpecialStats && environment.Server.Stats.Type == "jcmd" {
		var socket *net.UnixConn
		if socket, err = t.initiateJCMD(); err == nil && socket != nil {
			for _, s := range []string{"1", "\x00", "jcmd", "\x00", "GC.heap_info", "\x00", "\x00", "\x00"} {
				_, err = socket.Write([]byte(s))
				if err != nil {
					logging.Error.Printf("unable to send command to Java process: %v", err)
					break
				}
			}
			// only continue parsing if no errors sending command
			if err == nil {
				var jcmdData []byte
				jcmdData, err = io.ReadAll(socket)
				if err != nil {
					logging.Error.Printf("Could not get result of JCMD: %s", err.Error())
				}

				stats.Jvm = utils.ParseJCMDResponse(jcmdData)
			}
		}
		if stats.Jvm == nil {
			stats.Jvm = &utils.JvmStats{}
		}
	}

	t.lastStats = stats

	return stats, nil
}

func (t *tty) SendCodeImpl(environment *skypanel.Environment, code int) error {
	running, err := environment.IsRunning()

	if err != nil || !running {
		return err
	}

	t.processLocker.RLock()
	mainProcess := t.mainProcess
	t.processLocker.RUnlock()
	if mainProcess == nil || mainProcess.Process == nil {
		return errors.New("process is not running")
	}
	return mainProcess.Process.Signal(syscall.Signal(code))
}

func (t *tty) GetUIDImpl(*skypanel.Environment) int {
	return -1
}

func (t *tty) GetGidImpl(*skypanel.Environment) int {
	return -1
}

func (t *tty) IsRunningImpl(*skypanel.Environment) (isRunning bool, err error) {
	t.processLocker.RLock()
	mainProcess := t.mainProcess
	t.processLocker.RUnlock()
	isRunning = mainProcess != nil && mainProcess.Process != nil
	if isRunning {
		pr, pErr := os.FindProcess(mainProcess.Process.Pid)
		if pr == nil || pErr != nil {
			isRunning = false
		} else if pr.Signal(syscall.Signal(0)) != nil {
			isRunning = false
		}
	}
	return
}

func (t *tty) handleClose(environment *skypanel.Environment, callback func(exitCode int)) {
	t.processLocker.RLock()
	mainProcess := t.mainProcess
	t.processLocker.RUnlock()
	if mainProcess == nil {
		environment.Wait.Done()
		return
	}

	err := mainProcess.Wait()

	_ = environment.Console.Close()

	var exitCode int
	if mainProcess.ProcessState == nil || err != nil {
		var psErr *exec.ExitError
		if errors.As(err, &psErr) {
			exitCode = psErr.ExitCode()
		} else {
			exitCode = 1
		}
	} else {
		exitCode = mainProcess.ProcessState.ExitCode()
	}
	environment.LastExitCode = exitCode

	if err != nil {
		environment.Log(logging.Error, "Error waiting on process: %s\n", err)
		environment.DisplayToConsole(true, "Error waiting on process: %s", err)
	}

	if mainProcess.ProcessState != nil {
		environment.Log(logging.Debug, "%s\n", mainProcess.ProcessState.String())
	}

	if mainProcess.Process != nil {
		_ = mainProcess.Process.Release()
	}

	t.statLocker.Lock()
	//lint:ignore SA2001 used as a barrier
	//nolint:staticcheck // used as a barrier
	t.statLocker.Unlock()

	// if we are using unshare AND we're in tmp, we can nuke the workspace at this point
	if !t.DisableUnshare && strings.HasPrefix(mainProcess.Dir, os.TempDir()) {
		err = os.RemoveAll(mainProcess.Dir)
		if err != nil {
			logging.Debug.Printf("Failed to delete %s: %s", mainProcess.Dir, err.Error())
		}
	}

	t.processLocker.Lock()
	t.mainProcess = nil
	t.processLocker.Unlock()

	environment.Wait.Done()

	_ = environment.StatusTracker.WriteMessage(skypanel.Transmission{
		Message: skypanel.ServerRunning{
			Running:    false,
			Installing: environment.IsInstalling(),
		},
		Type: skypanel.MessageTypeStatus,
	})

	// t.disableStdin = false
	t.disableSpecialStats = false

	if callback != nil {
		callback(exitCode)
	}
}

func activateAttachAPI(pid int) error {
	// It's not, lets do a quick ceremony of touching a file and
	// sending SIGQUIT to activate this feature
	attachpath := attachPath(pid)
	if err := os.WriteFile(attachpath, nil, 0660); err != nil {
		return fmt.Errorf("could not touch file to activate attach api: %w", err)
	}

	defer func() {
		_ = os.Remove(attachpath)
	}()

	proc, err := os.FindProcess(pid)
	if err != nil { // can't happen on unix
		return fmt.Errorf("could not find process: %w", err)
	}

	if err = proc.Signal(syscall.SIGQUIT); err != nil {
		return fmt.Errorf("could not send signal 3 to activate attach API: %w", err)
	}

	// Check if the UNIX socket is active
	sock := socketPath(pid)
	for i := 1; i < 10; i++ {
		if _, err = os.Stat(sock); err != nil && !os.IsNotExist(err) {
			return err
		}

		// exponential backoff
		time.Sleep(time.Duration(1<<uint(i)) * time.Millisecond)
	}

	// if we got here, then the file wasn't available or otherwise not good anymore
	return err
}

func attachPath(pid int) string {
	return fmt.Sprintf("/proc/%v/cwd/.attach_pid%v", pid, pid)
}

func socketPath(pid int) string {
	return fmt.Sprintf("/proc/%v/root/tmp/.java_pid%v", pid, pid)
}

func (t *tty) initiateJCMD() (*net.UnixConn, error) {
	t.processLocker.RLock()
	mainProcess := t.mainProcess
	t.processLocker.RUnlock()
	if mainProcess == nil || mainProcess.Process == nil {
		return nil, errors.New("process is not running")
	}
	pid := mainProcess.Process.Pid
	sock := socketPath(pid)

	// Check if the UNIX socket is active
	if _, err := os.Stat(sock); err != nil && os.IsNotExist(err) {
		if err = activateAttachAPI(pid); err != nil {
			return nil, err
		}
	}

	addr, err := net.ResolveUnixAddr("unix", sock)
	if err != nil {
		return nil, err // can't happen (on linux)
	}

	return net.DialUnix("unix", nil, addr)
}

var cmdList = []string{
	"mount --make-rprivate --make-rslave --bind . .",
	"mkdir -p {dev,bin,usr,lib,etc,tmp,proc}",
	"mount -t tmpfs -o size=50m tmpfs tmp",
	"mount --bind /bin bin",
	"mount --bind /lib lib",
	"mount --rbind /usr usr",
	"mount --rbind /etc etc",
	"mount --rbind /dev dev",
	"mount --rbind /proc proc",
}

func (t *tty) createCmd(workDir, cmd string) (pr *exec.Cmd, err error) {
	if t.DisableUnshare || config.SecurityDisableUnshare.Value() {
		c, args := utils.SplitArguments(cmd)
		pr = exec.Command(c, args...)
		pr.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
		pr.Dir = workDir
		return
	}

	workDirMount := removeRoot(workDir)
	binaryFolderMount := removeRoot(config.BinariesFolder.Value())
	cacheFolderMount := removeRoot(config.CacheFolder.Value())

	mountFolders := []string{workDirMount, binaryFolderMount, cacheFolderMount}
	for _, v := range t.Mounts {
		mountFolders = append(mountFolders, removeRoot(v))
	}

	unshareArgs := make([]string, len(cmdList))
	copy(unshareArgs, cmdList)

	if runtime.GOARCH == "amd64" {
		unshareArgs = append(unshareArgs,
			"mkdir -p lib64",
			"mount --bind /lib64 lib64",
		)
	}

	var lstat os.FileInfo
	lstat, err = os.Lstat("/etc/resolv.conf")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	if err == nil && lstat.Mode()&os.ModeSymlink != 0 {
		var absPath string
		absPath, err = filepath.EvalSymlinks("/etc/resolv.conf")
		if err != nil {
			return
		}
		localPath := removeRoot(absPath)
		dir := removeRoot(filepath.Dir(absPath))
		unshareArgs = append(unshareArgs,
			fmt.Sprintf("mkdir -p %s", dir),
			fmt.Sprintf("touch %s", localPath),
			fmt.Sprintf("mount --rbind %s %s", absPath, localPath),
		)
	}

	absWorkDir, _ := filepath.Abs(workDir)
	absBinDir, _ := filepath.Abs(config.BinariesFolder.Value())
	absCacheDir, _ := filepath.Abs(config.CacheFolder.Value())

	mkdirDirs := make([]string, len(mountFolders))
	for i, f := range mountFolders {
		mkdirDirs[i] = shellQuote(f)
	}
	unshareArgs = append(unshareArgs,
		fmt.Sprintf("mkdir -p %s", strings.Join(mkdirDirs, " ")),
		fmt.Sprintf("mount --bind %s %s", shellQuote(absWorkDir), shellQuote(workDirMount)),
		fmt.Sprintf("mount --bind %s %s", shellQuote(absBinDir), shellQuote(binaryFolderMount)),
		fmt.Sprintf("mount --bind %s %s", shellQuote(absCacheDir), shellQuote(cacheFolderMount)),
	)

	for _, v := range t.Mounts {
		absV, _ := filepath.Abs(v)
		unshareArgs = append(unshareArgs, fmt.Sprintf("mount --bind %s %s", shellQuote(absV), shellQuote(removeRoot(v))))
	}

	unshareArgs = append(unshareArgs,
		// move cwd to bind mounted instace of .
		"cd .",
		"mkdir -p old-root",
		// make . the root for everything in the current namespace
		"pivot_root . old-root",
		// make the old root unaccessible by unmounting it
		// needs to be lazy because the old root is considered busy as it's still the root outside the namespace
		"umount -l /old-root",
		"rm -r /old-root",
		safeCmd(workDirMount, cmd))

	scriptDir, err := os.MkdirTemp("", "unshare-pp-")
	if err != nil {
		return
	}
	scriptPath := filepath.Join(scriptDir, "unshare.sh")
	script := "#!/bin/bash\nset -e\n" + strings.Join(unshareArgs, "\n")
	if err = os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return
	}
	pr = exec.Command("/bin/bash", scriptPath)
	pr.Dir = scriptDir
	pr.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
		Unshareflags: syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNS |
			syscall.CLONE_FILES |
			syscall.CLONE_NEWCGROUP |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWUTS,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	return
}

func removeRoot(path string) string {
	return strings.TrimPrefix(path, "/")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func safeCmd(workDirMount, cmd string) string {
	c, args := utils.SplitArguments(cmd)
	var parts []string
	parts = append(parts, shellQuote(c))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return fmt.Sprintf("cd /%s && %s", shellQuote(workDirMount), strings.Join(parts, " "))
}

func getDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

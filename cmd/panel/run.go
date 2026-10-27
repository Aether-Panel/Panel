package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/extensions"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers/docker"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/sftp"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/internal/web"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:    "run",
	Short:  "Runs the panel",
	Run:    executeRun,
	Hidden: true,
}

var webService *http.Server

func executeRun(_ *cobra.Command, _ []string) {
	term, _ := internalRun()
	<-term
	closePanel()
}

func internalRun() (terminate chan bool, success bool) {
	logging.Initialize(true)
	signal.Ignore(syscall.SIGPIPE, syscall.SIGHUP)

	terminate = make(chan bool, 2)

	go func() {
		quit := make(chan os.Signal, 1)
		// kill (no param) default send syscall.SIGTERM
		// kill -2 is syscall.SIGINT
		// kill -9 is syscall.SIGKILL but can"t be catch, so don't need add it
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logging.Info.Println("Shutting down...")
		terminate <- true
	}()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithWriter(logging.Info.Writer()))

	configureTrustedProxies(router)

	gin.DefaultWriter = logging.Info.Writer()
	gin.DefaultErrorWriter = logging.Error.Writer()
	skypanel.Engine = router

	if config.PanelEnabled.Value() && !setupPanel(router) {
		terminate <- true
		return
	}

	if config.DaemonEnabled.Value() && !startDaemon() {
		terminate <- true
		return
	}

	if !config.PanelEnabled.Value() {
		warnPanelDisabled()
	}

	web.RegisterRoutes(router)

	l, err := net.Listen("tcp", config.WebHost.Value())
	if err != nil {
		logging.Error.Printf("error starting http server: %s", err.Error())
		terminate <- true
		return
	}

	logging.Info.Printf("Listening for HTTP requests on %s", l.Addr().String())
	webService = &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		err = webService.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Error.Printf("error listening for http requests: %s", err.Error())
			terminate <- true
		}
	}()

	success = true
	return
}

// configureTrustedProxies trusts no proxies unless the config lists some.
func configureTrustedProxies(router *gin.Engine) {
	// do not trust proxies by default
	_ = router.SetTrustedProxies(nil)
	if proxies := config.SecurityTrustedProxies.Value(); proxies != nil {
		err := router.SetTrustedProxies(proxies)
		if err != nil {
			logging.Error.Printf("Failed to add trusted proxies: %s", err.Error())
		}
	}
	if header := config.SecurityTrustedProxyHeader.Value(); header != "" {
		router.TrustedPlatform = header
	}
}

// startDaemon starts the daemon and returns false when the panel has to stop.
func startDaemon() bool {
	err := daemon()
	if err == nil {
		return true
	}
	logging.Error.Printf("error starting daemon server: %s", err.Error())
	// En Docker, si el daemon falla por permisos del socket, continuar sin el daemon
	// El panel web y Gatus pueden funcionar sin el daemon
	if os.Getenv("SKYPANEL_PLATFORM") == "docker" {
		logging.Info.Printf("Continuing without daemon in Docker environment")
		return true
	}
	return false
}

func warnPanelDisabled() {
	logging.Info.Printf("Panel web routes are disabled. Only Daemon API is active.")
	if config.TokenPublicURL.Value() == "" {
		logging.Error.Printf("WARNING: Panel is disabled but token.public (TokenPublicURL) is empty!")
		logging.Error.Printf("This node will not be able to authenticate requests from the Master Panel.")
		logging.Error.Printf("Please configure it in config.json or set SKYPANEL_TOKEN_PUBLIC in docker-compose.")
	}
}

// setupPanel prepares the database and the session store for the web panel. It logs and
// returns false when the panel cannot start.
func setupPanel(router *gin.Engine) bool {
	panel()

	db, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("error connecting to database: %s", err.Error())
		return false
	}

	err = database.Upgrade(db, false)
	if err != nil {
		logging.Error.Printf("error upgrading database: %s", err.Error())
		return false
	}

	if config.SessionKey.Value() == "" {
		k := securecookie.GenerateRandomKey(32)
		if err := config.SessionKey.Set(hex.EncodeToString(k), true); err != nil {
			logging.Error.Printf("error saving session key: %s", err.Error())
			return false
		}
	}

	result, err := hex.DecodeString(config.SessionKey.Value())
	if err != nil {
		logging.Error.Printf("error decoding session key: %s", err.Error())
		return false
	}

	// Derive the local node secret from the persisted session key so the
	// local node keeps a stable identity across restarts (the secret is
	// otherwise regenerated in memory on every boot).
	models.SetLocalNodeSecretFromSeed(config.SessionKey.Value())

	sessionStore := cookie.NewStore(result)
	sessionStore.Options(sessions.Options{
		Path:     config.PanelWebCookiesPath.Value(),
		Domain:   config.PanelWebCookiesDomain.Value(),
		MaxAge:   config.PanelWebCookiesAge.Value(),
		Secure:   config.PanelWebCookiesSecure.Value(),
		HttpOnly: config.PanelWebCookiesHTTPOnly.Value(),
		SameSite: cookieSameSite(config.PanelWebCookiesSameSite.Value()),
	})
	router.Use(sessions.Sessions("session", sessionStore))

	if config.DaemonEnabled.Value() {
		services.SyncNodeToConfig()
	}
	return true
}

// cookieSameSite maps the configured SameSite name, falling back to Strict.
func cookieSameSite(name string) http.SameSite {
	switch name {
	case "None":
		return http.SameSiteNoneMode
	case "Lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteStrictMode
	}
}

func closePanel() {
	// shut down everything
	// all of these can be closed regardless of what type of install this is, as they all check if they are even being
	// used
	logging.Debug.Printf("stopping http server")
	if webService != nil {
		// let requests in progress finish, without holding the shutdown for slow ones
		ctx, cancel := context.WithTimeout(context.Background(), webShutdownTimeout)
		if err := webService.Shutdown(ctx); err != nil {
			logging.Error.Printf("error stopping http server: %s", err.Error())
			_ = webService.Close()
		}
		cancel()
	}

	logging.Debug.Printf("stopping sftp server")
	sftp.Stop()

	logging.Debug.Printf("stopping servers")
	servers.ShutdownService()
	stopAllServers(serverStopTimeout)

	logging.Debug.Printf("stopping database connections")
	database.Close()
}

const (
	webShutdownTimeout = 10 * time.Second
	// serverStopTimeout is how long each game server gets to stop before it is killed; all of
	// them stop at once, so it is also roughly the whole wait (compose gives the panel 90s)
	serverStopTimeout = time.Minute
)

// stopAllServers asks every server to stop at the same time. Stopping them one after
// another took up to a minute each, so with several servers most were never asked before
// Docker killed the panel.
func stopAllServers(timeout time.Duration) {
	var wg sync.WaitGroup
	for _, p := range servers.GetAll() {
		wg.Add(1)
		go func(p *servers.Server) {
			defer wg.Done()
			_ = p.Stop()
			if err := p.RunningEnvironment.WaitForMainProcessFor(timeout); err != nil {
				logging.Error.Printf("[%s] error stopping server: %s", p.ID(), err.Error())
			}
		}(p)
	}
	wg.Wait()
}

func panel() {
	services.LoadEmailService()

	// if we have the web, then let's use our sftp auth instead
	sftp.SetAuthorization(&services.DatabaseSFTPAuthorization{})

	// server events also go to the installed extensions
	servers.EventSink = extensions.Publish
}

func daemon() error {
	utils.DetermineKernelSupport()

	sftp.Run()

	var err error

	err = docker.InitContainerMountSource()
	if err != nil {
		return err
	}

	if _, err = os.Stat(config.ServersFolder.Value()); os.IsNotExist(err) {
		logging.Info.Printf("No server directory found, creating")
		err = os.MkdirAll(config.ServersFolder.Value(), 0755)
		if err != nil && !os.IsExist(err) {
			return err
		}
	}

	err = os.MkdirAll(config.BinariesFolder.Value(), 0755)
	if err != nil {
		logging.Error.Printf("Error creating binaries folder: %s", err.Error())
	}

	err = os.MkdirAll(config.CacheFolder.Value(), 0755)
	if err != nil {
		logging.Error.Printf("Error creating cache folder: %s", err.Error())
	}

	// update path to include our binary folder
	newPath := os.Getenv("PATH")
	fullPath, _ := filepath.Abs(config.BinariesFolder.Value())
	if !strings.Contains(newPath, fullPath) {
		_ = os.Setenv("PATH", fmt.Sprintf("%s%c%s", newPath, os.PathListSeparator, fullPath))
	}
	logging.Debug.Printf("Daemon PATH variable: %s", os.Getenv("PATH"))

	servers.LoadFromFolder()

	servers.InitService()

	for _, element := range servers.GetAll() {
		element.GetEnvironment().DisplayToConsole(true, "Daemon has been started\n")
		if element.IsAutoStart() {
			logging.Info.Printf("Queued server %s", element.ID())
			element.GetEnvironment().DisplayToConsole(true, "Server has been queued to start\n")
			servers.StartViaService(element)
		}
	}
	return nil
}

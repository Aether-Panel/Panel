package tests

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/oauth2"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	skypanelSftp "github.com/SkyPanel/SkyPanel/v3/internal/sftp"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestServers(t *testing.T) {
	db, initErr := database.GetConnection()
	if !assert.NoError(t, initErr) {
		return
	}

	session, initErr := createSessionAdmin()
	if !assert.NoError(t, initErr) {
		return
	}

	type testLocation struct {
		SFTPAuth skypanel.SFTPAuthorization
		Node     *models.Node
	}

	RemoteNode.PublicPort = models.LocalNode.PublicPort
	RemoteNode.PrivatePort = models.LocalNode.PrivatePort
	RemoteNode.SFTPPort = models.LocalNode.SFTPPort

	initErr = db.Create(RemoteNode).Error
	if !assert.NoError(t, initErr) {
		return
	}

	tests := []struct {
		name string
		test testLocation
	}{
		{
			name: "Local",
			test: testLocation{
				SFTPAuth: &services.DatabaseSFTPAuthorization{},
				Node:     models.LocalNode,
			},
		},
		{
			name: "Remote",
			test: testLocation{
				SFTPAuth: &oauth2.WebSSHAuthorization{},
				Node:     RemoteNode,
			},
		},
	}

	for _, tt := range tests {
		name := tt.name
		test := tt.test
		t.Run(name, func(t *testing.T) {
			skypanelSftp.SetAuthorization(test.SFTPAuth)
			_ = config.ClientSecret.Set(test.Node.Secret, false)
			var serverID = "testserver-" + strings.ToLower(name)
			c := &serverTestCtx{
				db:        db,
				session:   session,
				node:      test.Node,
				serverID:  serverID,
				serverDir: filepath.Join(config.ServersFolder.Value(), serverID),
			}
			runServerLocationTests(t, c)
		})
	}
}

// serverTestCtx carries the per-location state shared by the server subtests
type serverTestCtx struct {
	db        *gorm.DB
	session   string
	node      *models.Node
	serverID  string
	serverDir string
	taskID    string
}

// socketFlags records which websocket message types arrived. The reader goroutine sets them
// while the test reads them, so they are atomic.
type socketFlags struct {
	statsReceived   atomic.Bool
	messageReceived atomic.Bool
	statusReceived  atomic.Bool
}

func runServerLocationTests(t *testing.T, c *serverTestCtx) {
	t.Run("CreateServer", c.testCreateServer)
	t.Run("EnsureServerListContains1", c.testServerListContains)
	t.Run("EnsureSecondUserCannotSee", c.testSecondUserCannotSee)
	t.Run("AdminUpdate", c.testAdminUpdate)
	t.Run("AdminDataUpdate", c.testAdminDataUpdate)

	if t.Failed() {
		return
	}

	// previous test is a block,so we can now open up a websocket connection and start playing with it
	// the test here is... do we get all 3 types of messages
	flags := &socketFlags{}

	addr := fmt.Sprintf("%s:%d", models.LocalNode.PrivateHost, models.LocalNode.PrivatePort)

	u := fmt.Sprintf("ws://%s/api/servers/%s/socket", addr, c.serverID)

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.session)

	conn, _, webErr := websocket.DefaultDialer.Dial(u, header)
	if !assert.NoError(t, webErr) {
		return
	}
	var listening atomic.Bool
	listening.Store(true)
	defer utils.Close(conn)

	go readServerSocket(conn, &listening, flags)

	t.Run("AddSubUser", c.testAddSubUser)
	t.Run("GetSubUsers", c.testGetSubUsers)
	t.Run("GrantUserPermissions", c.testGrantUserPermissions)
	t.Run("SubuserGrantUserPermissions", c.testSubuserGrantUserPermissions)
	t.Run("UpdateVariable", c.testUpdateVariable)

	t.Run("GetStats", func(t *testing.T) {
		response := CallAPI("GET", "/api/servers/"+c.serverID+"/stats", nil, c.session)
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("GetStatsSecondUserFail", c.testGetStatsSecondUserFail)

	t.Run("SendStatsForServers", func(_ *testing.T) {
		servers.SendStatsForServers()
	})

	t.Run("GetEmptyFiles", func(t *testing.T) {
		response := CallAPI("GET", "/api/servers/"+c.serverID+"/file/", nil, c.session)
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("InstallServer", func(t *testing.T) {
		t.Skip("Skipping install test in unit tests to avoid long downloads")
	})

	t.Run("StartServer", func(t *testing.T) {
		response := CallAPI("POST", "/api/servers/"+c.serverID+"/start", nil, c.session)
		assert.Equal(t, http.StatusAccepted, response.Code)

		time.Sleep(1000 * time.Millisecond)
	})

	t.Run("StopServer", c.testStopServer)

	listening.Store(false)
	_ = conn.Close()

	// create a fake file that we can use to both
	t.Run("Compression", c.testCompression)

	t.Run("SFTP", func(t *testing.T) {
		t.Run("Admin", func(t *testing.T) {
			c.testSFTPLogin(t, loginAdminUser.Email, loginAdminUserPassword)
		})

		t.Run("User", func(t *testing.T) {
			c.testSFTPLogin(t, loginSftpUser.Email, loginSftpUserPassword)
		})

		t.Run("NonUser", c.testSFTPNonUser)
	})

	c.taskID = "testtask"
	t.Run("CreateTask", c.testCreateTask)
	t.Run("GetTasks", c.testGetTasks)
	t.Run("GetTask", c.testGetTask)
	t.Run("RunTask", c.testRunTask)
	t.Run("EditTask", c.testEditTask)
	t.Run("DeleteTask", c.testDeleteTask)

	t.Run("FileManager", c.testFileManager)

	t.Run("Delete", c.testDeleteServer)

	t.Run("WebSocketReceivedAll", func(t *testing.T) {
		assert.True(t, flags.statsReceived.Load(), "Stats were not received")
		assert.True(t, flags.statusReceived.Load(), "Status was not received")
		assert.True(t, flags.messageReceived.Load(), "Console messages were not received")
	})
}

// readServerSocket drains the socket until listening is cleared, flagging each message type seen
func readServerSocket(conn *websocket.Conn, listening *atomic.Bool, flags *socketFlags) {
	for listening.Load() {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("Error on websocket: %s\n", err.Error())
			continue
		}
		if messageType != websocket.TextMessage {
			fmt.Printf("Unexpected message type [%d]: %s\n", messageType, data)
			continue
		}
		flags.handle(data)
	}
}

func (f *socketFlags) handle(data []byte) {
	var msg map[string]interface{}
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&msg)
	if err != nil {
		fmt.Printf("Failed to decode message: %s\n", err.Error())
		return
	}

	msgData := msg["data"]

	switch msg["type"].(string) {
	case skypanel.MessageTypeLog:
		var ms skypanel.ServerLogs
		err = utils.UnmarshalTo(msgData, &ms)
		if err != nil {
			fmt.Printf("Failed to decode message: %s\n", err.Error())
			return
		}

		if config.ConsoleForward.Value() {
			fmt.Printf("[WEBSOCKET] %s\n", ms.Logs)
		}

		f.messageReceived.Store(true)
	case skypanel.MessageTypeStatus:
		f.statusReceived.Store(true)
	case skypanel.MessageTypeStats:
		f.statsReceived.Store(true)
	default:
		fmt.Printf("unknown message type: %s\n", msg["type"])
	}
}

func (c *serverTestCtx) testCreateServer(t *testing.T) {
	data := CreateServerData
	data = strings.Replace(data, "{{{INSERTNODEID}}}", fmt.Sprintf("%d", c.node.ID), 1)
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID, []byte(data), c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		t.Log(response.Body.String())
		return
	}

	var count int64
	err := c.db.Model(&models.Server{}).Where(&models.Server{Identifier: c.serverID}).Count(&count).Error
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, int64(1), count) {
		t.Logf("Server count is %d, expected 1", count)
		return
	}

	if !assert.DirExists(t, filepath.Join(config.ServersFolder.Value(), c.serverID)) {
		return
	}
}

func (c *serverTestCtx) testServerListContains(t *testing.T) {
	response := CallAPI("GET", "/api/servers", nil, c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var s *models.ServerSearchResponse
	err := json.NewDecoder(response.Body).Decode(&s)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotEmpty(t, s) {
		return
	}

	serverExists := false
	for _, v := range s.Servers {
		if v.Identifier == c.serverID {
			serverExists = true
		}
	}
	if !assert.True(t, serverExists, "server does not exist in API") {
		return
	}
}

func (c *serverTestCtx) testSecondUserCannotSee(t *testing.T) {
	sess, err := createSession(c.db, loginDifferentServerUser)
	if !assert.NoError(t, err) {
		return
	}
	response := CallAPI("GET", "/api/servers", nil, sess)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var s *models.ServerSearchResponse
	err = json.NewDecoder(response.Body).Decode(&s)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Empty(t, s.Servers) {
		return
	}
}

func (c *serverTestCtx) testAdminUpdate(t *testing.T) {
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/definition", EditServerData, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	var server *models.Server
	err := c.db.Model(&server).Where(&models.Server{Identifier: c.serverID}).Find(&server).Error
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, EditServerNewName, server.Name) {
		return
	}
	if !assert.Equal(t, EditServerNewIP, server.IP) {
		return
	}
	if !assert.Equal(t, EditServerNewPort, server.Port) {
		return
	}
}

func (c *serverTestCtx) testAdminDataUpdate(t *testing.T) {
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/data", NewVariableChanges, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	var server *models.Server
	err := c.db.Model(&server).Where(&models.Server{Identifier: c.serverID}).Find(&server).Error
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, NewVariableChangeIP, server.IP) {
		return
	}
	if !assert.Equal(t, NewVariableChangePort, server.Port) {
		return
	}
}

func (c *serverTestCtx) testAddSubUser(t *testing.T) {
	var data = []byte(`{"scopes": ["server.view", "server.data.view"]}`)
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/user/"+loginNoLoginUser.Email, data, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
}

func (c *serverTestCtx) testGetSubUsers(t *testing.T) {
	c.assertSubUserScopes(t, loginNoLoginUser.Email, []*scopes.Scope{
		scopes.ScopeServerView, scopes.ScopeServerViewData,
	})
}

func (c *serverTestCtx) testGrantUserPermissions(t *testing.T) {
	var data = []byte(`{"scopes": ["server.view", "server.data.view", "server.start", "server.users.view", "server.users.edit"]}`)
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/user/"+loginNoLoginUser.Email, data, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	c.assertSubUserScopes(t, loginNoLoginUser.Email, []*scopes.Scope{
		scopes.ScopeServerView, scopes.ScopeServerViewData, scopes.ScopeServerStart, scopes.ScopeServerUserView, scopes.ScopeServerUserEdit,
	})
}

func (c *serverTestCtx) testSubuserGrantUserPermissions(t *testing.T) {
	// first get a session for the fake user
	testSession, err := createSession(c.db, loginNoLoginUser)
	if !assert.NoError(t, err) {
		return
	}

	var data = []byte(`{"scopes": ["server.view", "server.start", "server.stop"]}`)
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/user/"+loginNoAdminWithServersUser.Email, data, testSession)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	c.assertSubUserScopes(t, loginNoAdminWithServersUser.Email, []*scopes.Scope{
		scopes.ScopeServerView, scopes.ScopeServerStart,
	})
}

// assertSubUserScopes lists the server's users as admin and checks the scopes of the given email
func (c *serverTestCtx) assertSubUserScopes(t *testing.T, email string, expectedScopes []*scopes.Scope) {
	response := CallAPIRaw("GET", "/api/servers/"+c.serverID+"/user", nil, c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}
	var perms []*models.UserPermissionsView
	err := json.NewDecoder(response.Body).Decode(&perms)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotEmpty(t, perms) {
		return
	}
	found := false
	for _, v := range perms {
		if v.Email == email {
			if !assert.Equal(t, expectedScopes, v.Scopes) {
				return
			}
			found = true
		}
	}

	if !found {
		assert.Fail(t, "Failed to locate user")
	}
}

func (c *serverTestCtx) testUpdateVariable(t *testing.T) {
	motd := "This is a changed MOTD"
	var variables = []byte(`{"motd": "` + motd + `" }`)
	response := CallAPIRaw("POST", "/api/servers/"+c.serverID+"/data", variables, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	response = CallAPI("GET", "/api/servers/"+c.serverID+"/data", variables, c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var res map[string]map[string]skypanel.Variable
	err := json.NewDecoder(response.Body).Decode(&res)
	if !assert.NoError(t, err) {
		return
	}
	data := res["data"]
	if !assert.Len(t, data, 1) {
		return
	}

	memVar := data["motd"]
	assert.Equal(t, motd, memVar.Value)
}

func (c *serverTestCtx) testGetStatsSecondUserFail(t *testing.T) {
	sess, err := createSession(c.db, loginDifferentServerUser)
	if !assert.NoError(t, err) {
		return
	}
	response := CallAPI("GET", "/api/servers/"+c.serverID+"/stats", nil, sess)
	assert.Equal(t, http.StatusForbidden, response.Code)
}

func (c *serverTestCtx) testStopServer(t *testing.T) {
	response := CallAPI("POST", "/api/servers/"+c.serverID+"/stop", nil, c.session)
	if !assert.Equal(t, http.StatusAccepted, response.Code) {
		return
	}

	// now we wait for the install to finish
	timeout := 60
	counter := 0
	for counter < timeout {
		time.Sleep(time.Second)
		response = CallAPI("GET", "/api/servers/"+c.serverID+"/status", nil, c.session)
		assert.Equal(t, http.StatusOK, response.Code)
		var msg skypanel.ServerRunning
		err := json.NewDecoder(response.Body).Decode(&msg)
		if !assert.NoError(t, err) {
			return
		}

		if msg.Running {
			counter++
		} else {
			break
		}
	}
	if counter >= timeout {
		assert.Fail(t, "Server took too long to stop, assuming test failed")
	}
}

func (c *serverTestCtx) testCompression(t *testing.T) {
	dir := filepath.Join(c.serverDir, "testarchive")
	err := os.Mkdir(dir, 0755)
	if !assert.NoError(t, err) {
		return
	}

	fileLocation := filepath.Join(dir, "file.img")
	tmpFile, err := os.Create(fileLocation)
	if !assert.NoError(t, err) {
		return
	}

	hasher := sha256.New()
	w := io.MultiWriter(tmpFile, hasher)

	_, err = io.CopyN(w, rand.Reader, 1024*1024)
	if !assert.NoError(t, err) {
		return
	}

	_ = tmpFile.Close()
	expectedHash := hasher.Sum(nil)

	// test other functionality
	t.Run("Archive", func(t *testing.T) {
		c.testArchive(t, dir)
	})

	t.Run("Extract", func(t *testing.T) {
		c.testExtract(t, fileLocation, expectedHash)
	})
}

func (c *serverTestCtx) testArchive(t *testing.T, dir string) {
	response := CallAPI("POST", "/api/servers/"+c.serverID+"/archive/archive.zip", []string{"testarchive"}, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	_ = os.RemoveAll(dir)
}

func (c *serverTestCtx) testExtract(t *testing.T, fileLocation string, expectedHash []byte) {
	response := CallAPI("POST", "/api/servers/"+c.serverID+"/extract/archive.zip", nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	if !assert.FileExists(t, fileLocation) {
		return
	}

	f, err := os.Open(fileLocation)
	if !assert.NoError(t, err) {
		return
	}
	defer utils.Close(f)
	h := sha256.New()
	_, err = io.Copy(h, f)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, expectedHash, h.Sum(nil), "File hashes do not match") {
		return
	}
}

// testSFTPLogin checks that the user can log into the server over sftp and list its root
func (c *serverTestCtx) testSFTPLogin(t *testing.T, email, password string) {
	sshConfig := &ssh.ClientConfig{
		User: email + "#" + c.serverID,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.node.PrivateHost, c.node.SFTPPort), sshConfig)
	defer utils.Close(client)
	if !assert.NoError(t, err) {
		return
	}
	sftpClient, err := sftp.NewClient(client)
	if !assert.NoError(t, err) {
		return
	}
	files, err := sftpClient.ReadDir("/")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotEmpty(t, files) {
		return
	}
}

func (c *serverTestCtx) testSFTPNonUser(t *testing.T) {
	sshConfig := &ssh.ClientConfig{
		User: loginDifferentServerUser.Email + "#" + c.serverID,
		Auth: []ssh.AuthMethod{
			ssh.Password(loginDifferentServerUserPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.node.PrivateHost, c.node.SFTPPort), sshConfig)
	defer utils.Close(client)
	if !assert.Error(t, err) {
		return
	}
}

func (c *serverTestCtx) testCreateTask(t *testing.T) {
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/tasks/"+c.taskID, TaskDefinition, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	assert.FileExists(t, filepath.Join(config.ServersFolder.Value(), c.serverID+".cron"))
}

func (c *serverTestCtx) testGetTasks(t *testing.T) {
	response := CallAPIRaw("GET", "/api/servers/"+c.serverID+"/tasks", nil, c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var res skypanel.ServerTasks
	err := json.NewDecoder(response.Body).Decode(&res)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotEmpty(t, res.Tasks) {
		return
	}
}

func (c *serverTestCtx) testGetTask(t *testing.T) {
	response := CallAPIRaw("GET", "/api/servers/"+c.serverID+"/tasks/"+c.taskID, nil, c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var res skypanel.ServerTask
	err := json.NewDecoder(response.Body).Decode(&res)
	if !assert.NoError(t, err) {
		return
	}
	assert.NotEmpty(t, res.Operations)
}

func (c *serverTestCtx) testRunTask(t *testing.T) {
	eulaFile := filepath.Join(c.serverDir, "eula.txt")
	_ = os.Remove(eulaFile)

	response := CallAPIRaw("POST", "/api/servers/"+c.serverID+"/tasks/"+c.taskID+"/run", nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	time.Sleep(5 * time.Second)

	assert.FileExists(t, eulaFile)
}

func (c *serverTestCtx) testEditTask(t *testing.T) {
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/tasks/"+c.taskID, TaskDefinition, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	if !assert.Len(t, servers.GetFromCache(c.serverID).Scheduler.GetTasks(), 1) {
		return
	}
	e := servers.GetFromCache(c.serverID).Scheduler.GetExecutor()
	if !assert.Len(t, e.Jobs(), 1) {
		return
	}
}

func (c *serverTestCtx) testDeleteTask(t *testing.T) {
	response := CallAPIRaw("DELETE", "/api/servers/"+c.serverID+"/tasks/"+c.taskID, nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	response = CallAPIRaw("GET", "/api/servers/"+c.serverID+"/tasks", nil, c.session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}

	var res skypanel.ServerTasks
	err := json.NewDecoder(response.Body).Decode(&res)
	if !assert.NoError(t, err) {
		return
	}
	assert.Empty(t, res.Tasks)
}

func (c *serverTestCtx) testFileManager(t *testing.T) {
	var fileName = "test-file-to-make"
	var folderName = "test-folder-creation"
	var fileContents = []byte("this is a test file")

	t.Run("CreateFolder", func(t *testing.T) {
		c.testCreateFolder(t, folderName)
	})

	t.Run("DeleteFolder", func(t *testing.T) {
		c.testDeleteFolder(t, folderName)
	})

	t.Run("CreateFile", func(t *testing.T) {
		c.testCreateFile(t, fileName, fileContents)
	})

	t.Run("DeleteFile", func(t *testing.T) {
		c.testDeleteFile(t, fileName)
	})

	t.Run("DeleteFileWithURIEncoding", c.testDeleteFileWithURIEncoding)
}

func (c *serverTestCtx) testCreateFolder(t *testing.T, folderName string) {
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/file/"+folderName+"?folder=true", nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	if !assert.DirExists(t, filepath.Join(c.serverDir, folderName)) {
		return
	}
}

func (c *serverTestCtx) testDeleteFolder(t *testing.T, folderName string) {
	response := CallAPIRaw("DELETE", "/api/servers/"+c.serverID+"/file/"+folderName, nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	if !assert.NoDirExists(t, filepath.Join(c.serverDir, folderName)) {
		return
	}
}

func (c *serverTestCtx) testCreateFile(t *testing.T, fileName string, fileContents []byte) {
	response := CallAPIRaw("PUT", "/api/servers/"+c.serverID+"/file/"+fileName, fileContents, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}
	if !assert.FileExists(t, filepath.Join(c.serverDir, fileName)) {
		return
	}
}

func (c *serverTestCtx) testDeleteFile(t *testing.T, fileName string) {
	response := CallAPIRaw("DELETE", "/api/servers/"+c.serverID+"/file/"+fileName, nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	if !assert.NoFileExists(t, filepath.Join(c.serverDir, fileName)) {
		return
	}
}

func (c *serverTestCtx) testDeleteFileWithURIEncoding(t *testing.T) {
	filename := "file.delete.test?id=12345"

	fileLocation := filepath.Join(c.serverDir, filename)
	tmpFile, err := os.Create(fileLocation)
	if !assert.NoError(t, err) {
		return
	}
	utils.Close(tmpFile)

	u := url.QueryEscape(filename)

	response := CallAPIRaw("DELETE", "/api/servers/"+c.serverID+"/file/"+u, nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	_, err = os.Stat(fileLocation)
	if !assert.ErrorIs(t, err, os.ErrNotExist) {
		return
	}
}

func (c *serverTestCtx) testDeleteServer(t *testing.T) {
	response := CallAPIRaw("DELETE", "/api/servers/"+c.serverID, nil, c.session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	// ensure was actually removed
	if !assert.NoDirExists(t, filepath.Join(config.ServersFolder.Value(), c.serverID)) {
		return
	}

	var count int64
	err := c.db.Model(&models.Server{}).Where(&models.Server{Identifier: c.serverID}).Count(&count).Error
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int64(0), count)
}

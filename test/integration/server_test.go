package tests

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/oauth2"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/runtime"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	pufferSftp "github.com/SkyPanel/SkyPanel/v3/internal/sftp"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
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
		Node     *domain.Node
	}

	RemoteNode.PublicPort = domain.LocalNode.PublicPort
	RemoteNode.PrivatePort = domain.LocalNode.PrivatePort
	RemoteNode.SFTPPort = domain.LocalNode.SFTPPort

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
				Node:     domain.LocalNode,
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
			pufferSftp.SetAuthorization(test.SFTPAuth)
			_ = config.ClientSecret.Set(test.Node.Secret, false)
			var serverID = "testserver-" + strings.ToLower(name)
			serverDir := filepath.Join(config.ServersFolder.Value(), serverID)
			t.Run("CreateServer", func(t *testing.T) {
				data := CreateServerData
				data = strings.Replace(data, "{{{INSERTNODEID}}}", fmt.Sprintf("%d", test.Node.ID), 1)
				response := CallAPIRaw("PUT", "/api/servers/"+serverID, []byte(data), session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					t.Log(response.Body.String())
					return
				}

				var count int64
				err := db.Model(&domain.Server{}).Where(&domain.Server{Identifier: serverID}).Count(&count).Error
				if !assert.NoError(t, err) {
					return
				}

				if !assert.Equal(t, int64(1), count) {
					t.Logf("Server count is %d, expected 1", count)
					return
				}

				if !assert.DirExists(t, filepath.Join(config.ServersFolder.Value(), serverID)) {
					return
				}
			})

			t.Run("EnsureServerListContains1", func(t *testing.T) {
				response := CallAPI("GET", "/api/servers", nil, session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}

				var s *domain.ServerSearchResponse
				err := json.NewDecoder(response.Body).Decode(&s)
				if !assert.NoError(t, err) {
					return
				}

				if !assert.NotEmpty(t, s) {
					return
				}

				serverExists := false
				for _, v := range s.Servers {
					if v.Identifier == serverID {
						serverExists = true
					}
				}
				if !assert.True(t, serverExists, "server does not exist in API") {
					return
				}
			})

			t.Run("EnsureSecondUserCannotSee", func(t *testing.T) {
				sess, err := createSession(db, loginDifferentServerUser)
				if !assert.NoError(t, err) {
					return
				}
				response := CallAPI("GET", "/api/servers", nil, sess)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}

				var s *domain.ServerSearchResponse
				err = json.NewDecoder(response.Body).Decode(&s)
				if !assert.NoError(t, err) {
					return
				}

				if !assert.Empty(t, s.Servers) {
					return
				}
			})

			t.Run("AdminUpdate", func(t *testing.T) {
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/definition", EditServerData, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				var server *domain.Server
				err := db.Model(&server).Where(&domain.Server{Identifier: serverID}).Find(&server).Error
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

			})

			t.Run("AdminDataUpdate", func(t *testing.T) {
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/data", NewVariableChanges, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				var server *domain.Server
				err := db.Model(&server).Where(&domain.Server{Identifier: serverID}).Find(&server).Error
				if !assert.NoError(t, err) {
					return
				}

				if !assert.Equal(t, NewVariableChangeIP, server.IP) {
					return
				}
				if !assert.Equal(t, NewVariableChangePort, server.Port) {
					return
				}

			})

			if t.Failed() {
				return
			}

			// previous test is a block,so we can now open up a websocket connection and start playing with it
			// the test here is... do we get all 3 types of messages
			statsReceived := false
			messageReceived := false
			statusReceived := false

			addr := fmt.Sprintf("%s:%d", domain.LocalNode.PrivateHost, domain.LocalNode.PrivatePort)

			u := fmt.Sprintf("ws://%s/api/servers/%s/socket", addr, serverID)

			header := http.Header{}
			header.Set("Authorization", "Bearer "+session)

			c, _, webErr := websocket.DefaultDialer.Dial(u, header)
			if !assert.NoError(t, webErr) {
				return
			}
			var listening atomic.Bool
			listening.Store(true)
			defer utils.Close(c)

			go func(conn *websocket.Conn) {
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
					var msg map[string]interface{}
					err = json.NewDecoder(bytes.NewReader(data)).Decode(&msg)
					if err != nil {
						fmt.Printf("Failed to decode message: %s\n", err.Error())
						continue
					}

					msgData := msg["data"]

					switch msg["type"].(string) {
					case skypanel.MessageTypeLog:
						var ms skypanel.ServerLogs
						err = utils.UnmarshalTo(msgData, &ms)
						if err != nil {
							fmt.Printf("Failed to decode message: %s\n", err.Error())
							continue
						}

						if config.ConsoleForward.Value() {
							fmt.Printf("[WEBSOCKET] %s\n", ms.Logs)
						}

						messageReceived = true
					case skypanel.MessageTypeStatus:
						statusReceived = true
					case skypanel.MessageTypeStats:
						statsReceived = true
					default:
						fmt.Printf("unknown message type: %s\n", msg["type"])
						continue
					}
				}
			}(c)

			t.Run("AddSubUser", func(t *testing.T) {
				var data = []byte(`{"scopes": ["server.view", "server.data.view"]}`)
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/user/"+loginNoLoginUser.Email, data, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}
			})

			t.Run("GetSubUsers", func(t *testing.T) {
				response := CallAPIRaw("GET", "/api/servers/"+serverID+"/user", nil, session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}
				var data []*domain.UserPermissionsView
				err := json.NewDecoder(response.Body).Decode(&data)
				if !assert.NoError(t, err) {
					return
				}

				if assert.NotEmpty(t, data) {
					return
				}
				found := false
				for _, v := range data {
					if v.Email == loginNoLoginUser.Email {
						var expectedScopes = []*scopes.Scope{
							scopes.ScopeServerView, scopes.ScopeServerViewData,
						}
						if !assert.Equal(t, expectedScopes, v.Scopes) {
							return
						}
						found = true
					}
				}

				if !found {
					assert.Fail(t, "Failed to locate user")
				}
			})

			t.Run("GrantUserPermissions", func(t *testing.T) {
				var data = []byte(`{"scopes": ["server.view", "server.data.view", "server.start", "server.users.view", "server.users.edit"]}`)
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/user/"+loginNoLoginUser.Email, data, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				response = CallAPIRaw("GET", "/api/servers/"+serverID+"/user", nil, session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}
				var perms []*domain.UserPermissionsView
				err := json.NewDecoder(response.Body).Decode(&perms)
				if !assert.NoError(t, err) {
					return
				}

				if assert.NotEmpty(t, perms) {
					return
				}
				found := false
				for _, v := range perms {
					if v.Email == loginNoLoginUser.Email {
						var expectedScopes = []*scopes.Scope{
							scopes.ScopeServerView, scopes.ScopeServerViewData, scopes.ScopeServerStart, scopes.ScopeServerUserView, scopes.ScopeServerUserEdit,
						}
						if !assert.Equal(t, expectedScopes, v.Scopes) {
							return
						}
						found = true
					}
				}

				if !found {
					assert.Fail(t, "Failed to locate user")
				}
			})

			t.Run("SubuserGrantUserPermissions", func(t *testing.T) {
				// first get a session for the fake user
				testSession, err := createSession(db, loginNoLoginUser)
				if !assert.NoError(t, err) {
					return
				}

				var data = []byte(`{"scopes": ["server.view", "server.start", "server.stop"]}`)
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/user/"+loginNoAdminWithServersUser.Email, data, testSession)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				response = CallAPIRaw("GET", "/api/servers/"+serverID+"/user", nil, session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}
				var perms []*domain.UserPermissionsView
				err = json.NewDecoder(response.Body).Decode(&perms)
				if !assert.NoError(t, err) {
					return
				}

				if assert.NotEmpty(t, perms) {
					return
				}
				found := false
				for _, v := range perms {
					if v.Email == loginNoAdminWithServersUser.Email {
						var expectedScopes = []*scopes.Scope{
							scopes.ScopeServerView, scopes.ScopeServerStart,
						}
						if !assert.Equal(t, expectedScopes, v.Scopes) {
							return
						}
						found = true
					}
				}

				if !found {
					assert.Fail(t, "Failed to locate user")
				}
			})

			t.Run("UpdateVariable", func(t *testing.T) {
				motd := "This is a changed MOTD"
				var variables = []byte(`{"motd": "` + motd + `" }`)
				response := CallAPIRaw("POST", "/api/servers/"+serverID+"/data", variables, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				response = CallAPI("GET", "/api/servers/"+serverID+"/data", variables, session)
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
			})

			t.Run("GetStats", func(t *testing.T) {
				response := CallAPI("GET", "/api/servers/"+serverID+"/stats", nil, session)
				assert.Equal(t, http.StatusOK, response.Code)
			})

			t.Run("GetStatsSecondUserFail", func(t *testing.T) {
				sess, err := createSession(db, loginDifferentServerUser)
				if !assert.NoError(t, err) {
					return
				}
				response := CallAPI("GET", "/api/servers/"+serverID+"/stats", nil, sess)
				assert.Equal(t, http.StatusForbidden, response.Code)
			})

			t.Run("SendStatsForServers", func(_ *testing.T) {
				servers.SendStatsForServers()
			})

			t.Run("GetEmptyFiles", func(t *testing.T) {
				response := CallAPI("GET", "/api/servers/"+serverID+"/file/", nil, session)
				assert.Equal(t, http.StatusOK, response.Code)
			})

			t.Run("InstallServer", func(t *testing.T) {
				t.Skip("Skipping install test in unit tests to avoid long downloads")
			})

			t.Run("StartServer", func(t *testing.T) {
				response := CallAPI("POST", "/api/servers/"+serverID+"/start", nil, session)
				assert.Equal(t, http.StatusAccepted, response.Code)

				time.Sleep(1000 * time.Millisecond)
			})

			t.Run("StopServer", func(t *testing.T) {
				response := CallAPI("POST", "/api/servers/"+serverID+"/stop", nil, session)
				if !assert.Equal(t, http.StatusAccepted, response.Code) {
					return
				}

				// now we wait for the install to finish
				timeout := 60
				counter := 0
				for counter < timeout {
					time.Sleep(time.Second)
					response = CallAPI("GET", "/api/servers/"+serverID+"/status", nil, session)
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
			})

			listening.Store(false)
			_ = c.Close()

			// create a fake file that we can use to both
			t.Run("Compression", func(t *testing.T) {
				dir := filepath.Join(serverDir, "testarchive")
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
					response := CallAPI("POST", "/api/servers/"+serverID+"/archive/archive.zip", []string{"testarchive"}, session)
					if !assert.Equal(t, http.StatusNoContent, response.Code) {
						return
					}
					_ = os.RemoveAll(dir)
				})

				t.Run("Extract", func(t *testing.T) {
					response := CallAPI("POST", "/api/servers/"+serverID+"/extract/archive.zip", nil, session)
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
				})
			})

			t.Run("SFTP", func(t *testing.T) {
				t.Run("Admin", func(t *testing.T) {
					sshConfig := &ssh.ClientConfig{
						User: loginAdminUser.Email + "#" + serverID,
						Auth: []ssh.AuthMethod{
							ssh.Password(loginAdminUserPassword),
						},
						HostKeyCallback: ssh.InsecureIgnoreHostKey(),
					}

					client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", test.Node.PrivateHost, test.Node.SFTPPort), sshConfig)
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
				})

				t.Run("User", func(t *testing.T) {
					sshConfig := &ssh.ClientConfig{
						User: loginSftpUser.Email + "#" + serverID,
						Auth: []ssh.AuthMethod{
							ssh.Password(loginSftpUserPassword),
						},
						HostKeyCallback: ssh.InsecureIgnoreHostKey(),
					}
					client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", test.Node.PrivateHost, test.Node.SFTPPort), sshConfig)
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
				})

				t.Run("NonUser", func(t *testing.T) {
					sshConfig := &ssh.ClientConfig{
						User: loginDifferentServerUser.Email + "#" + serverID,
						Auth: []ssh.AuthMethod{
							ssh.Password(loginDifferentServerUserPassword),
						},
						HostKeyCallback: ssh.InsecureIgnoreHostKey(),
					}
					client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", test.Node.PrivateHost, test.Node.SFTPPort), sshConfig)
					defer utils.Close(client)
					if !assert.Error(t, err) {
						return
					}
				})
			})

			var taskID = "testtask"
			t.Run("CreateTask", func(t *testing.T) {
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/tasks/"+taskID, TaskDefinition, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}
				assert.FileExists(t, filepath.Join(config.ServersFolder.Value(), serverID+".cron"))
			})

			t.Run("GetTasks", func(t *testing.T) {
				response := CallAPIRaw("GET", "/api/servers/"+serverID+"/tasks", nil, session)
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
			})

			t.Run("GetTask", func(t *testing.T) {
				response := CallAPIRaw("GET", "/api/servers/"+serverID+"/tasks/"+taskID, nil, session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}

				var res skypanel.ServerTask
				err := json.NewDecoder(response.Body).Decode(&res)
				if !assert.NoError(t, err) {
					return
				}
				assert.NotEmpty(t, res.Operations)
			})

			t.Run("RunTask", func(t *testing.T) {
				eulaFile := filepath.Join(serverDir, "eula.txt")
				_ = os.Remove(eulaFile)

				response := CallAPIRaw("POST", "/api/servers/"+serverID+"/tasks/"+taskID+"/run", nil, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				time.Sleep(5 * time.Second)

				assert.FileExists(t, eulaFile)
			})

			t.Run("EditTask", func(t *testing.T) {
				response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/tasks/"+taskID, TaskDefinition, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}
				if !assert.Len(t, runtime.GetFromCache(serverID).Scheduler.GetTasks(), 1) {
					return
				}
				e := runtime.GetFromCache(serverID).Scheduler.GetExecutor()
				if !assert.Len(t, e.Jobs(), 1) {
					return
				}
			})

			t.Run("DeleteTask", func(t *testing.T) {
				response := CallAPIRaw("DELETE", "/api/servers/"+serverID+"/tasks/"+taskID, nil, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				response = CallAPIRaw("GET", "/api/servers/"+serverID+"/tasks", nil, session)
				if !assert.Equal(t, http.StatusOK, response.Code) {
					return
				}

				var res skypanel.ServerTasks
				err := json.NewDecoder(response.Body).Decode(&res)
				if !assert.NoError(t, err) {
					return
				}
				assert.Empty(t, res.Tasks)
			})

			t.Run("FileManager", func(t *testing.T) {
				var fileName = "test-file-to-make"
				var folderName = "test-folder-creation"
				var fileContents = []byte("this is a test file")

				t.Run("CreateFolder", func(t *testing.T) {
					response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/file/"+folderName+"?folder=true", nil, session)
					if !assert.Equal(t, http.StatusNoContent, response.Code) {
						return
					}
					if !assert.DirExists(t, filepath.Join(serverDir, folderName)) {
						return
					}
				})

				t.Run("DeleteFolder", func(t *testing.T) {
					response := CallAPIRaw("DELETE", "/api/servers/"+serverID+"/file/"+folderName, nil, session)
					if !assert.Equal(t, http.StatusNoContent, response.Code) {
						return
					}
					if !assert.NoDirExists(t, filepath.Join(serverDir, folderName)) {
						return
					}
				})

				t.Run("CreateFile", func(t *testing.T) {
					response := CallAPIRaw("PUT", "/api/servers/"+serverID+"/file/"+fileName, fileContents, session)
					if !assert.Equal(t, http.StatusNoContent, response.Code) {
						return
					}
					if !assert.FileExists(t, filepath.Join(serverDir, fileName)) {
						return
					}
				})

				t.Run("DeleteFile", func(t *testing.T) {
					response := CallAPIRaw("DELETE", "/api/servers/"+serverID+"/file/"+fileName, nil, session)
					if !assert.Equal(t, http.StatusNoContent, response.Code) {
						return
					}

					if !assert.NoFileExists(t, filepath.Join(serverDir, fileName)) {
						return
					}
				})

				t.Run("DeleteFileWithURIEncoding", func(t *testing.T) {
					filename := "file.delete.test?id=12345"

					fileLocation := filepath.Join(serverDir, filename)
					tmpFile, err := os.Create(fileLocation)
					if !assert.NoError(t, err) {
						return
					}
					utils.Close(tmpFile)

					u := url.QueryEscape(filename)

					response := CallAPIRaw("DELETE", "/api/servers/"+serverID+"/file/"+u, nil, session)
					if !assert.Equal(t, http.StatusNoContent, response.Code) {
						return
					}

					_, err = os.Stat(fileLocation)
					if !assert.ErrorIs(t, err, os.ErrNotExist) {
						return
					}
				})
			})

			t.Run("Delete", func(t *testing.T) {
				response := CallAPIRaw("DELETE", "/api/servers/"+serverID, nil, session)
				if !assert.Equal(t, http.StatusNoContent, response.Code) {
					return
				}

				// ensure was actually removed
				if !assert.NoDirExists(t, filepath.Join(config.ServersFolder.Value(), serverID)) {
					return
				}

				var count int64
				err := db.Model(&domain.Server{}).Where(&domain.Server{Identifier: serverID}).Count(&count).Error
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, int64(0), count)
			})

			t.Run("WebSocketReceivedAll", func(t *testing.T) {
				assert.True(t, statsReceived, "Stats were not received")
				assert.True(t, statusReceived, "Status was not received")
				assert.True(t, messageReceived, "Console messages were not received")
			})
		})
	}
}

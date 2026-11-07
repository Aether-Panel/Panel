package sftp

import (
	"crypto/x509"
	"encoding/pem"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/oauth2"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"time"
)

// accessRecheckInterval is how long an open connection may outlive the user's access: the
// handshake is the only other check, and deleting the user or taking away the login or the
// server's SFTP scope must also end the connections they already have
const accessRecheckInterval = 30 * time.Second

var sftpServer net.Listener

var auth skypanel.SFTPAuthorization

func Run() {
	err := runServer()
	if err != nil {
		logging.Error.Printf("Error starting SFTP server: %s", err)
	}
}

func SetAuthorization(service skypanel.SFTPAuthorization) {
	auth = service
}

func Stop() {
	if sftpServer != nil {
		_ = sftpServer.Close()
	}
}

func runServer() error {
	if auth == nil {
		auth = &oauth2.WebSSHAuthorization{}
	}

	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return auth.Validate(c.User(), string(pass))
		},
	}
	if keys, ok := auth.(skypanel.SFTPKeyAuthorization); ok {
		serverConfig.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return keys.ValidateKey(c.User(), key)
		}
	}

	serverKeyFile := config.SftpKey.Value()

	_, e := os.Stat(serverKeyFile)

	if e != nil && os.IsNotExist(e) {
		logging.Info.Printf("Generating new key")
		var key ed25519.PrivateKey
		_, key, e = ed25519.GenerateKey(nil)
		if e != nil {
			return e
		}

		data, e := x509.MarshalPKCS8PrivateKey(key)
		block := &pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: data,
		}
		if e != nil {
			return e
		}

		e = os.WriteFile(serverKeyFile, pem.EncodeToMemory(block), 0700)
		if e != nil {
			return e
		}
	} else if e != nil {
		return e
	}

	logging.Info.Printf("Loading existing key")
	var data []byte
	data, e = os.ReadFile(serverKeyFile)
	if e != nil {
		return e
	}

	hkey, e := ssh.ParsePrivateKey(data)

	if e != nil {
		return e
	}

	serverConfig.AddHostKey(hkey)

	bind := config.SftpHost.Value()

	sftpServer, e = net.Listen("tcp", bind)
	if e != nil {
		return e
	}
	logging.Info.Printf("Started SFTP Server on %s", bind)

	go func() {
		for {
			conn, _ := sftpServer.Accept()
			if conn != nil {
				go HandleConn(conn, serverConfig)
			}
		}
	}()

	return nil
}

func HandleConn(conn net.Conn, serverConfig *ssh.ServerConfig) {
	defer utils.Close(conn)
	defer skypanel.Recover()
	logging.Info.Printf("SFTP connection from %s", conn.RemoteAddr().String())
	e := handleConn(conn, serverConfig)
	if e != nil {
		if e.Error() != "EOF" {
			logging.Error.Printf("sftpd connection error: %s", e)
		}
	}
}
func handleConn(conn net.Conn, serverConfig *ssh.ServerConfig) error {
	sc, chans, reqs, e := ssh.NewServerConn(conn, serverConfig)
	defer utils.Close(sc)
	if e != nil {
		return e
	}

	// The incoming Request channel must be serviced.
	go PrintDiscardRequests(reqs)

	if checker, ok := auth.(skypanel.SFTPRecheck); ok {
		done := make(chan struct{})
		defer close(done)
		go watchAccess(done, checker, sc.Permissions, accessRecheckInterval, sc.Close, sc.User())
	}

	// Service the incoming channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		if newChannel.ChannelType() != "session" {
			err := newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			if err != nil {
				return err
			}
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return err
		}

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go replySubsystemRequests(requests)

		serverID := sc.Permissions.Extensions["server_id"]
		server := servers.GetFromCache(serverID)
		if server == nil {
			// this daemon can't handle this request...
			return nil
		}

		fs := CreateRequestPrefix(sc.Conn.RemoteAddr(), server)
		s := sftp.NewRequestServer(channel, fs)

		if err = s.Serve(); err != nil {
			return err
		}
	}
	return nil
}

// watchAccess asks the authorization every interval whether the connection may stay open and
// closes it on a no. An error is no answer (the database or the panel is down): the connection
// stays, and the error is logged once until a check gets through again.
func watchAccess(done <-chan struct{}, checker skypanel.SFTPRecheck, perms *ssh.Permissions, interval time.Duration, closeConn func() error, user string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	failing := false
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}

		allowed, err := checker.Recheck(perms)
		if err != nil {
			if !failing {
				logging.Error.Printf("Cannot check whether SFTP user %s still has access, the connection stays open: %s", user, err)
			}
			failing = true
			continue
		}
		failing = false
		if !allowed {
			logging.Info.Printf("SFTP user %s no longer has access, closing the connection", user)
			_ = closeConn()
			return
		}
	}
}

// replySubsystemRequests accepts only the sftp subsystem request and refuses everything else
func replySubsystemRequests(in <-chan *ssh.Request) {
	for req := range in {
		ok := false
		// the payload is a length-prefixed name sent by the client: check its length first,
		// a short one would otherwise panic and take the whole daemon down
		if req.Type == "subsystem" && len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
			ok = true
		}
		_ = req.Reply(ok, nil)
	}
}

func PrintDiscardRequests(in <-chan *ssh.Request) {
	for req := range in {
		if req.WantReply {
			_ = req.Reply(false, nil)
		}
	}
}

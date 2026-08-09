package sftp

import (
	"errors"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/feature/permission"
	"github.com/SkyPanel/SkyPanel/v3/internal/feature/user"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/ssh"
)

type SftpAuthRepo struct {
}

func (s *SftpAuthRepo) Validate(username, password string) (perms *ssh.Permissions, err error) {
	parts := strings.Split(username, "#")
	if len(parts) != 2 {
		return nil, errors.New("incorrect username or password")
	}

	email := parts[0]
	serverID := parts[1]

	db, err := database.GetConnection()
	if err != nil {
		return nil, skypanel.ErrDatabaseNotAvailable
	}

	us := &user.UserRepo{DB: db}
	u, err := us.GetByEmail(email)
	if u == nil || err != nil || !us.IsValidCredentials(u, password) {
		return nil, errors.New("incorrect username or password")
	}

	ps := &permission.PermissionRepo{DB: db}
	allowed, err := ps.HasPermission(u.ID, serverID, scopes.ScopeServerSftp)
	if err != nil || !allowed {
		return nil, errors.New("incorrect username or password")
	}

	perms = &ssh.Permissions{}
	perms.Extensions = make(map[string]string)
	perms.Extensions["server_id"] = serverID
	return perms, nil
}

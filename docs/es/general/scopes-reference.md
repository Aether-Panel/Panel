# Complete Scopes Reference

Complete reference of all **72** permission scopes in Aether Panel (verified against `internal/scopes/scopes.go`).

---

## Scope Categories

| Category | Count |
|----------|-------|
| **Server Scopes** | 49 |
| **Global Scopes** | 23 |

**Total: 72 scopes**

---

## Server Scopes (49)

### Server Lifecycle (9)
| Scope | Description |
|-------|-------------|
| `server.view` | View server details |
| `server.create` | Create new server (global scope) |
| `server.delete` | Delete server |
| `server.start` | Start server |
| `server.stop` | Stop server |
| `server.restart` | Restart server (requires start + stop) |
| `server.kill` | Force kill server |
| `server.install` | Run installation |
| `server.reload` | Reload configuration |

### Server Data (6)
| Scope | Description |
|-------|-------------|
| `server.data.view` | View server variables |
| `server.data.edit` | Edit user-editable variables |
| `server.data.edit.admin` | Edit admin-only variables (port, CPU, memory, disk) |
| `server.ports.edit` | Manage port list + primary selection |
| `server.definition.view` | View server definition JSON |
| `server.definition.edit` | Edit server definition JSON |

### Server Flags (2)
| Scope | Description |
|-------|-------------|
| `server.flags.view` | View auto-start/restart flags |
| `server.flags.edit` | Edit auto-start/restart flags |

### Console & Commands (2)
| Scope | Description |
|-------|-------------|
| `server.console` | View console output |
| `server.console.send` | Send commands to console |

### Files & SFTP (4)
| Scope | Description |
|-------|-------------|
| `server.files.view` | List/download files |
| `server.files.edit` | Upload/edit/delete/move files |
| `server.sftp` | SFTP access |
| `server.archive` | Create/extract archives |

### Backups (4)
| Scope | Description |
|-------|-------------|
| `server.backup.view` | View backup list |
| `server.backup.create` | Create backup |
| `server.backup.restore` | Restore backup |
| `server.backup.delete` | Delete backup |

### Statistics & Status (2)
| Scope | Description |
|-------|-------------|
| `server.stats` | View CPU/RAM/disk/network stats |
| `server.status` | View running/stopped status |

### Tasks / Scheduler (5)
| Scope | Description |
|-------|-------------|
| `server.tasks.view` | View scheduled tasks |
| `server.tasks.run` | Manually run task |
| `server.tasks.create` | Create task |
| `server.tasks.edit` | Edit task |
| `server.tasks.delete` | Delete task |

### Database (3)
| Scope | Description |
|-------|-------------|
| `server.database.view` | View databases |
| `server.database.create` | Create database |
| `server.database.delete` | Delete database |

### Users / Permissions (4)
| Scope | Description |
|-------|-------------|
| `server.users.view` | View server users |
| `server.users.edit` | Edit server user permissions |
| `server.users.create` | Add user to server |
| `server.users.delete` | Remove user from server |

### Plugins (2)
| Scope | Description |
|-------|-------------|
| `server.plugins.view` | View installed plugins |
| `server.plugins.edit` | Install/remove plugins |

### Transfers (4)
| Scope | Description |
|-------|-------------|
| `server.transfer.view` | View internal transfers |
| `server.transfer.manage` | Manage internal transfers |
| `server.extransfer.view` | View external transfers |
| `server.extransfer.manage` | Manage external transfers |

### Admin Scopes (19)
| Scope | Description |
|-------|-------------|
| `server.admin` | Full server admin (all below) |
| `server.admin.view` | View admin panel |
| `server.admin.config.view` | View admin config (ports, limits, metadata) |
| `server.admin.config.manage` | Manage admin config (ports, limits, metadata) |
| `server.admin.install.view` | View installation steps |
| `server.admin.install.manage` | Manage installation steps |
| `server.admin.transfer.view` | View transfer details |
| `server.admin.transfer.manage` | Manage transfers |
| `server.admin.assignments.view` | View user assignments |
| `server.admin.assignments.manage` | Manage user assignments |
| `server.admin.clients.view` | View OAuth2 clients |
| `server.admin.clients.edit` | Edit OAuth2 clients |
| `server.admin.clients.create` | Create OAuth2 clients |
| `server.admin.clients.delete` | Delete OAuth2 clients |
| `server.admin.env.view` | View environment config |
| `server.admin.env.manage` | Manage environment config |
| `server.admin.data.view` | View all server data |
| `server.admin.data.edit` | Edit all server data |
| `server.admin.backup.view` | View all backups |
| `server.admin.backup.manage` | Manage all backups |

### Clients / OAuth2 (4)
| Scope | Description |
|-------|-------------|
| `server.clients.view` | View OAuth2 clients |
| `server.clients.edit` | Edit OAuth2 clients |
| `server.clients.create` | Create OAuth2 clients |
| `server.clients.delete` | Delete OAuth2 clients |

---

## Global Scopes (23)

### System Administration (6)
| Scope | Description |
|-------|-------------|
| `admin` | Superadmin - all permissions |
| `panel` | Access panel UI |
| `login` | Authenticate (login) |
| `settings.edit` | Edit global panel settings |
| `oauth2.auth` | Validate OAuth2 credentials |
| `license` | Manage license |

### User Management (6)
| Scope | Description |
|-------|-------------|
| `users.info.search` | Search users |
| `users.info.view` | View user details |
| `users.info.edit` | Create/edit/delete users |
| `users.perms.view` | View user permissions |
| `users.perms.edit` | Edit user permissions |
| `self.edit` | Edit own profile |

### Node Management (4)
| Scope | Description |
|-------|-------------|
| `nodes.view` | View nodes |
| `nodes.create` | Create node |
| `nodes.edit` | Edit node |
| `nodes.delete` | Delete node |
| `nodes.deploy` | Get deployment data |

### Template Management (4)
| Scope | Description |
|-------|-------------|
| `templates.view` | View templates |
| `templates.local.edit` | Edit local templates (repo 0) |
| `templates.repo.create` | Add template repositories |
| `templates.repo.delete` | Delete template repositories |

### Database Hosts (1)
| Scope | Description |
|-------|-------------|
| `databasehosts.manage` | CRUD database hosts |

### Provisioning (2)
| Scope | Description |
|-------|-------------|
| `provision.manage` | Manage provision products |
| `provision.view` | View provision products |

### Uptime / Monitoring (1)
| Scope | Description |
|-------|-------------|
| `uptime.view` | View uptime statistics |

### Role Management (2)
| Scope | Description |
|-------|-------------|
| `roles.view` | View roles |
| `roles.edit` | Create/edit/delete roles |

---

## Scope Hierarchy

```
admin (superadmin)
├── Global scopes (23)
└── Server scopes via server.admin (49)
    ├── server.view
    ├── server.start/stop/restart/kill
    ├── server.data.edit.admin
    ├── server.definition.edit
    ├── server.flags.edit
    ├── server.files.edit
    ├── server.backup.*
    ├── server.tasks.*
    ├── server.database.*
    ├── server.users.*
    ├── server.plugins.*
    ├── server.transfer.*
    ├── server.extransfer.*
    ├── server.plugins.*
    ├── server.clients.*
    └── server.admin.* (all 19 admin sub-permissions)
```

### ForServer Flag

Most server scopes have `ForServer: true` meaning they require a specific `serverId` in the route:

```go
// Route definition
servers.GET("/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), handler)

// Middleware checks:
// 1. User has global scope
// 2. User has server-specific scope for that serverId
// 3. User has server.admin for that serverId
// 4. User has admin (global)
```

---

## Role Default Scopes

### Admin (Role ID: 1)
```json
["admin"]
```

### Usuario (Role ID: 2) - After Migration `20260821-fix-usuario-role-scopes`
```json
[
  "server.view",
  "server.definition.view",
  "server.definition.edit",
  "server.flags.view",
  "server.flags.edit",
  "server.data.view",
  "server.data.edit",
  "server.files.view",
  "server.files.edit",
  "server.plugins.view",
  "server.plugins.edit",
  "server.backup.view",
  "server.backup.create",
  "server.backup.restore",
  "server.backup.delete",
  "server.tasks.view",
  "server.tasks.run",
  "server.tasks.create",
  "server.tasks.edit",
  "server.tasks.delete",
  "server.database.view",
  "server.database.create",
  "server.database.delete",
  "server.users.view",
  "server.users.edit",
  "server.users.create",
  "server.users.delete",
  "server.stats",
  "server.status",
  "server.console",
  "server.console.send",
  "server.sftp",
  "server.archive",
  "server.flags.view",
  "server.flags.edit"
]
```

**Revoked from Usuario (v2.0.1):**
- `server.admin.config.view`
- `server.admin.config.manage`
- `server.data.edit.admin`
- `server.data.edit`

---

## API Endpoint → Scope Mapping

### Server Endpoints
| Endpoint | Scope |
|----------|-------|
| `GET /api/servers` | `server.view` (global) or auth |
| `GET /api/servers/:id` | `server.view` |
| `PUT /api/servers/:id` | `server.create` |
| `DELETE /api/servers/:id` | `server.delete` |
| `POST /api/servers/:id/start` | `server.start` |
| `POST /api/servers/:id/stop` | `server.stop` |
| `POST /api/servers/:id/restart` | `server.start` + `server.stop` |
| `POST /api/servers/:id/kill` | `server.kill` |
| `POST /api/servers/:id/install` | `server.install` |
| `POST /api/servers/:id/reload` | `server.reload` |
| `GET /api/servers/:id/data` | `server.data.view` |
| `POST /api/servers/:id/data` | `server.data.edit` |
| `PUT /api/servers/:id/data` | `server.data.edit.admin` |
| `PUT /api/servers/:id/port-settings` | `server.data.view` |
| `GET /api/servers/:id/definition` | `server.definition.view` |
| `PUT /api/servers/:id/definition` | `server.definition.edit` |
| `GET /api/servers/:id/flags` | `server.flags.view` |
| `POST /api/servers/:id/flags` | `server.flags.edit` |
| `POST /api/servers/:id/start` | `server.start` |
| `POST /api/servers/:id/stop` | `server.stop` |
| `POST /api/servers/:id/restart` | `server.start` + `server.stop` |
| `POST /api/servers/:id/kill` | `server.kill` |
| `POST /api/servers/:id/install` | `server.install` |
| `POST /api/servers/:id/reload` | `server.reload` |
| `GET /api/servers/:id/console` | `server.console` |
| `POST /api/servers/:id/console` | `server.console.send` |
| `GET /api/servers/:id/status` | `server.status` |
| `GET /api/servers/:id/stats` | `server.stats` |
| `GET /api/servers/:id/query` | `server.stats` |
| `GET /api/servers/:id/socket` | `server.view` |
| `GET /api/servers/:id/backup` | `server.backup.view` |
| `POST /api/servers/:id/backup/create` | `server.backup.create` |
| `POST /api/servers/:id/backup/restore/:id` | `server.backup.restore` |
| `DELETE /api/servers/:id/backup/:id` | `server.backup.delete` |
| `GET /api/servers/:id/tasks` | `server.tasks.view` |
| `PUT /api/servers/:id/tasks/:taskId` | `server.tasks.edit` |
| `POST /api/servers/:id/tasks/:taskId/run` | `server.tasks.run` |
| `GET /api/servers/:id/databases` | `server.database.view` |
| `POST /api/servers/:id/databases` | `server.data.edit` |
| `DELETE /api/servers/:id/databases/:id` | `server.data.edit` |
| `GET /api/servers/:id/user` | `server.users.view` |
| `PUT /api/servers/:id/user/:email` | `server.users.edit` |
| `GET /api/servers/:id/plugins` | `server.plugins.view` |
| `POST /api/servers/:id/plugins/:pluginId` | `server.plugins.edit` |

### Global Endpoints
| Endpoint | Scope |
|----------|-------|
| `GET /api/nodes` | `nodes.view` |
| `POST /api/nodes` | `nodes.create` |
| `GET /api/users` | `users.info.search` |
| `POST /api/users` | `users.info.edit` |
| `GET /api/settings` | `settings.edit` |
| `POST /api/settings` | `settings.edit` |
| `GET /api/templates` | `templates.view` |
| `POST /api/templates` | `templates.repo.create` |
| `GET /api/databasehosts` | `databasehosts.manage` |
| `POST /api/databasehosts` | `databasehosts.manage` |
| `GET /api/uptime` | `uptime.view` |
| `POST /api/v1/provision` | API Key with `provision` permission |
| `POST /api/extransfer/validate` | Public (no auth) |

---

## Scope Constants (`internal/scopes/scopes.go`)

All 72 scopes defined as constants:

```go
package scopes

const (
    // Global
    ScopeAdmin              = "admin"
    ScopeLogin              = "login"
    ScopePanel              = "panel"
    ScopeSettingsEdit       = "settings.edit"
    ScopeOAuth2Auth         = "oauth2.auth"
    ScopeLicense            = "license"
    
    // Server lifecycle
    ScopeServerView         = "server.view"
    ScopeServerCreate       = "server.create"
    ScopeServerDelete       = "server.delete"
    ScopeServerStart        = "server.start"
    ScopeServerStop         = "server.stop"
    ScopeServerRestart      = "server.restart"
    ScopeServerKill         = "server.kill"
    ScopeServerInstall      = "server.install"
    ScopeServerReload       = "server.reload"
    
    // Server data
    ScopeServerViewData     = "server.data.view"
    ScopeServerEditData     = "server.data.edit"
    ScopeServerEditDataAdmin = "server.data.edit.admin"
    ScopeServerPortsEdit    = "server.ports.edit"
    ScopeServerDefinitionView = "server.definition.view"
    ScopeServerDefinitionEdit = "server.definition.edit"
    
    // Server flags
    ScopeServerFlagsView    = "server.flags.view"
    ScopeServerFlagsEdit    = "server.flags.edit"
    
    // Console & commands
    ScopeServerConsole      = "server.console"
    ScopeServerConsoleSend  = "server.console.send"
    
    // Files & SFTP
    ScopeServerFilesView    = "server.files.view"
    ScopeServerFilesEdit    = "server.files.edit"
    ScopeServerSftp         = "server.sftp"
    ScopeServerArchive      = "server.archive"
    
    // Backups
    ScopeServerBackupView   = "server.backup.view"
    ScopeServerBackupCreate = "server.backup.create"
    ScopeServerBackupRestore = "server.backup.restore"
    ScopeServerBackupDelete = "server.backup.delete"
    
    // Statistics & Status
    ScopeServerStats        = "server.stats"
    ScopeServerStatus       = "server.status"
    
    // Tasks / Scheduler
    ScopeServerTasksView    = "server.tasks.view"
    ScopeServerTasksRun     = "server.tasks.run"
    ScopeServerTasksCreate  = "server.tasks.create"
    ScopeServerTasksEdit    = "server.tasks.edit"
    ScopeServerTasksDelete  = "server.tasks.delete"
    
    // Database
    ScopeServerDatabaseView = "server.database.view"
    ScopeServerDatabaseCreate = "server.database.create"
    ScopeServerDatabaseDelete = "server.database.delete"
    
    // Users / Permissions
    ScopeServerUsersView    = "server.users.view"
    ScopeServerUsersEdit    = "server.users.edit"
    ScopeServerUsersCreate  = "server.users.create"
    ScopeServerUsersDelete  = "server.users.delete"
    
    // Plugins
    ScopeServerPluginsView  = "server.plugins.view"
    ScopeServerPluginsEdit  = "server.plugins.edit"
    
    // Transfers
    ScopeServerTransferView = "server.transfer.view"
    ScopeServerTransferManage = "server.transfer.manage"
    ScopeServerExTransferView = "server.extransfer.view"
    ScopeServerExTransferManage = "server.extransfer.manage"
    
    // Plugins
    ScopeServerPluginsView  = "server.plugins.view"
    ScopeServerPluginsEdit  = "server.plugins.edit"
    
    // Clients / OAuth2
    ScopeServerClientsView  = "server.clients.view"
    ScopeServerClientsEdit  = "server.clients.edit"
    ScopeServerClientsCreate = "server.clients.create"
    ScopeServerClientsDelete = "server.clients.delete"
    
    // Admin scopes
    ScopeServerAdmin        = "server.admin"
    ScopeServerAdminView    = "server.admin.view"
    ScopeServerAdminConfigView = "server.admin.config.view"
    ScopeServerAdminConfigManage = "server.admin.config.manage"
    ScopeServerAdminInstallView = "server.admin.install.view"
    ScopeServerAdminInstallManage = "server.admin.install.manage"
    ScopeServerAdminTransferView = "server.admin.transfer.view"
    ScopeServerAdminTransferManage = "server.admin.transfer.manage"
    ScopeServerAdminAssignmentsView = "server.admin.assignments.view"
    ScopeServerAdminAssignmentsManage = "server.admin.assignments.manage"
    ScopeServerAdminClientsView = "server.admin.clients.view"
    ScopeServerAdminClientsEdit = "server.admin.clients.edit"
    ScopeServerAdminClientsCreate = "server.admin.clients.create"
    ScopeServerAdminClientsDelete = "server.admin.clients.delete"
    ScopeServerAdminEnvView = "server.admin.env.view"
    ScopeServerAdminEnvManage = "server.admin.env.manage"
    ScopeServerAdminDataView = "server.admin.data.view"
    ScopeServerAdminDataEdit = "server.admin.data.edit"
    ScopeServerAdminBackupView = "server.admin.backup.view"
    ScopeServerAdminBackupManage = "server.admin.backup.manage"
    ScopeServerAdminTasksView = "server.admin.tasks.view"
    ScopeServerAdminTasksManage = "server.admin.tasks.manage"
    
    // Clients / OAuth2
    ScopeServerClientsView  = "server.clients.view"
    ScopeServerClientsEdit  = "server.clients.edit"
    ScopeServerClientsCreate = "server.clients.create"
    ScopeServerClientsDelete = "server.clients.delete"
    
    // Global
    ScopeLogin              = "login"
    ScopePanel              = "panel"
    ScopeSettingsEdit       = "settings.edit"
    ScopeOAuth2Auth         = "oauth2.auth"
    ScopeLicense            = "license"
    ScopeUsersInfoSearch    = "users.info.search"
    ScopeUsersInfoView      = "users.info.view"
    ScopeUsersInfoEdit      = "users.info.edit"
    ScopeUsersPermsView     = "users.perms.view"
    ScopeUsersPermsEdit     = "users.perms.edit"
    ScopeSelfEdit           = "self.edit"
    ScopeNodesView          = "nodes.view"
    ScopeNodesCreate        = "nodes.create"
    ScopeNodesEdit          = "nodes.edit"
    ScopeNodesDelete        = "nodes.delete"
    ScopeNodesDeploy        = "nodes.deploy"
    ScopeSelfEdit           = "self.edit"
    ScopeSelfClients        = "self.clients"
    ScopeOAuth2Auth         = "oauth2.auth"
    ScopeLicense            = "license"
    ScopeUsersInfoSearch    = "users.info.search"
    ScopeUsersInfoView      = "users.info.view"
    ScopeUsersInfoEdit      = "users.info.edit"
    ScopeUsersPermsView     = "users.perms.view"
    ScopeUsersPermsEdit     = "users.perms.edit"
    ScopeTemplatesView      = "templates.view"
    ScopeTemplatesLocalEdit = "templates.local.edit"
    ScopeTemplatesRepoCreate = "templates.repo.create"
    ScopeTemplatesRepoDelete = "templates.repo.delete"
    ScopeDatabaseHostsManage = "databasehosts.manage"
    ScopeProvisionManage    = "provision.manage"
    ScopeProvisionView      = "provision.view"
    ScopeUptimeView         = "uptime.view"
    ScopeRolesView          = "roles.view"
    ScopeRolesEdit          = "roles.edit"
    ScopeDatabaseHostsManage = "databasehosts.manage"
)
```

---

## Best Practices

1. **Use most specific scope** - Prefer `server.data.edit` over `server.data.edit.admin` when possible
2. **Combine with roles** - Assign roles instead of individual scopes to users
3. **Audit regularly** - Review permissions quarterly
4. **Principle of least privilege** - Grant minimum scopes needed
5. **Document custom scopes** - If adding new scopes, update this doc
# Database hosts and server databases

Database hosts let Aether Panel create MySQL/MariaDB databases for game servers on demand. An administrator registers a database server once (a **database host**); after that, anyone who manages a server can create and delete databases for it from the server's **Database** tab, and the panel creates the schema, a dedicated user and its password automatically.

[Panel usage](../general/usages.md#database-hosts) has a short introduction; this page goes into the details.

## What it does

- Registers one or more MySQL/MariaDB servers as database hosts, each optionally tied to a node and with an optional database limit.
- Tests the connection to a host before saving it.
- Creates a database per request with its own user that only has privileges on that database.
- Lists a server's databases with host, port, user and password (the password only for those who may manage them), and deletes them (database and user) on request.
- Drops every database of a server when the server is deleted.

## How it works

### Database hosts

A host stores a name, connection host (IP, FQDN or hostname), port (default 3306), an administrative username and password, a **database limit** (`max_databases`, 0 = unlimited) and an optional **node** (`node_id`). The password is stored in the panel database and is never returned by the API.

Only MySQL and MariaDB are supported: the panel connects with the Go MySQL driver (`go-sql-driver/mysql`) using 5-second connect, read and write timeouts.

### Creating a database

When a server asks for a database named `<name>` on a host, the panel:

1. Checks that the host accepts the server:
   - If the host has a node set, the server must run on that node (otherwise `ErrDatabaseHostOtherNode`).
   - If the host has a limit, the number of databases on it must be below the limit (otherwise `ErrDatabaseHostFull`).
2. Builds the real name `s_<serverId>_<name>`. Both parts may only contain letters, digits, `_` and `-`, and the whole name must be at most 64 characters (the MySQL limit). With the panel's generated 12-character server identifiers this leaves 49 characters for `<name>`. Otherwise: `ErrInvalidDatabaseName`.
3. Refuses a name already tracked on that host (`ErrDatabaseNameTaken`).
4. Generates a user `db_` + 10 random lowercase letters/digits and a 16-character random password.
5. On the host runs `CREATE DATABASE` (without `IF NOT EXISTS`, so an existing untracked schema makes it fail instead of being taken over) with `utf8mb4` / `utf8mb4_unicode_ci`, then `CREATE USER '<user>'@'%'`, `GRANT ALL PRIVILEGES ON <db>.* TO '<user>'@'%'` and `FLUSH PRIVILEGES`. If any step after `CREATE DATABASE` fails, the user and the database are dropped again so a retry with the same name works.
6. Saves the record, including `remote_connection` (`host:port` of the database host).

The prefix keeps tenants apart on a shared host: one server can never create or drop another server's database or a system schema.

### Deleting

Deleting a database runs `DROP USER IF EXISTS`, `DROP DATABASE IF EXISTS` and `FLUSH PRIVILEGES` on the host, then removes the record. The API checks that the database belongs to the server in the URL.

When a **server** is deleted, the panel drops all its databases and users first. This is best effort: if the host is unreachable the server is deleted anyway, the panel logs `server <id>: database <name> and its user <user> could not be dropped from host <id> (<name>), remove them by hand: <reason>` (the reason says whether connecting, dropping the user or dropping the database failed, and names a missing privilege only when MySQL denied it), and whoever deleted the server sees a warning listing the databases left on the host.

A **database host** can only be deleted once it has no databases left: while it still has any, the deletion is refused with HTTP 409 `ErrDatabaseHostInUse`. This way no databases or users are left on the MySQL server with nothing in the panel to delete them. Delete the databases from their servers first, then the host.

### Which hosts a server sees

`GET /api/servers/:id/databasehosts` returns only the hosts the server can use (serving its node or every node, and with room left), as `{id, name}` pairs: server owners never see the address or credentials of a host.

## Using it

### In the panel

**Register a host** (sidebar **Database Hosts**, requires `admin`):

1. Click **Add Host**. The wizard has three steps: **Credentials**, **Instructions**, **Configuration**.
2. **Credentials**: username and password of a MySQL account allowed to create users and databases.
3. **Instructions** shows the SQL to create such an account, for example:

   ```sql
   CREATE USER 'aetheruser'@'%' IDENTIFIED BY 'your_strong_password';
   GRANT ALL PRIVILEGES ON *.* TO 'aetheruser'@'%' WITH GRANT OPTION;
   ```

   and, for a host on another machine, `bind-address=0.0.0.0` under `[mysqld]` in `my.cnf` plus opening the port in the firewall. When the panel runs in Docker it connects from its container's IP, so the account must be created for `'%'`.
4. **Configuration**: **Host Name** (label in the panel), **Connection Host**, **Port**, **Database Limit** (blank or 0 = unlimited) and **Linked Nodes**.
5. Use **Test connection** to check the credentials, then **Create Host**.

To edit a host use **Edit** in its row menu; leave the password empty to keep the current one.

**Create a database for a server** (server page → **Database** tab, needs `server.data.edit`):

1. Click **New Database**, enter a **Database Name** and pick a **Database Host**.
2. Click **Create Database**. A dialog shows host, port, name, user and password: "Save these credentials".
3. Each database row shows user, password (hidden by default, with show/copy controls) and **Host and port**. When the API does not send the password (the caller lacks `server.data.edit`), the row shows "Only visible to users who can manage databases" instead. The bin icon deletes the database after confirmation.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/databasehosts` | `admin` | List hosts (no passwords) |
| `POST` | `/api/databasehosts` | `admin` | Create a host |
| `POST` | `/api/databasehosts/test` | `admin` | Test a connection (`host`, `port` default 3306, `username`, `password`) |
| `GET` / `PUT` / `DELETE` | `/api/databasehosts/:id` | `admin` | Read, update (empty `password` keeps the current one), delete (409 `ErrDatabaseHostInUse` while it still has databases) |
| `GET` | `/api/servers/:serverId/databasehosts` | `server.data.edit` | Hosts this server may use (`id`, `name`) |
| `GET` | `/api/servers/:serverId/databases` | `server.view` | Databases of the server. The `password` field is only included when the caller holds `server.data.edit` on that server; otherwise it is omitted |
| `POST` | `/api/servers/:serverId/databases` | `server.data.edit` | Create a database: `{"database_host_id": 1, "database_name": "shop"}` |
| `DELETE` | `/api/servers/:serverId/databases/:id` | `server.data.edit` | Delete a database and its user |

Validation errors from creation (`ErrInvalidDatabaseName`, `ErrDatabaseNameTaken`, `ErrDatabaseHostFull`, `ErrDatabaseHostOtherNode`) return HTTP 400; MySQL errors return 500. See the [API reference](../reference/11-api-reference.md) and the [scopes reference](../general/scopes-reference.md).

## Requirements and configuration

- A reachable MySQL or MariaDB server and an account with `CREATE`, `CREATE USER`, `DROP`, `DROP USER` and `GRANT OPTION` (the `GRANT ALL ... WITH GRANT OPTION` above covers them).
- The connection host must be reachable **from the panel** (which creates the databases) and **from the game servers** (which use the credentials): the address saved as Connection Host is the one shown to users.
- On the Docker install, the bundled MariaDB (`skypanel-mysql`, `mariadb:10.11`) is on both the panel network and the game servers' network (`skypanel-servers`), so it can be registered with Connection Host `mysql` and port `3306`. It is not registered automatically, and its host port (`127.0.0.1:3307`) is only published on the loopback interface.
- No configuration keys apply to this feature. See [Configuration](../backend/02-configuration.md) for the panel's own database (`panel.database.*`), which is separate.

## Limits

- MySQL/MariaDB only.
- Database names: letters, digits, `_` and `-`; 64 characters in total including the `s_<serverId>_` prefix.
- Users are created for `'%'` (any host); restrict network access to the database port with a firewall.
- The **Database Limit** counts databases on the host across all servers; there is no per-server limit.
- `max_connections` exists in the database record but the panel does not set or enforce it.
- Passwords of server databases are stored in the panel database. The API only returns them to callers with `server.data.edit` on the server; with `server.view` alone you see host, port and user.
- A host that still has databases cannot be deleted (`ErrDatabaseHostInUse`).

## Troubleshooting

| Message | Cause | Fix |
|---------|-------|-----|
| `connection failed: …` (Test connection) | Wrong host/port/credentials, or MySQL refuses remote connections | Check `bind-address`, the firewall and that the user exists for `'%'` |
| "The connection attempt timed out." | No answer within 15 s (the UI's limit) | Check routing/firewall between the panel and the host |
| `host and username are required` | Empty fields sent to the test endpoint | Fill in host and username |
| `failed to connect to MySQL host (check credentials and host permissions)` | The saved credentials no longer work | Edit the host with the correct password |
| `failed to create database (ensure user has CREATE permissions and the name is free)` | Missing privilege, or a schema with that name already exists on the host | Grant `CREATE`; drop or rename the stray schema |
| `failed to create MySQL user (ensure user has CREATE USER permissions)` | Missing `CREATE USER` | Grant it to the host account |
| `failed to grant privileges (ensure user has GRANT OPTION)` | Account lacks `WITH GRANT OPTION` | Recreate the grant with `WITH GRANT OPTION` |
| `database name may only use letters, digits, '_' and '-', within the length limit` | Invalid characters or too long | Use a shorter name with allowed characters |
| `a database with that name already exists on this host` | The name is already tracked on that host | Pick another name |
| `this database host has reached its database limit` | Host is at its **Database Limit** | Raise the limit or use another host |
| `this database host serves another node` | The host is tied to a different node than the server's | Use a host for that node or one without a node |
| `database does not belong to this server` (403) | The database ID belongs to another server | Use the right server ID |
| The **Database** tab is missing | The user lacks `server.data.edit` on that server | Grant it (server owners have it by default) |
| `this database host still has databases, delete them first` (409) | Tried to delete a host that still has databases | Delete them from each server's **Database** tab, then try again |
| The host list in the create dialog is empty | No host serves the server's node or all are full | Add a host, link it to the node or raise the limit |

## See also

- [Panel usage: database hosts](../general/usages.md#database-hosts)
- [Core concepts](../general/concepts.md)
- [Scopes reference](../general/scopes-reference.md)
- [Configuration](../backend/02-configuration.md)
- [API reference](../reference/11-api-reference.md)

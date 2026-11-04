package skypanel

import (
	"errors"
	"runtime/debug"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"gopkg.in/go-playground/validator.v9"
)

var ErrUnknownError = CreateError("unknown error", "ErrUnknownError")
var ErrInternal = CreateError("internal error, the details are in the panel log", "ErrInternal")
var ErrInvalidCredentials = CreateError("invalid credentials", "ErrInvalidCredentials")
var ErrTooManyAttempts = CreateError("too many failed attempts, wait a few minutes and try again", "ErrTooManyAttempts")
var ErrServiceNotAvailable = CreateError("service not available", "ErrServiceNotAvailable")
var ErrFeatureDisabled = CreateError("this experimental feature is turned off in the panel settings", "ErrFeatureDisabled")
var ErrAIAnalysisDisabled = CreateError("the AI log analysis is turned off in the panel settings", "ErrAIAnalysisDisabled")
var ErrAIConfirmationInvalid = CreateError("this request expired, was already answered or belongs to someone else: ask again", "ErrAIConfirmationInvalid")
var ErrEmailNotConfigured = CreateError("email not configured", "ErrEmailNotConfigured")
var ErrTokenInvalid = CreateError("token is invalid", "ErrTokenInvalid")
var ErrClientNotFound = CreateError("client not found", "ErrClientNotFound")
var ErrUserNotFound = CreateError("user not found", "ErrUserNotFound")
var ErrLoginNotPermitted = CreateError("login not permitted", "ErrLoginNotPermitted")

// ErrAccountPending is a registered account an administrator has not approved yet
var ErrAccountPending = CreateError("your account is waiting for an administrator to approve it", "ErrAccountPending")
var ErrOtpAlreadyEnabled = CreateError("2FA is already enabled, disable it before enrolling again", "ErrOtpAlreadyEnabled")
var ErrDatabaseHostFull = CreateError("this database host has reached its database limit", "ErrDatabaseHostFull")
var ErrDatabaseHostOtherNode = CreateError("this database host serves another node", "ErrDatabaseHostOtherNode")
var ErrDatabaseHostInUse = CreateError("this database host still has databases, delete them first", "ErrDatabaseHostInUse")
var ErrDatabaseNameTaken = CreateError("a database with that name already exists on this host", "ErrDatabaseNameTaken")
var ErrInvalidDatabaseName = CreateError("database name may only use letters, digits, '_' and '-', within the length limit", "ErrInvalidDatabaseName")
var ErrInvalidTokenState = CreateError("invalid token state", "ErrInvalidTokenState")
var ErrNoPermission = CreateError("no permission to perform action", "ErrNoPermission")
var ErrServerNotFound = CreateError("server not found", "ErrServerNotFound")
var ErrDatabaseNotAvailable = CreateError("database not available", "ErrDatabaseNotAvailable")
var ErrServerOffline = CreateError("server offline", "ErrServerOffline")
var ErrIllegalFileAccess = CreateError("invalid file access", "ErrIllegalFileAccess")
var ErrFileNotFound = CreateError("file not found", "ErrFileNotFound")
var ErrFileExists = CreateError("file exists where there should be none", "ErrFileExists")
var ErrServerDisabled = CreateError("server is disabled", "ErrServerDisabled")
var ErrContainerRunning = CreateError("container already running", "ErrContainerRunning")
var ErrImageDownloading = CreateError("image downloading", "ErrImageDownloading")
var ErrProcessRunning = CreateError("process already running", "ErrProcessRunning")
var ErrMissingFactory = CreateError("missing factory", "ErrMissingFactory")
var ErrServerAlreadyExists = CreateError("server already exists", "ErrServerAlreadyExists")
var ErrInvalidUnixTime = CreateError("time provided is not a valid UNIX time", "ErrInvalidUnixTime")
var ErrKeyNotPEM = CreateError("key is not in PEM format", "ErrKeyNotPEM")
var ErrCannotValidateToken = CreateError("could not validate access token", "ErrCannotValidateToken")
var ErrMissingAccessToken = CreateError("access token not provided", "ErrMissingAccessToken")
var ErrNotBearerToken = CreateError("access token must be a Bearer token", "ErrNotBearerToken")
var ErrKeyNotECDSA = CreateError("key is not ECDSA key", "ErrKeyNotECDSA")
var ErrMissingScope = CreateError("missing scope", "ErrMissingScope")
var ErrNodeHasServers = CreateError("node has servers associated", "ErrNodeHasServers")
var ErrInvalidSession = CreateError("invalid session", "ErrInvalidSession")
var ErrSessionExpired = CreateError("session expired", "ErrSessionExpired")
var ErrTaskNotFound = CreateError("task not found", "ErrTaskNotFound")
var ErrNotImplemented = CreateError("not implemented", "ErrNotImplemented")
var ErrDockerNotSupported = CreateError("docker not supported", "ErrDockerNotSupported")
var ErrServerRunning = CreateError("server running", "ErrServerRunning")
var ErrConsoleDetached = CreateError("the panel is not attached to this server's console since it restarted; kill and start the server to reconnect", "ErrConsoleDetached")
var ErrSSHKeyInvalid = CreateError("not a valid SSH public key, paste one line such as the contents of id_ed25519.pub", "ErrSSHKeyInvalid")
var ErrSSHKeyIsPrivate = CreateError("that is a private key, paste the public key (the .pub file) instead", "ErrSSHKeyIsPrivate")
var ErrSSHKeyWeak = CreateError("RSA keys must have at least 2048 bits", "ErrSSHKeyWeak")
var ErrSSHKeyExists = CreateError("that key is already added to your account", "ErrSSHKeyExists")
var ErrSSHKeyLimit = CreateError("you have reached the maximum number of SSH keys", "ErrSSHKeyLimit")
var ErrSSHKeyName = CreateError("give the key a name of up to 100 characters", "ErrSSHKeyName")
var ErrCaptchaFailed = CreateError("the anti-bot check failed, try it again", "ErrCaptchaFailed")
var ErrSSONotForAdmins = CreateError("sign-in links are not issued for administrator accounts", "ErrSSONotForAdmins")
var ErrServerOwner = CreateError("the server's owner cannot be removed or limited; an administrator can give the server to someone else", "ErrServerOwner")
var ErrAPIKeyNotAllowed = CreateError("this API key is not allowed to do that", "ErrAPIKeyNotAllowed")
var ErrAPIKeyPermissions = CreateError("pick at least one action, from: create, terminate, suspend, change-package, sso", "ErrAPIKeyPermissions")
var ErrDiskLimitExceeded = CreateError("the server has used all of its disk space, delete files to free some", "ErrDiskLimitExceeded")
var ErrNodeDiskFull = CreateError("the node is almost out of disk space, free some before installing", "ErrNodeDiskFull")
var ErrNodeAuthFailed = CreateError("the node refused the panel's credentials: check that the node's secret matches the panel and that both clocks are right", "ErrNodeAuthFailed")
var ErrNoCommand = CreateError("command could not be determined", "ErrNoCommand")
var ErrRepoExists = CreateError("repo already exists with given name", "ErrRepoExists")
var ErrEnvironmentNotSupported = CreateError("environment not supported", "ErrEnvironmentNotSupported")
var ErrPasswordRequirements = CreateError("password does not meet requirements", "ErrPasswordRequirements")
var ErrBackupInProgress = CreateError("backup in progress", "ErrBackupInProgress")
var ErrInviteSignupsClosed = CreateError("sign-ups are closed: that person needs an account first", "ErrInviteSignupsClosed")
var ErrBackupLimit = CreateError("this server already has as many backups as it may keep: delete an old one first", "ErrBackupLimit")
var ErrBackupServerRunning = CreateError("cannot backup server, is running", "ErrBackupServerRunning")
var ErrContainerNotUnique = CreateError("multiple containers found", "ErrContainerNotUnique")
var ErrNoContainerFound = CreateError("no container found", "ErrNoContainerFound")
var ErrNoMountFound = CreateError("no mount found", "ErrNoMountFound")
var ErrUnsupportedMountType = CreateError("unsupported mount type", "ErrUnsupportedMountType")

func CreateErrMissingScope(scope scopes.Scope) *Error {
	return CreateError(ErrMissingScope.Message, ErrMissingScope.Code).Metadata(map[string]interface{}{"scope": scope})
}

var ErrSettingNotConfigured = func(name string) *Error {
	return CreateError("${setting} is not configured", "ErrSettingNotConfigured").Metadata(map[string]interface{}{"setting": name})
}

var ErrNoTemplate = func(template string) *Error {
	return CreateError("no template with name ${name}", "ErrNoTemplate").Metadata(map[string]interface{}{"template": template})
}

var ErrServiceInvalidProvider = func(service, provider string) *Error {
	return CreateError("${service} does not support ${provider}", "ErrServiceInvalidProvider").Metadata(map[string]interface{}{"service": service, "provider": provider})
}

var ErrTransferRunning = CreateError("this server is already being transferred: wait for that transfer to end", "ErrTransferRunning")
var ErrTransferBackupRunning = CreateError("a backup or restore of this server is being written: transfer it once it is done", "ErrTransferBackupRunning")
var ErrTransferSubServer = CreateError("a sub-server stays on its parent's node: transfer the parent server and its sub-servers go with it", "ErrTransferSubServer")

var ErrTransferPortsTaken = func(ports []uint16) *Error {
	return CreateError("ports ${ports} of this server are already used on the target node: free them or pick another node", "ErrTransferPortsTaken").Metadata(map[string]interface{}{"ports": ports})
}

// The port errors take the port as text: the number, or the number and the template variable
// it comes from, such as "27015 (queryport)"
var ErrPortOutOfRange = func(port string) *Error {
	return CreateError("port ${port} is outside the allowed range (1024-65535)", "ErrPortOutOfRange").Metadata(map[string]interface{}{"port": port})
}

var ErrPortDuplicated = func(port string) *Error {
	return CreateError("port ${port} is listed more than once", "ErrPortDuplicated").Metadata(map[string]interface{}{"port": port})
}

var ErrPortUsedByPanel = func(port string) *Error {
	return CreateError("port ${port} is used by the panel itself on this node", "ErrPortUsedByPanel").Metadata(map[string]interface{}{"port": port})
}

var ErrPortTaken = func(port string) *Error {
	return CreateError("port ${port} is already used by another server on this node", "ErrPortTaken").Metadata(map[string]interface{}{"port": port})
}

var ErrPortBusy = func(port uint16) *Error {
	return CreateError("port ${port} is already in use by another server or process on this node: stop whatever is using it or change the server's port before starting it", "ErrPortBusy").Metadata(map[string]interface{}{"port": port})
}

var ErrCloudflareRecordID = func(id string) *Error {
	return CreateError("invalid DNS record ID: ${id}", "ErrCloudflareRecordID").Metadata(map[string]interface{}{"id": id})
}

var ErrCloudflareDomainNotInZone = func(domain, zone string) *Error {
	return CreateError("the domain ${domain} does not belong to the zone ${zone}", "ErrCloudflareDomainNotInZone").Metadata(map[string]interface{}{"domain": domain, "zone": zone})
}

var ErrCloudflareUnreachable = func(reason string) *Error {
	return CreateError("could not reach Cloudflare: ${reason}", "ErrCloudflareUnreachable").Metadata(map[string]interface{}{"reason": reason})
}

var ErrCloudflareBadAnswer = func(reason string) *Error {
	return CreateError("Cloudflare's answer could not be read: ${reason}", "ErrCloudflareBadAnswer").Metadata(map[string]interface{}{"reason": reason})
}

// ErrCloudflareRefused and ErrCloudflareRefusedCode carry Cloudflare's own message, and its
// error code when it gave one
var ErrCloudflareRefused = func(message string) *Error {
	return CreateError("Cloudflare answered: ${message}", "ErrCloudflareRefused").Metadata(map[string]interface{}{"message": message})
}

var ErrCloudflareRefusedCode = func(message string, code int) *Error {
	return CreateError("Cloudflare answered: ${message} (code ${cloudflareCode})", "ErrCloudflareRefusedCode").Metadata(map[string]interface{}{"message": message, "cloudflareCode": code})
}

var ErrDNSBaseDomainInvalid = CreateError("the base domain is not a valid domain name", "ErrDNSBaseDomainInvalid")
var ErrDiscordNoWebhooks = CreateError("there are no Discord webhooks configured", "ErrDiscordNoWebhooks")

// ErrDiscordTestFailed names the webhooks the test message did not reach: alerts, reports,
// status or transfers, which the page shows with the names of its own sections
var ErrDiscordTestFailed = func(webhooks []string) *Error {
	return CreateError("the test message did not reach the ${webhooks} webhook; check that it still exists in Discord", "ErrDiscordTestFailed").Metadata(map[string]interface{}{"webhooks": webhooks})
}

var ErrNotificationLanguage = CreateError("the notification language must be es or en", "ErrNotificationLanguage")

var ErrFieldRequired = func(fieldName string) *Error {
	return CreateError("${field} is required", "ErrFieldRequired").Metadata(map[string]interface{}{"field": fieldName})
}

var ErrFieldMustBePrintable = func(fieldName string) *Error {
	return CreateError("${field} must be printable ascii characters", "ErrFieldMustBePrintable").Metadata(map[string]interface{}{"field": fieldName})
}

var ErrFieldHasURICharacters = func(fieldName string) *Error {
	return CreateError("${field} must not contain characters which cannot be used in URIs", "ErrFieldHasURICharacters").Metadata(map[string]interface{}{"field": fieldName})
}

var ErrFieldIsInvalidHost = func(fieldName string) *Error {
	return CreateError("${field} must be a valid IP or FQDN", "ErrFieldIsInvalidHost").Metadata(map[string]interface{}{"field": fieldName})
}

var ErrFieldIsInvalidIP = func(fieldName string) *Error {
	return CreateError("${field} must be a valid IP", "ErrFieldIsInvalidIP").Metadata(map[string]interface{}{"field": fieldName})
}

var ErrFieldTooLarge = func(fieldName string, value int64) *Error {
	return CreateError("${field} cannot be larger than ${max}", "ErrFieldTooLarge").Metadata(map[string]interface{}{"field": fieldName, "max": value})
}

var ErrFieldTooSmall = func(fieldName string, value int64) *Error {
	return CreateError("${field} cannot be smaller than ${min}", "ErrFieldTooSmall").Metadata(map[string]interface{}{"field": fieldName, "min": value})
}

var ErrFieldNotBetween = func(fieldName string, minVal, maxVal int64) *Error {
	return CreateError("${field} must be between ${min} and ${max}", "ErrFieldNotBetween").Metadata(map[string]interface{}{"field": fieldName, "min": minVal, "max": maxVal})
}

var ErrFieldEqual = func(fieldName1, fieldName2 string) *Error {
	return CreateError("${field1} cannot be equal to ${field2}", "ErrFieldEqual").Metadata(map[string]interface{}{"field1": fieldName1, "field2": fieldName2})
}

var ErrFieldNotEqual = func(fieldName1, fieldName2 string) *Error {
	return CreateError("${field1} is not equal to ${field2}", "ErrFieldNotEqual").Metadata(map[string]interface{}{"field1": fieldName1, "field2": fieldName2})
}

var ErrFieldNotEmail = func(fieldName string) *Error {
	return CreateError("${field} is not a valid email", "ErrFieldNotEmail").Metadata(map[string]interface{}{"field": fieldName})
}

var ErrFieldLength = func(fieldName string, minVal int, maxVal int) *Error {
	return CreateError("${field} must be between ${min} and ${max} characters", "ErrFieldLength").Metadata(map[string]interface{}{"field": fieldName, "min": minVal, "max": maxVal})
}

var ErrFactoryError = func(operatorName string, err error) *Error {
	return CreateError("factory `${operatorName}` encountered an error: `${err}`", "ErrFactoryError").Metadata(map[string]interface{}{"operatorName": operatorName, "err": err.Error()})
}

var ErrNodeInvalid = CreateError("node is invalid", "ErrNodeInvalid")

var ErrUnsupportedOS = func(actual, expected string) *Error {
	return CreateError("OS (${actual}) not supported. Supported OS: ${expected}", "ErrUnsupportedOS").Metadata(map[string]interface{}{"actual": actual, "expected": expected})
}

var ErrUnsupportedArch = func(actual, expected string) *Error {
	return CreateError("Architecture ${actual} not supported. Supported Architectures: ${expected}", "ErrUnsupportedArch").Metadata(map[string]interface{}{"actual": actual, "expected": expected})
}

var ErrMissingBinary = func(expected string) *Error {
	return CreateError("missing binary: ${expected}", "ErrMissingBinary").Metadata(map[string]interface{}{"expected": expected})
}

var ErrPathNotAbs = func(path string) *Error {
	return CreateError("path not abs: ${path}", "ErrPathNotAbs").Metadata(map[string]interface{}{"path": path})
}

var ErrCurseForgeDistribution = func(projectId uint) *Error {
	return CreateError("CurseForge modpack with project ID ${projectId} does not allow third-party distribution", "ErrCurseForgeDistribution").Metadata(map[string]interface{}{"projectId": projectId})
}

var ErrCurseForgeFile = func(projectId uint, fileID uint) *Error {
	return CreateError("File ID ${fileID} under project ID ${projectId} not found", "ErrCurseForgeFile").Metadata(map[string]interface{}{"fileID": fileID, "projectId": projectId})
}

var ErrCurseForgeStatus = func(status string) *Error {
	return CreateError("Invalid status code from CurseForge: ${status}", "ErrCurseForgeStatus").Metadata(map[string]interface{}{"status": status})
}

func GenerateValidationMessage(err error) error {
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		msg := make([]string, 0)
		for _, e := range errs {
			t := e.Field() + ": " + e.ActualTag()
			if e.Param() != "" {
				t += " (" + e.Param() + ")"
			}
			msg = append(msg, t)
		}
		return errors.New(strings.Join(msg, ", "))
	}
	return nil
}

type Error struct {
	Message string                 `json:"msg,omitempty"`
	Code    string                 `json:"code,omitempty"`
	Meta    map[string]interface{} `json:"metadata,omitempty"`
	// error
} // @name Error

func (ge *Error) GetMessage() string {
	return utils.ReplaceTokens(ge.Message, ge.Meta)
}

func (ge *Error) GetCode() string {
	return ge.Code
}

func (ge *Error) Error() string {
	return ge.GetMessage()
}

func (ge *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return ge.GetCode() == t.GetCode()
}

func (ge *Error) Metadata(metadata map[string]interface{}) *Error {
	cp := ge
	cp.Meta = metadata
	return cp
}

func CreateError(msg, code string) *Error {
	return &Error{
		Message: msg,
		Code:    code,
	}
}

func FromError(err error) *Error {
	if err == nil {
		return nil
	}

	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return CreateError(err.Error(), "ErrGeneric")
}

func Recover() {
	if err := recover(); err != nil {
		if _, ok := err.(error); !ok {
			err = errors.New(utils.ToString(err))
		}

		logging.Error.Printf("CRITICAL ERROR: \n%+v\n%s", err, debug.Stack())
	}
}

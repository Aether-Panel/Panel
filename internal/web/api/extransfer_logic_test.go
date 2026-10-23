package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const exTestServer = "srv01"

func setupExTransferTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "extransfer.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Node{}, &models.Server{}, &models.ExTransferSession{}, &models.ExTransferLog{}))

	previous := removeOriginArchive
	removeOriginArchive = func(*gorm.DB, string) {}
	t.Cleanup(func() { removeOriginArchive = previous })

	return db
}

func newExTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return pub, priv
}

func newExTestSession(t *testing.T, db *gorm.DB, status models.TransferStatus, destKey ed25519.PublicKey, mutate func(*models.ExTransferSession)) *models.ExTransferSession {
	t.Helper()
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	session := &models.ExTransferSession{
		SessionUUID:   uuid.NewString(),
		ServerID:      exTestServer,
		UserID:        1,
		TokenHash:     hex.EncodeToString(hash),
		Status:        status,
		DestPublicKey: base64.StdEncoding.EncodeToString(destKey),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	}
	if mutate != nil {
		mutate(session)
	}
	require.NoError(t, db.Omit(clause.Associations).Create(session).Error)
	return session
}

func exSign(key ed25519.PrivateKey, action, sessionID string) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(action+":"+sessionID)))
}

func callExSigned(t *testing.T, db *gorm.DB, handler gin.HandlerFunc, sessionID, signature string) (int, map[string]interface{}) {
	t.Helper()
	body, _ := json.Marshal(ExSignedReq{SessionID: sessionID, Signature: signature})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("db", db)
	handler(c)

	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func callExDownload(t *testing.T, db *gorm.DB, sessionID, signature string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	query := url.Values{"session_id": {sessionID}, "signature": {signature}}.Encode()
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	c.Set("db", db)
	downloadExTransfer(c)

	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func reloadExSession(t *testing.T, db *gorm.DB, s *models.ExTransferSession) *models.ExTransferSession {
	t.Helper()
	var fresh models.ExTransferSession
	require.NoError(t, db.Where("id = ?", s.ID).First(&fresh).Error)
	return &fresh
}

func countExLogs(t *testing.T, db *gorm.DB, sessionID, action string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.ExTransferLog{}).Where("session_id = ? AND action = ?", sessionID, action).Count(&n).Error)
	return n
}

func TestExTransferHeartbeat_ExtendsTheExpiry(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
		s.ExpiresAt = time.Now().Add(time.Minute)
	})

	code, out := callExSigned(t, db, heartbeatExTransfer, session.SessionUUID, exSign(priv, "HEARTBEAT", session.SessionUUID))

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, string(models.StatusMigrating), out["status"])
	fresh := reloadExSession(t, db, session)
	assert.WithinDuration(t, time.Now().Add(exTransferHeartbeatExtension), fresh.ExpiresAt, 5*time.Second)
	if assert.NotNil(t, fresh.LastHeartbeatAt) {
		assert.WithinDuration(t, time.Now(), *fresh.LastHeartbeatAt, 5*time.Second)
	}
}

func TestExTransferHeartbeat_NeverPastTheCap(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	created := time.Now().Add(-exTransferMaxLifetime + time.Minute)
	session := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
		s.CreatedAt = created
		s.ExpiresAt = time.Now().Add(30 * time.Second)
	})

	code, _ := callExSigned(t, db, heartbeatExTransfer, session.SessionUUID, exSign(priv, "HEARTBEAT", session.SessionUUID))

	assert.Equal(t, http.StatusOK, code)
	fresh := reloadExSession(t, db, session)
	limit := created.Add(exTransferMaxLifetime)
	assert.False(t, fresh.ExpiresAt.After(limit.Add(time.Millisecond)), "expiry %s is past the cap %s", fresh.ExpiresAt, limit)
	assert.True(t, fresh.ExpiresAt.After(session.ExpiresAt), "the expiry still moves forward up to the cap")

	// once at the cap, heartbeats keep it there and the session runs out
	require.NoError(t, db.Model(fresh).Update("expires_at", time.Now().Add(-time.Second)).Error)
	code, out := callExSigned(t, db, heartbeatExTransfer, session.SessionUUID, exSign(priv, "HEARTBEAT", session.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "SESSION_EXPIRED", out["error_code"])
}

func TestExTransferHeartbeat_TellsTheDestinationToStop(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusCancelled, pub, nil)

	code, out := callExSigned(t, db, heartbeatExTransfer, session.SessionUUID, exSign(priv, "HEARTBEAT", session.SessionUUID))

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, string(models.StatusCancelled), out["status"])
	fresh := reloadExSession(t, db, session)
	assert.WithinDuration(t, session.ExpiresAt, fresh.ExpiresAt, time.Second, "a cancelled session is not kept alive")
	assert.Nil(t, fresh.LastHeartbeatAt)
}

func TestExTransferSignedEndpoints_RefuseBadRequests(t *testing.T) {
	endpoints := map[string]gin.HandlerFunc{
		"HEARTBEAT": heartbeatExTransfer,
		"CONFIRM":   confirmExTransfer,
		"CANCEL":    cancelExTransfer,
	}

	for action, handler := range endpoints {
		t.Run(action, func(t *testing.T) {
			db := setupExTransferTest(t)
			pub, priv := newExTestKey(t)
			_, otherPriv := newExTestKey(t)
			session := newExTestSession(t, db, models.StatusMigrating, pub, nil)
			expired := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
				s.ExpiresAt = time.Now().Add(-time.Minute)
			})

			cases := []struct {
				name      string
				sessionID string
				signature string
				wantCode  int
				wantError string
			}{
				{"unknown session", uuid.NewString(), exSign(priv, action, session.SessionUUID), http.StatusNotFound, "NOT_FOUND"},
				{"signed by another key", session.SessionUUID, exSign(otherPriv, action, session.SessionUUID), http.StatusForbidden, "INVALID_SIGNATURE"},
				{"signature of another action", session.SessionUUID, exSign(priv, "DOWNLOAD", session.SessionUUID), http.StatusForbidden, "INVALID_SIGNATURE"},
				{"signature of another session", session.SessionUUID, exSign(priv, action, expired.SessionUUID), http.StatusForbidden, "INVALID_SIGNATURE"},
				{"malformed signature", session.SessionUUID, "not base64!", http.StatusBadRequest, "INVALID_SIGNATURE_FORMAT"},
				{"expired session", expired.SessionUUID, exSign(priv, action, expired.SessionUUID), http.StatusForbidden, "SESSION_EXPIRED"},
			}
			for _, tc := range cases {
				code, out := callExSigned(t, db, handler, tc.sessionID, tc.signature)
				assert.Equal(t, tc.wantCode, code, tc.name)
				assert.Equal(t, tc.wantError, out["error_code"], tc.name)
			}

			code, out := callExSigned(t, db, handler, "", "")
			assert.Equal(t, http.StatusBadRequest, code)
			assert.Equal(t, "BAD_REQUEST", out["error_code"])

			fresh := reloadExSession(t, db, session)
			assert.Equal(t, models.StatusMigrating, fresh.Status, "refused requests change nothing")
			assert.Nil(t, fresh.LastHeartbeatAt)
			assert.Equal(t, models.StatusMigrating, reloadExSession(t, db, expired).Status)
		})
	}
}

func TestExTransferConfirm_CompletesTheSessionOnce(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	sig := exSign(priv, "CONFIRM", session.SessionUUID)

	code, out := callExSigned(t, db, confirmExTransfer, session.SessionUUID, sig)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, string(models.StatusCompleted), out["status"])
	assert.Equal(t, models.StatusCompleted, reloadExSession(t, db, session).Status)
	assert.EqualValues(t, 1, countExLogs(t, db, session.SessionUUID, "CONFIRM"))

	// the same confirmation again is fine and logs nothing new
	code, out = callExSigned(t, db, confirmExTransfer, session.SessionUUID, sig)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, string(models.StatusCompleted), out["status"])
	assert.EqualValues(t, 1, countExLogs(t, db, session.SessionUUID, "CONFIRM"))

	// the data cannot be downloaded again, nor the session kept alive or cancelled
	code, out = callExDownload(t, db, session.SessionUUID, exSign(priv, "DOWNLOAD", session.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "INVALID_STATE", out["error_code"])

	code, out = callExSigned(t, db, heartbeatExTransfer, session.SessionUUID, exSign(priv, "HEARTBEAT", session.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "INVALID_STATE", out["error_code"])

	code, out = callExSigned(t, db, cancelExTransfer, session.SessionUUID, exSign(priv, "CANCEL", session.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "INVALID_STATE", out["error_code"])
	assert.Equal(t, models.StatusCompleted, reloadExSession(t, db, session).Status)
}

func TestExTransferConfirm_NeedsAMigratingSession(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusValidated, pub, nil)

	code, out := callExSigned(t, db, confirmExTransfer, session.SessionUUID, exSign(priv, "CONFIRM", session.SessionUUID))

	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "INVALID_STATE", out["error_code"])
	assert.Equal(t, models.StatusValidated, reloadExSession(t, db, session).Status)
}

func TestExTransferCancel_IsIdempotent(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	sig := exSign(priv, "CANCEL", session.SessionUUID)

	for i := 0; i < 2; i++ {
		code, out := callExSigned(t, db, cancelExTransfer, session.SessionUUID, sig)
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, string(models.StatusCancelled), out["status"])
	}
	assert.Equal(t, models.StatusCancelled, reloadExSession(t, db, session).Status)
	assert.EqualValues(t, 1, countExLogs(t, db, session.SessionUUID, "CANCEL"), "only the first cancel is logged")

	code, out := callExDownload(t, db, session.SessionUUID, exSign(priv, "DOWNLOAD", session.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "INVALID_STATE", out["error_code"])

	code, out = callExSigned(t, db, confirmExTransfer, session.SessionUUID, exSign(priv, "CONFIRM", session.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "INVALID_STATE", out["error_code"])
}

func TestExTransferDownload_RefusesExpiredAndUnknownSessions(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	expired := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
		s.ExpiresAt = time.Now().Add(-time.Minute)
	})

	code, out := callExDownload(t, db, expired.SessionUUID, exSign(priv, "DOWNLOAD", expired.SessionUUID))
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "SESSION_EXPIRED", out["error_code"])

	code, out = callExDownload(t, db, uuid.NewString(), exSign(priv, "DOWNLOAD", expired.SessionUUID))
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "NOT_FOUND", out["error_code"])
}

func callCancelByUser(t *testing.T, db *gorm.DB) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("db", db)
	c.Set("server", &models.Server{Identifier: exTestServer, Name: "Survival"})
	c.Set("user", &models.User{Username: "owner"})
	cancelExTransferByUser(c)

	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestCancelExTransferByUser_CancelsBothSides(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	waiting := newExTestSession(t, db, models.StatusCreated, nil, nil)
	migrating := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	done := newExTestSession(t, db, models.StatusCompleted, pub, nil)
	other := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) { s.ServerID = "another" })

	ctx, finish, ok := startPull(exTestServer)
	require.True(t, ok)
	defer finish()

	code, out := callCancelByUser(t, db)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, out["cancelled_pull"])
	assert.EqualValues(t, 2, out["cancelled_sessions"])
	assert.Equal(t, models.StatusCancelled, reloadExSession(t, db, waiting).Status)
	assert.Equal(t, models.StatusCancelled, reloadExSession(t, db, migrating).Status)
	assert.Equal(t, models.StatusCompleted, reloadExSession(t, db, done).Status, "finished sessions stay finished")
	assert.Equal(t, models.StatusMigrating, reloadExSession(t, db, other).Status, "other servers are left alone")
	assert.EqualValues(t, 1, countExLogs(t, db, migrating.SessionUUID, "CANCEL_BY_OWNER"))

	select {
	case <-ctx.Done():
		assert.ErrorIs(t, context.Cause(ctx), errPullCancelledByOwner)
	default:
		t.Fatal("the running pull was not cancelled")
	}

	// the destination learns about it on its next heartbeat
	code, hb := callExSigned(t, db, heartbeatExTransfer, migrating.SessionUUID, exSign(priv, "HEARTBEAT", migrating.SessionUUID))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, string(models.StatusCancelled), hb["status"])

	// nothing is left to cancel
	finish()
	code, out = callCancelByUser(t, db)
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "NO_ACTIVE_TRANSFER", out["error_code"])
}

func TestStartPull_OnePerServer(t *testing.T) {
	_, finish, ok := startPull("pullsrv")
	require.True(t, ok)
	assert.True(t, isPullRunning("pullsrv"))

	_, _, again := startPull("pullsrv")
	assert.False(t, again, "a second pull for the same server is refused")

	finish()
	assert.False(t, isPullRunning("pullsrv"))
	assert.False(t, cancelPull("pullsrv"))

	_, finish, ok = startPull("pullsrv")
	assert.True(t, ok)
	finish()
}

// exTestOrigin serves the public transfer endpoints like the origin panel does.
func exTestOrigin(t *testing.T, db *gorm.DB, sessionID string, key ed25519.PrivateKey) *originSession {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("db", db) })
	RegisterExTransferRoutes(router.Group("/api/extransfer"))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	base, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &originSession{client: srv.Client(), base: base, id: sessionID, key: key}
}

func TestKeepOriginAlive_StopsWhenTheOriginCancels(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	origin := exTestOrigin(t, db, session.SessionUUID, priv)

	ctx, abort := context.WithCancelCause(context.Background())
	defer abort(nil)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		keepOriginAlive(ctx, origin, 10*time.Millisecond, abort)
	}()

	assert.Eventually(t, func() bool {
		return reloadExSession(t, db, session).LastHeartbeatAt != nil
	}, 5*time.Second, 10*time.Millisecond, "heartbeats reach the origin")
	assert.NoError(t, ctx.Err(), "a live session keeps the pull going")

	require.NoError(t, db.Model(&models.ExTransferSession{}).Where("id = ?", session.ID).Update("status", models.StatusCancelled).Error)

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the heartbeat loop did not notice the cancel")
	}
	assert.ErrorIs(t, context.Cause(ctx), errPullCancelledByOrigin)
}

func TestKeepOriginAlive_StopsWhenTheOriginRefusesTheSession(t *testing.T) {
	db := setupExTransferTest(t)
	_, priv := newExTestKey(t)
	origin := exTestOrigin(t, db, uuid.NewString(), priv)

	ctx, abort := context.WithCancelCause(context.Background())
	defer abort(nil)
	keepOriginAlive(ctx, origin, 10*time.Millisecond, abort)

	assert.ErrorIs(t, context.Cause(ctx), errPullCancelledByOrigin)
}

func TestOriginSession_ConfirmAndCancel(t *testing.T) {
	db := setupExTransferTest(t)
	pub, priv := newExTestKey(t)
	finished := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	failed := newExTestSession(t, db, models.StatusMigrating, pub, nil)

	assert.NoError(t, exTestOrigin(t, db, finished.SessionUUID, priv).confirm())
	assert.Equal(t, models.StatusCompleted, reloadExSession(t, db, finished).Status)

	exTestOrigin(t, db, failed.SessionUUID, priv).notifyCancel("disk full")
	assert.Equal(t, models.StatusCancelled, reloadExSession(t, db, failed).Status)
	var entry models.ExTransferLog
	require.NoError(t, db.Where("session_id = ? AND action = ?", failed.SessionUUID, "CANCEL").First(&entry).Error)
	assert.Contains(t, entry.Details, "disk full")

	// before validate there is no session to tell about
	var noSession *originSession
	noSession.notifyCancel("nothing to tell")
}

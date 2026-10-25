package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOtpTestDB(t *testing.T) *gorm.DB {
	dbPath := filepath.Join(t.TempDir(), "otp_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.RecoveryCode{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return db
}

func createOtpTestUser(t *testing.T, db *gorm.DB, secret string, active bool) *models.User {
	user := &models.User{
		Username:       "otp-test-user",
		Email:          "otp-test@example.com",
		HashedPassword: "dummy_hashed_password",
		OtpSecret:      secret,
		OtpActive:      active,
	}
	if !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	return user
}

func TestStartOtpEnroll_RefusesWhenOtpAlreadyActive(t *testing.T) {
	db := setupOtpTestDB(t)
	us := &User{DB: db}
	user := createOtpTestUser(t, db, "EXISTINGSECRETEXISTINGSECRET", true)

	_, _, err := us.StartOtpEnroll(user.ID)
	assert.ErrorIs(t, err, skypanel.ErrOtpAlreadyEnabled)

	// the active factor must be untouched: a stolen session must not be able to
	// weaken 2FA by starting a new enrollment
	reloaded, err := us.GetByID(user.ID)
	assert.NoError(t, err)
	assert.True(t, reloaded.OtpActive)
	assert.Equal(t, "EXISTINGSECRETEXISTINGSECRET", reloaded.OtpSecret)
}

func TestStartOtpEnroll_DoesNotTouchLoginSecretUntilConfirmed(t *testing.T) {
	db := setupOtpTestDB(t)
	us := &User{DB: db}
	user := createOtpTestUser(t, db, "", false)

	secret, img, err := us.StartOtpEnroll(user.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.NotEmpty(t, img)

	reloaded, err := us.GetByID(user.ID)
	assert.NoError(t, err)
	assert.False(t, reloaded.OtpActive)
	assert.Empty(t, reloaded.OtpSecret)
	assert.Equal(t, secret, reloaded.OtpPendingSecret)
}

func TestValidateOtpEnroll_ActivatesPendingSecret(t *testing.T) {
	db := setupOtpTestDB(t)
	us := &User{DB: db}
	user := createOtpTestUser(t, db, "", false)

	// an old recovery code must not survive a fresh enrollment
	stale := models.RecoveryCode{UserID: user.ID}
	assert.NoError(t, stale.SetCode("STALECODE123"))
	assert.NoError(t, db.Create(&stale).Error)

	secret, _, err := us.StartOtpEnroll(user.ID)
	assert.NoError(t, err)

	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)

	codes, err := us.ValidateOtpEnroll(user.ID, code)
	assert.NoError(t, err)
	assert.Len(t, codes, 10)

	reloaded, err := us.GetByID(user.ID)
	assert.NoError(t, err)
	assert.True(t, reloaded.OtpActive)
	assert.Equal(t, secret, reloaded.OtpSecret)
	assert.Empty(t, reloaded.OtpPendingSecret)

	var count int64
	assert.NoError(t, db.Model(&models.RecoveryCode{}).Where("user_id = ?", user.ID).Count(&count).Error)
	assert.EqualValues(t, 10, count)
}

func TestValidateOtpEnroll_RejectsWithoutPendingEnrollment(t *testing.T) {
	db := setupOtpTestDB(t)
	us := &User{DB: db}
	// user already has active 2FA and no pending enrollment
	user := createOtpTestUser(t, db, "JBSWY3DPEHPK3PXP", true)

	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	assert.NoError(t, err)

	_, err = us.ValidateOtpEnroll(user.ID, code)
	assert.Error(t, err)
}

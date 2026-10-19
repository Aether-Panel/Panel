package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupActivityTestDB(t *testing.T) *gorm.DB {
	dbPath := filepath.Join(t.TempDir(), "activity_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.ActivityLog{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return db
}

func createActivityTestUser(t *testing.T, db *gorm.DB) *models.User {
	user := &models.User{
		Username:       "activity-test-user",
		Email:          "activity-test@example.com",
		HashedPassword: "dummy_hashed_password",
	}
	if !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	return user
}

func TestActivity_RecordKeepsWhoAndWhere(t *testing.T) {
	db := setupActivityTestDB(t)
	user := createActivityTestUser(t, db)
	activity := &Activity{DB: db}

	err := activity.Record(models.ActivityLogin, &user.ID, user.Username, user.Email, "203.0.113.7", "Mozilla/5.0 Chrome/120.0")
	assert.NoError(t, err)

	entries, total, err := activity.List(10, 1, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, entries, 1)
	assert.Equal(t, models.ActivityLogin, entries[0].Event)
	assert.Equal(t, user.Username, entries[0].Username)
	assert.Equal(t, user.Email, entries[0].Email)
	assert.Equal(t, "203.0.113.7", entries[0].IP)
	assert.NotNil(t, entries[0].UserID)
	assert.Equal(t, user.ID, *entries[0].UserID)
	assert.False(t, entries[0].CreatedAt.IsZero())
}

func TestActivity_ListReturnsNewestFirst(t *testing.T) {
	db := setupActivityTestDB(t)
	activity := &Activity{DB: db}
	now := time.Now()

	for i, event := range []string{models.ActivityLogin, models.ActivityRegister, models.ActivityLoginFailed} {
		entry := &models.ActivityLog{
			Event:     event,
			Username:  "user",
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		}
		assert.NoError(t, db.Create(entry).Error)
	}

	entries, total, err := activity.List(10, 1, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, entries, 3)
	assert.Equal(t, models.ActivityLoginFailed, entries[0].Event)
	assert.Equal(t, models.ActivityLogin, entries[2].Event)
}

func TestActivity_ListFiltersByEventAndPaginates(t *testing.T) {
	db := setupActivityTestDB(t)
	activity := &Activity{DB: db}
	now := time.Now()

	for i := 0; i < 3; i++ {
		assert.NoError(t, db.Create(&models.ActivityLog{
			Event:     models.ActivityRegister,
			Username:  "newbie",
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		}).Error)
	}
	assert.NoError(t, db.Create(&models.ActivityLog{
		Event:     models.ActivityLogin,
		Username:  "veteran",
		CreatedAt: now.Add(time.Hour),
	}).Error)

	registers, total, err := activity.List(10, 1, models.ActivityRegister)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, registers, 3)

	firstPage, total, err := activity.List(1, 1, models.ActivityRegister)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, firstPage, 1)
	assert.Equal(t, "newbie", firstPage[0].Username)

	secondPage, _, err := activity.List(1, 2, models.ActivityRegister)
	assert.NoError(t, err)
	assert.Len(t, secondPage, 1)
	assert.NotEqual(t, firstPage[0].ID, secondPage[0].ID)
}

func TestActivity_ListPrunesPastRetention(t *testing.T) {
	db := setupActivityTestDB(t)
	activity := &Activity{DB: db}

	assert.NoError(t, db.Create(&models.ActivityLog{
		Event:     models.ActivityLogin,
		Username:  "ancient",
		CreatedAt: time.Now().Add(-ActivityRetention - 24*time.Hour),
	}).Error)
	assert.NoError(t, db.Create(&models.ActivityLog{
		Event:     models.ActivityLogin,
		Username:  "recent",
		CreatedAt: time.Now().Add(-time.Hour),
	}).Error)

	entries, total, err := activity.List(10, 1, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, entries, 1)
	assert.Equal(t, "recent", entries[0].Username)

	var remaining int64
	assert.NoError(t, db.Model(&models.ActivityLog{}).Count(&remaining).Error)
	assert.Equal(t, int64(1), remaining)
}

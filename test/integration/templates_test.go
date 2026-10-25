package tests

import (
	"encoding/json"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func TestTemplateAPI(t *testing.T) {
	db, err := database.GetConnection()
	if !assert.NoError(t, err) {
		return
	}

	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	t.Parallel()
	t.Run("GetRepos", func(t *testing.T) {
		t.Parallel()
		testGetTemplateRepos(t, session)
	})

	t.Run("AddTemplateToLocal", func(t *testing.T) {
		testAddTemplateToLocal(t, db, session)
	})

	t.Run("DeleteTemplateFromLocal", func(t *testing.T) {
		testDeleteTemplateFromLocal(t, db, session)
	})
}

func testGetTemplateRepos(t *testing.T, session string) {
	response := CallAPI("GET", "/api/templates", nil, session)
	if !assert.Equal(t, http.StatusOK, response.Code) {
		return
	}
	var templates []*models.TemplateRepo
	err := json.NewDecoder(response.Body).Decode(&templates)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotEmpty(t, templates) {
		return
	}
	hasLocal := false
	for _, v := range templates {
		if v.IsLocal {
			hasLocal = true
		}
	}

	assert.True(t, hasLocal, "No local repo")
}

func testAddTemplateToLocal(t *testing.T, db *gorm.DB, session string) {
	response := CallAPIRaw("PUT", "/api/templates/0/minecraft-vanilla", TemplateData, session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	mo := &models.Template{
		Name: "minecraft-vanilla",
	}
	var count int64
	err := db.Model(mo).Where(mo).Count(&count).Error
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, int64(1), count) {
		return
	}
}

func testDeleteTemplateFromLocal(t *testing.T, db *gorm.DB, session string) {
	mo := &models.Template{
		Name: "minecraft-vanilla",
	}
	var count int64
	err := db.Model(mo).Where(mo).Count(&count).Error
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, int64(1), count) {
		return
	}

	response := CallAPIRaw("DELETE", "/api/templates/0/minecraft-vanilla", TemplateData, session)
	if !assert.Equal(t, http.StatusNoContent, response.Code) {
		return
	}

	err = db.Model(mo).Where(mo).Count(&count).Error
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, int64(0), count) {
		return
	}
}

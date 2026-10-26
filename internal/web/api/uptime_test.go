package api

import (
	"fmt"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"
)

// The uptime overview used to read one page of 1000 servers; the rest vanished from it.
func TestSearchAllServers_ReadsEveryPage(t *testing.T) {
	ps, _ := setupSubServerTest(t)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("srv%02d", i)
		require.NoError(t, ps.DB.Omit(clause.Associations).Create(&models.Server{Identifier: id, Name: id, Type: "generic"}).Error)
	}
	previous := uptimeSearchPage
	uptimeSearchPage = 2
	t.Cleanup(func() { uptimeSearchPage = previous })

	all, total, err := searchAllServers(&services.Server{DB: ps.DB}, services.ServerSearch{})

	assert.NoError(t, err)
	assert.EqualValues(t, 5, total)
	assert.Len(t, all, 5)
}

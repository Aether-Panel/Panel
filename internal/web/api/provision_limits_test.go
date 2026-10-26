package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func TestInjectResourceLimits_ThePlanOverridesTheTemplate(t *testing.T) {
	creation := &models.ServerCreation{}
	creation.Server.Variables = map[string]skypanel.Variable{
		"memory": {Value: 1024, Display: "Memory"},
		"cpu":    {Value: 100},
	}

	injectResourceLimits(creation, 200, 4096, 0)

	assert.Equal(t, 200, creation.Server.Variables["cpu"].Value)
	assert.Equal(t, int64(4096), creation.Server.Variables["memory"].Value)
	assert.Equal(t, "Memory", creation.Server.Variables["memory"].Display, "the template's description stays")
	_, hasDisk := creation.Server.Variables["disk"]
	assert.False(t, hasDisk, "a zero keeps the template's value")
}

func TestResourceLimitData_OnlyTheLimitsThatAreSet(t *testing.T) {
	assert.Empty(t, resourceLimitData(0, 0, 0))
	assert.Equal(t, map[string]interface{}{"disk": int64(10240)}, resourceLimitData(0, 0, 10240))
}

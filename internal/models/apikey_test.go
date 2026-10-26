package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIKeyAllows(t *testing.T) {
	legacy := &APIKey{Permissions: []string{APIKeyAll}}
	unset := &APIKey{}
	limited := &APIKey{Permissions: []string{APIKeyCreate, APIKeySuspend}}

	for _, action := range APIKeyActions {
		assert.True(t, legacy.Allows(action), "a key made before keys could be limited keeps every action: %s", action)
		assert.True(t, unset.Allows(action), "so does one created through the API with no list: %s", action)
	}
	assert.True(t, limited.Allows(APIKeyCreate))
	assert.True(t, limited.Allows(APIKeySuspend))
	assert.False(t, limited.Allows(APIKeyTerminate))
	assert.False(t, limited.Allows(APIKeySSO))
}

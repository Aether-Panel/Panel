package offsite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndpointHost(t *testing.T) {
	for _, tt := range []struct {
		endpoint string
		insecure bool
		host     string
		secure   bool
	}{
		{"s3.us-west-002.backblazeb2.com", false, "s3.us-west-002.backblazeb2.com", true},
		{"minio.lan:9000", true, "minio.lan:9000", false},
		{"https://account.r2.cloudflarestorage.com", false, "account.r2.cloudflarestorage.com", true},
		{"http://127.0.0.1:9000", false, "127.0.0.1:9000", false},
	} {
		host, secure, err := Config{Endpoint: tt.endpoint, Insecure: tt.insecure}.endpointHost()
		assert.NoError(t, err)
		assert.Equal(t, tt.host, host, tt.endpoint)
		assert.Equal(t, tt.secure, secure, tt.endpoint)
	}
}

func TestObjectKeys(t *testing.T) {
	c := Config{Prefix: "aether-backups"}
	assert.Equal(t, "aether-backups/srv1/auto-x.tar.gz", c.objectKey("srv1", "auto-x.tar.gz"))
	assert.Equal(t, "aether-backups/srv1/x.tar.gz", c.objectKey("srv1", "../../x.tar.gz"), "names cannot climb out")
	assert.Equal(t, "aether-backups/srv1/", c.serverPrefix("srv1"))
	assert.Equal(t, "srv1/x.tar.gz", Config{}.objectKey("srv1", "x.tar.gz"))
}

func TestIncompleteConfigIsRefused(t *testing.T) {
	assert.ErrorIs(t, Test(context.Background(), Config{Endpoint: "s3.example.com"}), ErrNotConfigured)
}

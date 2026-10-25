// Package offsite keeps a copy of every backup in an S3-compatible bucket (AWS S3, Backblaze
// B2, Cloudflare R2, Wasabi, MinIO...), so losing the node's disk does not take the backups
// with it.
package offsite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config is where the backups go
type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Prefix    string
	// Insecure talks plain http, for a MinIO on the local network
	Insecure bool
}

var ErrNotConfigured = errors.New("off-site backups are not configured")

// FromSettings reads the configuration; ok is false when off-site backups are off or
// incomplete
func FromSettings() (Config, bool) {
	cfg := Config{
		Endpoint:  strings.TrimSpace(config.BackupsS3Endpoint.Value()),
		Region:    strings.TrimSpace(config.BackupsS3Region.Value()),
		Bucket:    strings.TrimSpace(config.BackupsS3Bucket.Value()),
		AccessKey: strings.TrimSpace(config.BackupsS3AccessKey.Value()),
		SecretKey: config.BackupsS3SecretKey.Value(),
		Prefix:    strings.Trim(strings.TrimSpace(config.BackupsS3Prefix.Value()), "/"),
		Insecure:  config.BackupsS3Insecure.Value(),
	}
	return cfg, config.BackupsS3Enabled.Value() && cfg.complete()
}

func (c Config) complete() bool {
	return c.Endpoint != "" && c.Bucket != "" && c.AccessKey != "" && c.SecretKey != ""
}

// endpointHost accepts "s3.example.com", "s3.example.com:9000" or a full URL
func (c Config) endpointHost() (string, bool, error) {
	endpoint := c.Endpoint
	secure := !c.Insecure
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" {
			return "", false, fmt.Errorf("invalid endpoint %q", endpoint)
		}
		secure = u.Scheme == "https"
		endpoint = u.Host
	}
	return endpoint, secure, nil
}

func (c Config) client() (*minio.Client, error) {
	if !c.complete() {
		return nil, ErrNotConfigured
	}
	host, secure, err := c.endpointHost()
	if err != nil {
		return nil, err
	}
	return minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""),
		Secure: secure,
		Region: c.Region,
	})
}

// objectKey is where a server's backup lives in the bucket
func (c Config) objectKey(serverID, fileName string) string {
	key := path.Join(serverID, path.Base(fileName))
	if c.Prefix != "" {
		key = c.Prefix + "/" + key
	}
	return key
}

func (c Config) serverPrefix(serverID string) string {
	return strings.TrimSuffix(c.objectKey(serverID, "x"), "x")
}

// Upload copies a finished backup to the bucket
func Upload(ctx context.Context, c Config, serverID, fileName, localPath string) error {
	client, err := c.client()
	if err != nil {
		return err
	}
	_, err = client.FPutObject(ctx, c.Bucket, c.objectKey(serverID, fileName), localPath, minio.PutObjectOptions{ContentType: "application/gzip"})
	return err
}

// Download fetches a backup to a local file, writing it under a temporary name first so a
// broken transfer never looks like a complete archive
func Download(ctx context.Context, c Config, serverID, fileName, localPath string) error {
	client, err := c.client()
	if err != nil {
		return err
	}
	partial := localPath + ".download"
	if err := client.FGetObject(ctx, c.Bucket, c.objectKey(serverID, fileName), partial, minio.GetObjectOptions{}); err != nil {
		_ = os.Remove(partial)
		return err
	}
	return os.Rename(partial, localPath)
}

// Open streams a backup straight from the bucket
func Open(ctx context.Context, c Config, serverID, fileName string) (io.ReadCloser, int64, error) {
	client, err := c.client()
	if err != nil {
		return nil, 0, err
	}
	object, err := client.GetObject(ctx, c.Bucket, c.objectKey(serverID, fileName), minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, err
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, 0, err
	}
	return object, info.Size, nil
}

// Exists tells whether the bucket holds the backup
func Exists(ctx context.Context, c Config, serverID, fileName string) bool {
	client, err := c.client()
	if err != nil {
		return false
	}
	_, err = client.StatObject(ctx, c.Bucket, c.objectKey(serverID, fileName), minio.StatObjectOptions{})
	return err == nil
}

// Delete removes a backup from the bucket; a missing object is not an error
func Delete(ctx context.Context, c Config, serverID, fileName string) error {
	client, err := c.client()
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, c.Bucket, c.objectKey(serverID, fileName), minio.RemoveObjectOptions{})
}

// Object is a backup found in the bucket
type Object struct {
	FileName string
	Size     int64
	ModTime  time.Time
}

// List returns the server's backups in the bucket
func List(ctx context.Context, c Config, serverID string) ([]Object, error) {
	client, err := c.client()
	if err != nil {
		return nil, err
	}
	var result []Object
	for object := range client.ListObjects(ctx, c.Bucket, minio.ListObjectsOptions{Prefix: c.serverPrefix(serverID)}) {
		if object.Err != nil {
			return nil, object.Err
		}
		name := path.Base(object.Key)
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		result = append(result, Object{FileName: name, Size: object.Size, ModTime: object.LastModified})
	}
	return result, nil
}

// Test checks the settings by writing, reading and deleting a small object
func Test(ctx context.Context, c Config) error {
	client, err := c.client()
	if err != nil {
		return err
	}
	exists, err := client.BucketExists(ctx, c.Bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the bucket %q does not exist", c.Bucket)
	}
	key := c.objectKey(".aether-test", fmt.Sprintf("probe-%d", time.Now().UnixNano()))
	content := []byte("aether panel off-site backup test")
	if _, err := client.PutObject(ctx, c.Bucket, key, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("could not write to the bucket: %w", err)
	}
	if err := client.RemoveObject(ctx, c.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("could write but not delete in the bucket: %w", err)
	}
	return nil
}

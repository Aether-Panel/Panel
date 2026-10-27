package logging

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readGzip(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	zr, err := gzip.NewReader(file)
	require.NoError(t, err)
	data, err := io.ReadAll(zr)
	require.NoError(t, err)
	return string(data)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestRotator_ArchivesTheDayWhenTheDateChanges(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 25, 23, 59, 0, 0, time.Local)
	r, err := NewRotator(dir, 14)
	require.NoError(t, err)
	r.Wait()
	r.now = func() time.Time { return now }
	r.day = now.Format(dayFormat)

	_, err = r.Write([]byte("sunday\n"))
	require.NoError(t, err)

	now = now.Add(2 * time.Minute)
	_, err = r.Write([]byte("monday\n"))
	require.NoError(t, err)
	r.Wait()
	require.NoError(t, r.Close())

	assert.Equal(t, "monday\n", readFile(t, filepath.Join(dir, logName)))
	assert.Equal(t, "sunday\n", readGzip(t, filepath.Join(dir, "skypanel-2026-10-25.log.gz")))
	assert.NoFileExists(t, filepath.Join(dir, "skypanel-2026-10-25.log"))
}

func TestRotator_ArchivesALeftoverFileFromAnEarlierDay(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, logName)
	require.NoError(t, os.WriteFile(old, []byte("before restart\n"), 0644))
	yesterday := time.Now().AddDate(0, 0, -1)
	require.NoError(t, os.Chtimes(old, yesterday, yesterday))

	r, err := NewRotator(dir, 14)
	require.NoError(t, err)
	r.Wait()
	require.NoError(t, r.Close())

	assert.Equal(t, "", readFile(t, old))
	archive := filepath.Join(dir, "skypanel-"+yesterday.Format(dayFormat)+".log.gz")
	assert.Equal(t, "before restart\n", readGzip(t, archive))
}

func TestRotator_KeepsAppendingToTodaysFile(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, logName)
	require.NoError(t, os.WriteFile(current, []byte("earlier today\n"), 0644))

	r, err := NewRotator(dir, 14)
	require.NoError(t, err)
	_, err = r.Write([]byte("after restart\n"))
	require.NoError(t, err)
	r.Wait()
	require.NoError(t, r.Close())

	assert.Equal(t, "earlier today\nafter restart\n", readFile(t, current))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestRotator_StartsANewFileWhenTheDayIsTooBig(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRotator(dir, 14)
	require.NoError(t, err)
	r.size = maxLogSize - 3

	_, err = r.Write([]byte("overflow\n"))
	require.NoError(t, err)
	r.Wait()
	require.NoError(t, r.Close())

	day := time.Now().Format(dayFormat)
	assert.FileExists(t, filepath.Join(dir, "skypanel-"+day+".log.gz"))
	assert.Equal(t, "overflow\n", readFile(t, filepath.Join(dir, logName)))
}

func TestRotator_NeverOverwritesAnArchive(t *testing.T) {
	dir := t.TempDir()
	r := &Rotator{dir: dir}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skypanel-2026-10-25.log.gz"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skypanel-2026-10-25.1.log"), nil, 0644))

	name, ok := r.freeArchiveName("2026-10-25")
	assert.True(t, ok)
	assert.Equal(t, filepath.Join(dir, "skypanel-2026-10-25.2.log"), name)
}

func TestRotator_DeletesOnlyOldArchives(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "skypanel-2026-09-01.log.gz")
	recent := filepath.Join(dir, "skypanel-2026-10-24.log.gz")
	other := filepath.Join(dir, "something-else.log")
	for _, path := range []string{old, recent, other} {
		require.NoError(t, os.WriteFile(path, nil, 0644))
	}
	longAgo := time.Now().AddDate(0, 0, -30)
	require.NoError(t, os.Chtimes(old, longAgo, longAgo))
	require.NoError(t, os.Chtimes(other, longAgo, longAgo))

	r, err := NewRotator(dir, 14)
	require.NoError(t, err)
	r.Wait()
	require.NoError(t, r.Close())

	assert.NoFileExists(t, old)
	assert.FileExists(t, recent)
	assert.FileExists(t, other)
}

func TestRotator_ZeroRetentionKeepsEverything(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "skypanel-2020-01-01.log.gz")
	require.NoError(t, os.WriteFile(old, nil, 0644))
	longAgo := time.Now().AddDate(-5, 0, 0)
	require.NoError(t, os.Chtimes(old, longAgo, longAgo))

	r, err := NewRotator(dir, 0)
	require.NoError(t, err)
	r.Wait()
	require.NoError(t, r.Close())

	assert.FileExists(t, old)
}

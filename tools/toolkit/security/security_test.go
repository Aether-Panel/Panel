package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanPath_AcceptsRelativePaths(t *testing.T) {
	cases := map[string]string{
		"server.properties":      "server.properties",
		"./a":                    "a",
		"a/b/c.txt":              "a/b/c.txt",
		"a//b/./c":               "a/b/c",
		"  config/app.yml  ":     "config/app.yml",
		`config\app.yml`:         "config/app.yml",
		"world/":                 "world",
		"...":                    "...",
		"a..b/c":                 "a..b/c",
		"~/x":                    "~/x",
		"C":                      "C",
		"name with spaces.txt":   "name with spaces.txt",
		"mods/ünïcødé.jar":       "mods/ünïcødé.jar",
		"plugins/./Essentials/.": "plugins/Essentials",
	}
	for in, want := range cases {
		got, err := CleanPath(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
}

func TestCleanPath_RefusesTraversal(t *testing.T) {
	cases := []string{
		"../x",
		"..",
		"a/../../x",
		"a/../b",
		"a/b/..",
		`..\x`,
		`a\..\..\x`,
		"./../x",
		"/etc/passwd",
		"/",
		"//server/share",
		`\\server\share\x`,
		`C:\x`,
		"C:/x",
		"c:",
		"z:relative",
	}
	for _, in := range cases {
		_, err := CleanPath(in)
		assert.ErrorIs(t, err, ErrOutsideRoot, in)
		_, err = CleanDir(in)
		assert.ErrorIs(t, err, ErrOutsideRoot, in)
	}
}

func TestCleanPath_RefusesInvalidPaths(t *testing.T) {
	deep := strings.Repeat("a/", MaxPathElements) + "b"
	cases := []string{
		"",
		"   ",
		".",
		"./",
		"a\x00b",
		"\x00",
		strings.Repeat("x", MaxPathLength+1),
		deep,
	}
	for _, in := range cases {
		_, err := CleanPath(in)
		assert.ErrorIs(t, err, ErrInvalidPath, "%.40q", in)
	}
}

func TestCleanPath_LimitsAreInclusive(t *testing.T) {
	longest := strings.Repeat("x", MaxPathLength)
	got, err := CleanPath(longest)
	require.NoError(t, err)
	assert.Equal(t, longest, got)

	deepest := strings.TrimSuffix(strings.Repeat("a/", MaxPathElements), "/")
	got, err = CleanPath(deepest)
	require.NoError(t, err)
	assert.Equal(t, deepest, got)
}

func TestCleanPath_ErrorsDoNotEchoTheInput(t *testing.T) {
	_, err := CleanPath("/srv/secret/host/path")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "/srv/secret")
}

func TestCleanDir_EmptyAndDotMeanTheRoot(t *testing.T) {
	for _, in := range []string{"", ".", "./", " . ", "./."} {
		got, err := CleanDir(in)
		require.NoError(t, err, in)
		assert.Equal(t, Root, got, in)
		assert.True(t, IsRoot(got))
	}
	got, err := CleanDir("./plugins/")
	require.NoError(t, err)
	assert.Equal(t, "plugins", got)
	assert.False(t, IsRoot(got))

	_, err = CleanDir("a\x00b")
	assert.ErrorIs(t, err, ErrInvalidPath)
}

func TestProtectedPaths(t *testing.T) {
	t.Cleanup(func() { ProtectedPaths = nil })

	assert.False(t, IsProtected("anything"), "nothing is protected by default")
	assert.False(t, TouchesProtected("anything"))

	ProtectedPaths = []string{"secrets", "config/keys.yml", "../ignored"}
	assert.True(t, IsProtected("secrets"))
	assert.True(t, IsProtected("secrets/a/b"))
	assert.True(t, IsProtected("config/keys.yml"))
	assert.False(t, IsProtected("config"))
	assert.False(t, IsProtected("config/keys.yml.bak"))
	assert.False(t, IsProtected("secrets2"))

	// changing a folder that holds a protected path touches it
	assert.True(t, TouchesProtected("config"))
	assert.True(t, TouchesProtected(Root))
	assert.True(t, TouchesProtected("secrets/x"))
	assert.False(t, TouchesProtected("world"))
}

func TestLooksBinary(t *testing.T) {
	assert.False(t, LooksBinary([]byte("motd=hello\n")))
	assert.False(t, LooksBinary(nil))
	assert.True(t, LooksBinary([]byte{'a', 0, 'b'}))

	late := append([]byte(strings.Repeat("a", Limits.SniffBytes)), 0)
	assert.False(t, LooksBinary(late), "only the start of the file is checked")
}

func TestLimitsMatchTheDesign(t *testing.T) {
	assert.Equal(t, int64(1<<20), Limits.MaxReadBytes)
	assert.Equal(t, int64(1<<20), Limits.MaxWriteBytes)
	assert.Equal(t, int64(1<<20), Limits.MaxEditBytes)
	assert.Equal(t, 1000, Limits.MaxListEntries)
	assert.Equal(t, 8, Limits.MaxSearchDepth)
	assert.Equal(t, 5000, Limits.MaxSearchFiles)
	assert.Equal(t, 200, Limits.MaxSearchResults)
	assert.Equal(t, int64(1<<20), Limits.MaxSearchFileBytes)
	assert.Equal(t, 200, Limits.MaxQueryLength)
}

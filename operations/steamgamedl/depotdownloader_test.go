package steamgamedl

import "testing"

func Test_getLatestVersion(t *testing.T) {
	t.Run("resolved dd versions", func(t *testing.T) {
		got, err := getLatestVersion()
		if err != nil {
			t.Errorf("getLatestVersion() error = %v", err)
			return
		}
		if got == "" {
			t.Errorf("getLatestVersion() got no link")
		}
	})
}

package neoforgedl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// metadataXML is a trimmed copy of the NeoForge maven-metadata.xml so the test
// does not depend on maven.neoforged.net being reachable from the runner.
const metadataXML = `<?xml version="1.0" encoding="UTF-8"?>
<metadata>
  <versioning>
    <latest>21.0.1</latest>
    <release>21.0.1</release>
    <versions>
      <version>20.4.100</version>
      <version>20.4.237</version>
      <version>21.0.1</version>
    </versions>
  </versioning>
</metadata>`

func Test_getLatestForMCVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, metadataXML)
	}))
	defer server.Close()

	orig := metadataURL
	metadataURL = server.URL
	defer func() { metadataURL = orig }()

	type args struct {
		minecraftVersion string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "ValidateParser",
			args: args{minecraftVersion: "1.20.4"},
			want: "20.4.237",
		},
		{
			name:    "NotSupportedVersion",
			args:    args{minecraftVersion: "1.12.2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getLatestForMCVersion(tt.args.minecraftVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestForMCVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("getLatestForMCVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

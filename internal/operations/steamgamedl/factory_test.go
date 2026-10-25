package steamgamedl

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

func TestFactoryReadsTheTemplatesAppId(t *testing.T) {
	// every Steam template writes appId; reading only appID left the download without an app
	for _, key := range []string{"appId", "appID"} {
		op, err := Factory.Create(skypanel.CreateOperation{OperationArgs: map[string]interface{}{key: "896660"}})
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got := op.(SteamGameDl).AppID; got != "896660" {
			t.Fatalf("%s: AppID = %q, want 896660", key, got)
		}
	}
	if _, err := Factory.Create(skypanel.CreateOperation{OperationArgs: map[string]interface{}{}}); err == nil {
		t.Fatal("a step without an app id must fail instead of downloading nothing")
	}
}

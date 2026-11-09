// Package stats sends the anonymous daily usage count behind the "active panels" figure on
// aetherpanel.es. Each report holds an id derived one way from this panel's session key, the
// panel version and how many servers it manages: no addresses, names, users or settings.
// panel.stats.enabled=false turns it off.
package stats

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

const (
	endpoint = "https://stats.aetherpanel.es/v1/ping"
	every    = 24 * time.Hour
	timeout  = 10 * time.Second
)

type report struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Servers int64  `json:"servers"`
}

// Run reports once a day, after a random 5 to 60 minute wait so panels restarted together do
// not all report at once. Failures are dropped: a missing count must never trouble the panel.
func Run() {
	if !config.StatsEnabled.Value() {
		return
	}
	time.Sleep(5*time.Minute + rand.N(55*time.Minute))
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if config.StatsEnabled.Value() {
			if r, err := collect(); err == nil {
				_ = send(&http.Client{Timeout: timeout}, endpoint, r)
			}
		}
		<-ticker.C
	}
}

// instanceID stays the same for as long as the session key does, and the key cannot be
// worked back from it
func instanceID(sessionKey string) string {
	sum := sha256.Sum256([]byte("aether-panel-stats\x00" + sessionKey))
	return hex.EncodeToString(sum[:16])
}

func collect() (report, error) {
	db, err := database.GetConnection()
	if err != nil {
		return report{}, err
	}
	var servers int64
	if err := db.Model(&models.Server{}).Count(&servers).Error; err != nil {
		return report{}, err
	}
	return report{ID: instanceID(config.SessionKey.Value()), Version: skypanel.Version, Servers: servers}, nil
}

func send(client *http.Client, url string, r report) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AetherPanel/"+r.Version)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("stats endpoint answered %d", res.StatusCode)
	}
	return nil
}

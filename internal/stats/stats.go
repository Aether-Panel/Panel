// Package stats sends the anonymous daily usage count behind the "active panels" figure on
// aetherpanel.es. Each report holds an id derived one way from this panel's session key, the
// panel version and how many servers it manages: no addresses, names, users or settings.
// panel.stats.enabled=false turns it off.
package stats

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

const (
	endpoint = "https://stats.aetherpanel.es/v1/ping"
	every    = 24 * time.Hour
	timeout  = 10 * time.Second

	startDelay  = 10 * time.Second
	startJitter = 20 * time.Second
	retry       = 30 * time.Minute
)

type report struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Servers int64  `json:"servers"`
}

// Run reports soon after start, after a random 10 to 30 second wait so panels restarted
// together do not all report at once, and then once a day. A failed report is retried within
// the hour and written to the log, never more: a missing count must never trouble the panel.
func Run() {
	if !config.StatsEnabled.Value() {
		logging.Info.Println("Anonymous usage count is off (panel.stats.enabled=false)")
		return
	}
	time.Sleep(startDelay + jitter(startJitter))
	for {
		wait := every
		if config.StatsEnabled.Value() {
			if err := sendReport(); err != nil {
				logging.Info.Printf("Anonymous usage count not sent, retrying within the hour: %v", err)
				wait = retry + jitter(retry)
			}
		}
		time.Sleep(wait)
	}
}

func sendReport() error {
	r, err := collect()
	if err != nil {
		return err
	}
	return send(&http.Client{Timeout: timeout}, endpoint, r)
}

// jitter is a random wait below limit; without randomness it is no wait at all
func jitter(limit time.Duration) time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(limit)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
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

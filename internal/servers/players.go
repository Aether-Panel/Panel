package servers

import (
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/query"
	"github.com/spf13/cast"
)

// Asking the game who is online: Minecraft answers the server list ping, Steam games answer
// A2S. Answers are cached briefly, since the panel's cards and the history sampler both ask.

const (
	// queryCacheFor keeps one answer for every caller in a short window
	queryCacheFor = 10 * time.Second
	// sourceQueryRetry leaves a server without a declared query type alone for a while after
	// it ignored an A2S query, instead of costing a timeout on every poll
	sourceQueryRetry = 2 * time.Minute
)

type cachedQuery struct {
	result map[string]interface{}
	ok     bool
	at     time.Time
}

var (
	queryCache        = make(map[string]cachedQuery)
	sourceQueryMisses = make(map[string]time.Time)
	queryLock         sync.Mutex
)

// queryPort is the port a game answers queries on: a template's query port variable when it
// has one, otherwise the game port
func queryPort(data map[string]interface{}) int {
	for _, key := range []string{"queryport", "query_port", "queryPort", "steamqueryport", "steam_query_port"} {
		if port := cast.ToInt(data[key]); port > 0 {
			return port
		}
	}
	return cast.ToInt(data["port"])
}

// QueryGame asks the running game for its players. The result is {"minecraft": ...} or
// {"source": ...}; ok is false when the server is off or does not answer.
func (p *Server) QueryGame() (map[string]interface{}, bool) {
	if running, err := p.IsRunning(); err != nil || !running {
		return nil, false
	}

	queryLock.Lock()
	cached, hit := queryCache[p.ID()]
	queryLock.Unlock()
	if hit && time.Since(cached.at) < queryCacheFor {
		return cached.result, cached.ok
	}

	result, ok := p.queryGame()
	queryLock.Lock()
	queryCache[p.ID()] = cachedQuery{result: result, ok: ok, at: time.Now()}
	queryLock.Unlock()
	return result, ok
}

func (p *Server) queryGame() (map[string]interface{}, bool) {
	data := p.DataToMap()
	ip := cast.ToString(data["ip"])

	switch p.Query.Type {
	case "minecraft":
		port := cast.ToInt(data["port"])
		if port <= 0 {
			return nil, false
		}
		res, err := query.Minecraft(ip, port)
		if err != nil {
			return nil, false
		}
		return map[string]interface{}{"minecraft": res}, true
	case "source", "steam", "":
		port := queryPort(data)
		if port <= 0 {
			return nil, false
		}
		guess := p.Query.Type == ""
		if guess {
			queryLock.Lock()
			missed, ok := sourceQueryMisses[p.ID()]
			queryLock.Unlock()
			if ok && time.Since(missed) < sourceQueryRetry {
				return nil, false
			}
		}
		res, err := query.Source(ip, port)
		if err != nil {
			if guess {
				queryLock.Lock()
				sourceQueryMisses[p.ID()] = time.Now()
				queryLock.Unlock()
			}
			return nil, false
		}
		return map[string]interface{}{"source": res}, true
	}
	return nil, false
}

// playerCounts reads how many people are online out of how many slots; bots on Steam
// servers fill slots but are not players
func playerCounts(result map[string]interface{}) (online, slots int, ok bool) {
	if mc, found := result["minecraft"].(query.MinecraftResponse); found {
		return mc.NumPlayers, mc.MaxPlayers, true
	}
	if src, found := result["source"].(query.SourceResponse); found {
		online = src.NumPlayers - src.Bots
		if online < 0 {
			online = 0
		}
		return online, src.MaxPlayers, true
	}
	return 0, 0, false
}

// samplePlayers records the player count of every running server that answers queries
func samplePlayers(now time.Time) {
	for _, p := range GetAll() {
		result, ok := p.QueryGame()
		if !ok {
			continue
		}
		if online, slots, ok := playerCounts(result); ok {
			p.recordPlayers(online, slots, now)
		}
	}
}

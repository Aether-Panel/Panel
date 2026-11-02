package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/daemon"
	"github.com/gin-gonic/gin"
)

// networkHistoryResponse is the traffic of every node together, with how many nodes answered
type networkHistoryResponse struct {
	daemon.NetworkHistory
	Nodes       int `json:"nodes"`
	Unavailable int `json:"unavailable"`
}

const nodeNetworkTimeout = 10 * time.Second

// @Summary Network history of all nodes
// @Description The traffic of every node added up, over the last hour, 6 hours or day
// @Description (range=1h|6h|24h). Each node keeps its own history, a point a minute.
// @Success 200 {object} networkHistoryResponse
// @Tags Nodes
// @Router /api/nodes/network-history [get]
// @Security OAuth2Application[nodes.view]
func getNodesNetworkHistory(c *gin.Context) {
	name := c.DefaultQuery("range", "1h")
	step, ok := daemon.NetworkRanges[name]
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "range must be 1h, 6h or 24h"})
		return
	}
	ns := &services.Node{DB: middleware.GetDatabase(c)}
	nodes, err := ns.GetAll()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	histories := make([]*daemon.NetworkHistory, len(nodes))
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Add(1)
		go func(i int, node *models.Node) {
			defer wg.Done()
			histories[i] = fetchNodeNetwork(c.Request.Context(), ns, node, name)
		}(i, node)
	}
	wg.Wait()

	c.JSON(http.StatusOK, sumNetworkHistories(histories, int64(step/time.Second)))
}

func fetchNodeNetwork(ctx context.Context, ns *services.Node, node *models.Node, rangeName string) *daemon.NetworkHistory {
	ctx, cancel := context.WithTimeout(ctx, nodeNetworkTimeout)
	defer cancel()
	res, err := ns.CallNodeContext(ctx, node, "GET", "/daemon/network/history?range="+url.QueryEscape(rangeName), nil, nil)
	defer utils.CloseResponse(res)
	if err != nil || res.StatusCode != http.StatusOK {
		return nil // an older node without the history, or one that is down
	}
	history := &daemon.NetworkHistory{}
	if json.NewDecoder(res.Body).Decode(history) != nil {
		return nil
	}
	return history
}

// sumNetworkHistories adds the nodes' points up step by step. A node with no point in a step
// (it was down) adds nothing to it; a nil history is a node that did not answer.
func sumNetworkHistories(histories []*daemon.NetworkHistory, step int64) networkHistoryResponse {
	sums := make(map[int64]*daemon.NetworkPoint)
	out := networkHistoryResponse{NetworkHistory: daemon.NetworkHistory{Step: step, Points: []daemon.NetworkPoint{}}, Nodes: len(histories)}
	for _, h := range histories {
		if h == nil || h.Step != step {
			out.Unavailable++
			continue
		}
		for _, p := range h.Points {
			sum := sums[p.T]
			if sum == nil {
				sum = &daemon.NetworkPoint{T: p.T}
				sums[p.T] = sum
			}
			sum.Rx += p.Rx
			sum.Tx += p.Tx
		}
	}
	for _, p := range sums {
		out.Points = append(out.Points, daemon.NetworkPoint{T: p.T, Rx: roundRate(p.Rx), Tx: roundRate(p.Tx)})
	}
	sort.Slice(out.Points, func(i, j int) bool { return out.Points[i].T < out.Points[j].T })
	return out
}

func roundRate(v float64) float64 {
	return math.Round(v*100) / 100
}

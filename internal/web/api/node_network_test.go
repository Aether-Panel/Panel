package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/web/daemon"
	"github.com/stretchr/testify/assert"
)

func TestSumNetworkHistories(t *testing.T) {
	a := &daemon.NetworkHistory{Step: 60, Points: []daemon.NetworkPoint{{T: 60, Rx: 1.5, Tx: 1}, {T: 120, Rx: 2, Tx: 1}}}
	b := &daemon.NetworkHistory{Step: 60, Points: []daemon.NetworkPoint{{T: 120, Rx: 3, Tx: 2}, {T: 180, Rx: 1, Tx: 1}}}

	got := sumNetworkHistories([]*daemon.NetworkHistory{a, b, nil}, 60)

	assert.Equal(t, 3, got.Nodes)
	assert.Equal(t, 1, got.Unavailable, "a node that did not answer")
	assert.Equal(t, []daemon.NetworkPoint{{T: 60, Rx: 1.5, Tx: 1}, {T: 120, Rx: 5, Tx: 3}, {T: 180, Rx: 1, Tx: 1}}, got.Points)

	empty := sumNetworkHistories(nil, 60)
	assert.NotNil(t, empty.Points, "an empty list, not null")
}

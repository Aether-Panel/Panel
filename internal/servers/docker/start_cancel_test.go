package docker

import (
	"context"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func TestCancelStart_StopsTheStartInProgress(t *testing.T) {
	d := &Docker{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.setStartCancel(cancel)

	(&skypanel.Environment{Implementation: d}).CancelStart()

	assert.ErrorIs(t, ctx.Err(), context.Canceled, "a delete while the image downloads cancels the start")
}

func TestCancelStart_WithoutAStartDoesNothing(t *testing.T) {
	d := &Docker{}
	assert.NotPanics(t, d.CancelStartImpl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.setStartCancel(cancel)
	d.setStartCancel(nil)
	d.CancelStartImpl()
	assert.NoError(t, ctx.Err(), "a start that already finished is not cancelled afterwards")
}

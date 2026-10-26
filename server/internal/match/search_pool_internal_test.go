package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchPoolDropsWhenFull(t *testing.T) {
	pool := newSearchPool(1)
	release := make(chan struct{})
	started := make(chan struct{})
	t.Cleanup(func() { close(release) })

	require.True(t, pool.submit(func() {
		close(started)
		<-release
	}))
	<-started

	blocked := func() { <-release }
	for i := range 4 {
		require.True(t, pool.submit(blocked), "submit %d fits the queue", i+2)
	}
	assert.False(t, pool.submit(blocked), "the 6th submit finds one busy worker and a full queue")
}

func TestSearchPoolSurvivesAPanickingJob(t *testing.T) {
	pool := newSearchPool(1)
	done := make(chan struct{})

	require.True(t, pool.submit(func() { panic("search blew up") }))
	require.True(t, pool.submit(func() { close(done) }))

	<-done
}

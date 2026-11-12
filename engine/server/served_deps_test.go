package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine"
)

// A nested client's served schema and host inertness are engine-internal
// metadata: they never cross the wire, so they must survive the JSON round
// trip that snapshots a client's metadata.
func TestClientMetadataKeepsEngineInternalFields(t *testing.T) {
	t.Parallel()
	deps := &core.SchemaBuilder{}
	cloned, err := cloneClientMetadata(&engine.ClientMetadata{ClientID: "nested", ServedDeps: deps, InertAttachables: true})
	require.NoError(t, err)
	require.Same(t, deps, nestedServedDeps(cloned))
	require.True(t, cloned.InertAttachables)
	require.Nil(t, nestedServedDeps(&engine.ClientMetadata{}))
	require.Nil(t, nestedServedDeps(nil))
}

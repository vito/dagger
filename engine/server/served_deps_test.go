package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine"
)

// A nested client's served schema is engine-internal metadata: it never
// crosses the wire, so it must survive the JSON round trip that snapshots a
// client's metadata.
func TestClientMetadataKeepsServedDeps(t *testing.T) {
	t.Parallel()
	deps := &core.SchemaBuilder{}
	cloned, err := cloneClientMetadata(&engine.ClientMetadata{ClientID: "nested", ServedDeps: deps})
	require.NoError(t, err)
	require.Same(t, deps, nestedServedDeps(cloned))
	require.Nil(t, nestedServedDeps(&engine.ClientMetadata{}))
	require.Nil(t, nestedServedDeps(nil))
}

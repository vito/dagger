//go:build !wasm
// +build !wasm

package telemetry

import (
	"runtime"

	"github.com/denisbrodbeck/machineid"
)

func (labels Labels) WithClientLabels(engineVersion string) Labels {
	labels["dagger.io/client.os"] = runtime.GOOS
	labels["dagger.io/client.arch"] = runtime.GOARCH
	labels["dagger.io/client.version"] = engineVersion

	machineID, err := machineid.ProtectedID("dagger")
	if err == nil {
		labels["dagger.io/client.machine_id"] = machineID
	}

	return labels
}

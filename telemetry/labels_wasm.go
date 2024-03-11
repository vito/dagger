package telemetry

import "runtime"

func (labels Labels) WithClientLabels(engineVersion string) Labels {
	labels["dagger.io/client.os"] = runtime.GOOS
	labels["dagger.io/client.arch"] = runtime.GOARCH
	labels["dagger.io/client.version"] = engineVersion
	return labels
}

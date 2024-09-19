package core

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/moby/buildkit/executor"
	dkrspecs "github.com/moby/docker-image-spec/specs-go/v1"

	"dagger.io/dagger/telemetry"
	"github.com/dagger/dagger/engine/buildkit"
	"github.com/dagger/dagger/engine/slog"
)

type portHealthChecker struct {
	bk    *buildkit.Client
	ns    buildkit.Namespaced
	host  string
	ports []Port
}

func newPortHealth(bk *buildkit.Client, ns buildkit.Namespaced, host string, ports []Port) *portHealthChecker {
	return &portHealthChecker{
		bk:    bk,
		ns:    ns,
		host:  host,
		ports: ports,
	}
}

func (d *portHealthChecker) Check(ctx context.Context) (rerr error) {
	ports := make([]Port, 0, len(d.ports))
	portStrs := make([]string, 0, len(d.ports))
	for _, port := range d.ports {
		if !port.ExperimentalSkipHealthcheck {
			ports = append(ports, port)
			portStrs = append(portStrs, fmt.Sprintf("%d/%s", port.Port, port.Protocol.Network()))
		}
	}
	if len(ports) == 0 {
		return nil
	}

	// always show health checks
	ctx, span := Tracer(ctx).Start(ctx, strings.Join(portStrs, " "))
	defer telemetry.End(span, func() error { return rerr })

	slog := slog.SpanLogger(ctx, InstrumentationLibrary)

	dialer := net.Dialer{
		Timeout: time.Second,
	}

	for _, port := range ports {
		retry := backoff.NewExponentialBackOff(
			backoff.WithInitialInterval(100*time.Millisecond),
			backoff.WithMaxInterval(10*time.Second),
		)
		endpoint, err := backoff.RetryWithData(func() (string, error) {
			return buildkit.RunInNetNS(ctx, d.bk, d.ns, func() (string, error) {
				// NB(vito): it's a _little_ silly to dial a UDP network to see that it's
				// up, since it'll be a false positive even if they're not listening yet,
				// but it at least checks that we're able to resolve the container address.
				conn, err := dialer.Dial(
					port.Protocol.Network(),
					net.JoinHostPort(d.host, fmt.Sprintf("%d", port.Port)),
				)
				if err != nil {
					slog.Warn("port not ready", "error", err, "elapsed", retry.GetElapsedTime())
					return "", err
				}

				endpoint := conn.RemoteAddr().String()
				_ = conn.Close()
				return endpoint, nil
			})
		}, backoff.WithContext(retry, ctx))
		if err != nil {
			return fmt.Errorf("checking for port %d/%s: %w", port.Port, port.Protocol.Network(), err)
		}

		slog.Info("port is healthy", "endpoint", endpoint)
	}

	return nil
}

type dockerHealthChecker struct {
	bk  *buildkit.Client
	ctr *buildkit.Container
	hc  *dkrspecs.HealthcheckConfig
}

func newDockerHealth(bk *buildkit.Client, ctr *buildkit.Container, hc *dkrspecs.HealthcheckConfig) *dockerHealthChecker {
	return &dockerHealthChecker{
		bk:  bk,
		ctr: ctr,
		hc:  hc,
	}
}

func (d *dockerHealthChecker) Check(ctx context.Context) (rerr error) {
	timeout := d.hc.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // TODO: inherit?
	}
	interval := d.hc.Interval
	if interval == 0 {
		interval = time.Second // TODO: inherit?
	}

	ctx, span := Tracer(ctx).Start(ctx, "HEALTHCHECK "+strings.Join(d.hc.Test, " "))
	defer telemetry.End(span, func() error { return rerr })
	stdio := telemetry.SpanStdio(ctx, InstrumentationLibrary)

	deadline := time.Now().Add(timeout)
	cmd, args := d.hc.Test[0], d.hc.Test[1:]
	switch cmd {
	case "CMD":
		// nothing to do - args is OK
	case "CMD-SHELL":
		cmd = "CMD"
		args = []string{"/bin/sh", "-c", strings.Join(args, " ")}
	default:
		return fmt.Errorf("unsupported healthcheck CMD: %q", cmd)
	}
	for time.Now().Before(deadline) {
		err := d.bk.Worker.Executor().Exec(ctx, d.ctr.NamespaceID(), executor.ProcessInfo{
			Meta: executor.Meta{
				Args: args,
			},
			Stdout: stdio.Stdout,
			Stderr: stdio.Stderr,
		})
		if err == nil {
			// passed!
			return nil
		}
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("timeout waiting for CMD healthcheck after %s", timeout)
}

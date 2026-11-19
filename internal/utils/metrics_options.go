package utils

import (
	"crypto/tls"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

const (
	// DefaultMetricsPort is the numeric port we use
	DefaultMetricsPort = "8443"

	// DefaultMetricsAddr is used by main.go (binds to everything)
	DefaultMetricsAddr = ":" + DefaultMetricsPort

	// LocalMetricsAddr is used by tests (binds to localhost)
	LocalMetricsAddr = "127.0.0.1:" + DefaultMetricsPort
)

// GetTLSOpts returns the TLS configuration slice for both Webhooks and Metrics.
func GetTLSOpts(enableHTTP2 bool) []func(*tls.Config) {

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}
	return tlsOpts
}

// GetMetricsOptions returns a consistent metrics configuration.
// It applies Authentication and Authorization filters ONLY if secure is true.
func GetMetricsOptions(addr string, secure bool, enableHTTP2 bool) metricsserver.Options {
	options := metricsserver.Options{
		BindAddress:   addr,
		SecureServing: secure,
	}

	if secure {
		// Only enable the guard if the endpoint is secure
		options.FilterProvider = filters.WithAuthenticationAndAuthorization

		// Configure TLS options only for secure endpoints
		options.TLSOpts = GetTLSOpts(enableHTTP2)
	}

	return options
}

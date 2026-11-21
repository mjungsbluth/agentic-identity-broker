package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelmTelemetryDisabled(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--set", "broker.telemetry.enabled=false", "--set-string", "broker.telemetry.exporter.endpoint=otel-collector.monitoring.svc:4317"},
	} {
		configuration, _, _ := credentialHelmConfiguration(t, renderHelmTemplate(t, args...))
		require.NotContains(t, configuration, "telemetry", "disabled telemetry must not render an exporter configuration")
	}
}

// Rendering coverage for US3-S2/S3/S4 and SR-003 in specs/017-opentelemetry-support/spec.md.
func TestHelmTelemetryExporterConfiguration(t *testing.T) {
	for _, tc := range []struct {
		protocol string
		endpoint string
	}{
		{"grpc", "otel-collector.monitoring.svc:4317"},
		{"https", "https://otel-collector.monitoring.svc:4318"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			values := map[string]any{"broker": map[string]any{"telemetry": map[string]any{
				"enabled":     true,
				"serviceName": "broker: production",
				"resourceAttributes": map[string]string{
					"deployment.environment": "production",
					"service.version":        "01.02",
				},
				"exporter": map[string]any{
					"protocol": tc.protocol,
					"endpoint": tc.endpoint,
					"headers": map[string]string{
						"X-Tenant.ID": "platform: primary",
					},
					"timeout":     "15s",
					"compression": "gzip",
				},
			}}}
			configuration, _, _ := credentialHelmConfiguration(t, renderHelmTemplate(t, "-f", credentialHelmValuesFile(t, values)))
			require.Equal(t, map[string]any{
				"enabled":      true,
				"service_name": "broker: production",
				"resource_attributes": map[string]any{
					"deployment.environment": "production",
					"service.version":        "01.02",
				},
				"traces": map[string]any{
					"enabled":       true,
					"sampling_rate": 1,
					"propagators":   []any{"tracecontext", "ottrace", "b3multi", "baggage"},
				},
				"metrics": map[string]any{"enabled": true, "export_interval": "30s"},
				"logs":    map[string]any{"enabled": true},
				"exporter": map[string]any{
					"protocol":    tc.protocol,
					"endpoint":    tc.endpoint,
					"headers":     map[string]any{"X-Tenant.ID": "platform: primary"},
					"timeout":     "15s",
					"compression": "gzip",
					"insecure":    false,
				},
			}, configuration["telemetry"])
		})
	}
}

func TestHelmTelemetrySignalOverrides(t *testing.T) {
	for _, signal := range []string{"traces", "metrics", "logs"} {
		t.Run(signal, func(t *testing.T) {
			configuration, _, _ := credentialHelmConfiguration(t, renderHelmTemplate(t,
				"--set", "broker.telemetry.enabled=true",
				"--set-string", "broker.telemetry.exporter.endpoint=otel-collector.monitoring.svc:4317",
				"--set", "broker.telemetry."+signal+".enabled=false",
				"--set", "broker.telemetry.traces.samplingRate=0",
				"--set-string", "broker.telemetry.metrics.exportInterval=1m",
			))
			telemetry, ok := configuration["telemetry"].(map[string]any)
			require.True(t, ok)
			for _, name := range []string{"traces", "metrics", "logs"} {
				settings, ok := telemetry[name].(map[string]any)
				require.True(t, ok)
				require.Equal(t, name != signal, settings["enabled"], "signal toggles must be independent")
			}
			require.Equal(t, 0, telemetry["traces"].(map[string]any)["sampling_rate"])
			require.Equal(t, "1m", telemetry["metrics"].(map[string]any)["export_interval"])
		})
	}
}

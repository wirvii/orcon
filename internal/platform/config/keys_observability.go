// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

// keysObservability declares the keys of metrics, traces, logs export and the
// optional update check.
func keysObservability() []Key {
	return []Key{
		{
			Path: "observability.metrics.prometheus.enabled", Kind: KindBool, Default: "false",
			Description: text(
				"Serve metrics in the Prometheus format at /metrics on the operations listener (ops.address).",
				"Servir las métricas en formato Prometheus en /metrics, en el escuchador de operaciones (ops.address)."),
		},
		{
			Path: "observability.otlp.endpoint", Kind: KindString,
			Description: text(
				"Address of the OpenTelemetry collector that receives metrics, traces and logs.",
				"Dirección del colector de OpenTelemetry que recibe métricas, trazas y registros."),
		},
		{
			Path: "observability.otlp.protocol", Kind: KindEnum, Enum: []string{"grpc", "http"}, Default: "http",
			Description: text(
				"Protocol used to talk to the collector: http (http/protobuf, the default of the OpenTelemetry exporter specification) or grpc.",
				"Protocolo que se usa para hablar con el colector: http (http/protobuf, el valor por defecto de la especificación de exportadores de OpenTelemetry) o grpc."),
		},
		{
			Path: "observability.otlp.insecure", Kind: KindBool, Default: "false",
			Description: text("Talk to the collector without TLS. Use only on a trusted network.", "Hablar con el colector sin TLS. Úselo solo en una red de confianza."),
		},
		{
			Path: "observability.otlp.headers", Kind: KindMap, Secret: true,
			Description: text(
				"Headers sent to the collector, as name=value pairs. They often carry an API token, so they are a secret.",
				"Encabezados que se envían al colector, como pares nombre=valor. A menudo llevan un token de API, por eso son un secreto."),
		},
		{
			Path: "observability.metrics.otlp.enabled", Kind: KindBool, Default: "false",
			Description: text("Export metrics to the collector.", "Exportar las métricas al colector."),
		},
		{
			Path: "observability.traces.otlp.enabled", Kind: KindBool, Default: "false",
			Description: text("Export traces to the collector.", "Exportar las trazas al colector."),
		},
		{
			Path: "observability.logs.otlp.enabled", Kind: KindBool, Default: "false",
			Description: text("Export logs to the collector.", "Exportar los registros al colector."),
		},
		{
			Path: "observability.traces.sampleRatio", Kind: KindFloat, Default: "0.01", HasRange: true, Min: 0, Max: 1, Reload: ReloadLive,
			Description: text(
				"Fraction of traces that are recorded, from 0 (none) to 1 (all).",
				"Fracción de las trazas que se registran, de 0 (ninguna) a 1 (todas)."),
		},
		{
			Path: "updateCheck.enabled", Kind: KindBool, Default: "false", Reload: ReloadLive,
			Description: text(
				"Check for new Orcon releases. Off by default: Orcon never contacts the project on its own. When on, it reads the public release list and sends nothing about the installation.",
				"Buscar versiones nuevas de Orcon. Apagado por defecto: Orcon nunca contacta al proyecto por su cuenta. Encendido, lee la lista pública de versiones y no envía nada de la instalación."),
		},
		{
			Path: "updateCheck.url", Kind: KindURL, Default: "https://api.github.com/repos/wirvii/orcon/releases", Validate: validateHTTPURL, Reload: ReloadLive,
			Description: text(
				"Address of the public list of releases that the update check reads.",
				"Dirección de la lista pública de versiones que lee la búsqueda de actualizaciones."),
		},
		{
			Path: "updateCheck.interval", Kind: KindDuration, Default: "24h", Reload: ReloadLive,
			Description: text("How often the update check runs.", "Cada cuánto se busca una actualización."),
		},
	}
}

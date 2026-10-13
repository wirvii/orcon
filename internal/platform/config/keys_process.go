// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import "errors"

// keysProcess declares the keys of the process itself: role, data directory,
// public URL, listeners, logging and the memory limit.
func keysProcess() []Key {
	return []Key{
		{
			Path: "role", Kind: KindEnum, Enum: []string{"standalone", "api", "scheduler", "worker"}, Default: "standalone",
			Description: text(
				"The part of Orcon this process runs: standalone (everything, with SQLite), or api, scheduler and worker as separate processes (PostgreSQL).",
				"La parte de Orcon que ejecuta este proceso: standalone (todo, con SQLite), o api, scheduler y worker como procesos separados (PostgreSQL)."),
		},
		{
			Path: "dataDir", Kind: KindPath, DefaultFunc: defaultDataDir,
			DefaultDoc: text(
				"Linux: $XDG_DATA_HOME/orcon or ~/.local/share/orcon; macOS: ~/Library/Application Support/Orcon; Windows: %LOCALAPPDATA%\\Orcon; container image: /var/lib/orcon",
				"Linux: $XDG_DATA_HOME/orcon o ~/.local/share/orcon; macOS: ~/Library/Application Support/Orcon; Windows: %LOCALAPPDATA%\\Orcon; imagen de contenedor: /var/lib/orcon"),
			Description: text(
				"Directory where Orcon keeps its local state: the SQLite database, filesystem blobs, the master key file and temporary files.",
				"Directorio donde Orcon guarda su estado local: la base SQLite, los blobs en disco, el archivo de la clave maestra y los temporales."),
		},
		{
			Path: "publicUrl", Kind: KindURL, Default: "http://127.0.0.1:8080", Validate: validatePublicURL, Roles: rolesServing,
			Description: text(
				"The address users type to reach Orcon. It is used in links, cookies and the first-run URL. Outside localhost it must be https://.",
				"La dirección que escriben las personas para llegar a Orcon. Se usa en enlaces, cookies y en la URL de la primera ejecución. Fuera de localhost debe ser https://."),
		},
		{
			Path: "http.address", Kind: KindAddress, Default: "127.0.0.1:8080", Roles: rolesServing,
			Description: text(
				"Address the HTTP server listens on, as host:port. The default listens on the loopback interface only.",
				"Dirección en la que escucha el servidor HTTP, como host:puerto. El valor por defecto escucha solo en la interfaz de bucle local."),
		},
		{
			Path: "http.tls.certificateFile", Kind: KindPath, Roles: rolesServing,
			Description: text(
				"Path of the PEM certificate chain. With http.tls.keyFile, the server terminates TLS itself.",
				"Ruta de la cadena de certificados en PEM. Junto con http.tls.keyFile, el servidor termina TLS por sí mismo."),
		},
		{
			Path: "http.tls.keyFile", Kind: KindPath, Roles: rolesServing,
			Description: text(
				"Path of the PEM private key of the certificate. The file holds the key; this setting is only its path.",
				"Ruta de la clave privada en PEM del certificado. El archivo contiene la clave; este valor es solo su ruta."),
		},
		{
			Path: "http.trustedProxies", Kind: KindList, Validate: validateCIDR, Roles: rolesServing,
			Description: text(
				"Networks (CIDR) of the reverse proxies whose X-Forwarded-* headers are trusted. Empty trusts none.",
				"Redes (CIDR) de los proxies inversos cuyos encabezados X-Forwarded-* se aceptan. Vacío no confía en ninguno."),
		},
		{
			Path: "http.shutdownTimeout", Kind: KindDuration, Default: "30s", Roles: rolesServing,
			Description: text(
				"How long the process waits for requests and work in flight to finish after SIGTERM before it exits.",
				"Cuánto espera el proceso a que terminen las peticiones y el trabajo en curso tras SIGTERM antes de salir."),
		},
		{
			Path: "ops.address", Kind: KindAddress,
			Description: text(
				"Address of the operations listener, which serves only /healthz, /readyz and /metrics. Empty turns it off. Required by the scheduler and worker roles under an orchestrator.",
				"Dirección del escuchador de operaciones, que sirve solo /healthz, /readyz y /metrics. Vacío lo apaga. Los roles scheduler y worker lo necesitan bajo un orquestador."),
		},
		{
			Path: "log.level", Kind: KindEnum, Enum: []string{"debug", "info", "warn", "error"}, Default: "info", Reload: ReloadLive,
			Description: text(
				"Minimum severity of log records.",
				"Severidad mínima de los registros."),
		},
		{
			Path: "log.format", Kind: KindEnum, Enum: []string{"auto", "json", "text"}, Default: "auto",
			Description: text(
				"Log format: json (one JSON object per line), text (human readable) or auto (text on a terminal, json otherwise).",
				"Formato de los registros: json (un objeto JSON por línea), text (legible) o auto (texto en una terminal, json en otro caso)."),
		},
		{
			Path: "runtime.memoryLimitRatio", Kind: KindFloat, Default: "0.9", HasRange: true, Min: 0.5, Max: 0.95,
			Description: text(
				"Fraction of the container memory limit that becomes the soft memory limit of the Go runtime, when GOMEMLIMIT is not set and the process runs under a cgroup memory limit.",
				"Fracción del límite de memoria del contenedor que se usa como límite blando de memoria del runtime de Go, cuando GOMEMLIMIT no está fijado y el proceso corre bajo un límite de memoria de cgroup."),
		},
	}
}

func defaultDataDir(env Platform, _ Lookup) (string, error) {
	switch env.GOOS {
	case "windows":
		if dir := env.Getenv("LOCALAPPDATA"); dir != "" {
			return join(env.GOOS, dir, "Orcon"), nil
		}
		return "", errors.New("LOCALAPPDATA is not set; set dataDir")
	case "darwin":
		if home := env.Getenv("HOME"); home != "" {
			return join(env.GOOS, home, "Library", "Application Support", "Orcon"), nil
		}
		return "", errors.New("HOME is not set; set dataDir")
	default:
		if dir := env.Getenv("XDG_DATA_HOME"); dir != "" {
			return join(env.GOOS, dir, "orcon"), nil
		}
		if home := env.Getenv("HOME"); home != "" {
			return join(env.GOOS, home, ".local", "share", "orcon"), nil
		}
		return "", errors.New("HOME is not set; set dataDir")
	}
}

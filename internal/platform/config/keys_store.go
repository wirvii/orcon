// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

// keysStore declares the keys of the database and of blob storage.
func keysStore() []Key {
	return []Key{
		{
			Path: "store.driver", Kind: KindEnum, Enum: []string{"sqlite", "postgres"}, Default: "sqlite",
			Description: text(
				"The database: sqlite (one process, the standalone role) or postgres (separate roles, several workers).",
				"La base de datos: sqlite (un proceso, el rol standalone) o postgres (roles separados, varios workers)."),
		},
		{
			Path: "store.sqlite.path", Kind: KindPath, DefaultFunc: underDataDir("orcon.db"), DefaultDoc: docUnderDataDir("orcon.db"),
			Roles: []Role{RoleStandalone},
			Description: text(
				"Path of the SQLite database file.",
				"Ruta del archivo de la base de datos SQLite."),
		},
		{
			Path: "store.sqlite.synchronous", Kind: KindEnum, Enum: []string{"FULL", "NORMAL"}, Default: "FULL",
			Roles: []Role{RoleStandalone},
			Description: text(
				"SQLite synchronous mode. FULL never loses a committed transaction on power loss; NORMAL is faster and can lose the last transactions.",
				"Modo síncrono de SQLite. FULL no pierde una transacción confirmada si se va la luz; NORMAL es más rápido y puede perder las últimas transacciones."),
		},
		{
			Path: "store.postgres.dsn", Kind: KindString, Secret: true,
			Description: text(
				"PostgreSQL connection string (postgres://user:password@host:5432/dbname?sslmode=verify-full). It contains a password, so it is a secret.",
				"Cadena de conexión de PostgreSQL (postgres://usuario:clave@host:5432/base?sslmode=verify-full). Contiene una contraseña, por eso es un secreto."),
		},
		{
			Path: "store.postgres.maxConnections", Kind: KindInt, Default: "20", HasRange: true, Min: 1, Max: 1000,
			Description: text(
				"Maximum number of connections this process opens to PostgreSQL.",
				"Número máximo de conexiones que este proceso abre a PostgreSQL."),
		},
		{
			Path: "store.postgres.declaredCapacity", Kind: KindBytes, Reload: ReloadLive,
			Description: text(
				"Disk capacity of the database, for example 500GiB, used to warn before it fills. Empty means Orcon does not know it.",
				"Capacidad en disco de la base, por ejemplo 500GiB, que se usa para avisar antes de que se llene. Vacío significa que Orcon no la conoce."),
		},
		{
			Path: "store.migrations.auto", Kind: KindBool, Default: "true",
			Description: text(
				"Apply pending database migrations when the process starts, when that is safe for the processes already running. Turn it off to migrate with orcon store schema apply.",
				"Aplicar las migraciones pendientes de la base al arrancar, cuando es seguro para los procesos que ya corren. Apáguelo para migrar con orcon store schema apply."),
		},
		{
			Path: "blobs.backend", Kind: KindEnum, Enum: []string{"filesystem", "postgres", "s3"}, DefaultFunc: defaultBlobBackend,
			DefaultDoc: text("filesystem with store.driver sqlite; postgres with store.driver postgres", "filesystem con store.driver sqlite; postgres con store.driver postgres"),
			Description: text(
				"Where large payloads (blobs) are stored: the file system, the PostgreSQL database, or S3-compatible object storage.",
				"Dónde se guardan los datos grandes (blobs): el sistema de archivos, la base PostgreSQL o un almacenamiento de objetos compatible con S3."),
		},
		{
			Path: "blobs.filesystem.path", Kind: KindPath, DefaultFunc: underDataDir("blobs"), DefaultDoc: docUnderDataDir("blobs"),
			Description: text(
				"Directory of the filesystem blob store. With several processes it must be a path they all share.",
				"Directorio del almacén de blobs en disco. Con varios procesos debe ser una ruta que todos compartan."),
		},
		{
			Path: "blobs.s3.bucket", Kind: KindString,
			Description: text("Bucket of the S3 blob store.", "Bucket del almacén de blobs en S3."),
		},
		{
			Path: "blobs.s3.prefix", Kind: KindString,
			Description: text("Key prefix inside the bucket.", "Prefijo de las claves dentro del bucket."),
		},
		{
			Path: "blobs.s3.endpoint", Kind: KindURL,
			Description: text(
				"Endpoint of an S3-compatible service. Empty uses Amazon S3.",
				"Endpoint de un servicio compatible con S3. Vacío usa Amazon S3."),
		},
		{
			Path: "blobs.s3.region", Kind: KindString,
			Description: text("Region of the bucket.", "Región del bucket."),
		},
		{
			Path: "blobs.s3.forcePathStyle", Kind: KindBool, Default: "false",
			Description: text(
				"Address the bucket in the URL path instead of the host name. Needed by some S3-compatible services.",
				"Direccionar el bucket en la ruta de la URL en vez del nombre de host. Lo necesitan algunos servicios compatibles con S3."),
		},
		{
			Path: "blobs.s3.useAmbientIdentity", Kind: KindBool, Default: "false",
			Description: text(
				"Use the cloud identity of the server (instance role, workload identity) instead of access keys.",
				"Usar la identidad de nube del servidor (rol de instancia, identidad de carga de trabajo) en vez de claves de acceso."),
		},
		{
			Path: "blobs.s3.accessKeyId", Kind: KindString, Secret: true,
			Description: text("Access key id for the S3 blob store.", "Id de la clave de acceso del almacén de blobs en S3."),
		},
		{
			Path: "blobs.s3.secretAccessKey", Kind: KindString, Secret: true,
			Description: text("Secret access key for the S3 blob store.", "Clave secreta de acceso del almacén de blobs en S3."),
		},
	}
}

func defaultBlobBackend(_ Platform, get Lookup) (string, error) {
	if get("store.driver") == "postgres" {
		return "postgres", nil
	}
	return "filesystem", nil
}

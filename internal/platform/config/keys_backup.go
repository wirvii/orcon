// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

// keysBackup declares the keys of backups and their verification (SPEC-015 R-BAK-*).
func keysBackup() []Key {
	return []Key{
		{
			Path: "backup.schedule", Kind: KindString, Validate: validateCron, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Cron expression, in UTC, of the automatic backups. Empty means no scheduled backups.",
				"Expresión cron, en UTC, de los respaldos automáticos. Vacío significa sin respaldos programados."),
		},
		{
			Path: "backup.target", Kind: KindString, Validate: validateBackupTarget, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Where data backups are written: file:///absolute/path or s3://bucket/prefix.",
				"Dónde se escriben los respaldos de datos: file:///ruta/absoluta o s3://bucket/prefijo."),
		},
		{
			Path: "backup.s3.endpoint", Kind: KindURL, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Endpoint of an S3-compatible service for backups. Empty uses Amazon S3.", "Endpoint de un servicio compatible con S3 para los respaldos. Vacío usa Amazon S3."),
		},
		{
			Path: "backup.s3.region", Kind: KindString, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Region of the backup bucket.", "Región del bucket de respaldos."),
		},
		{
			Path: "backup.s3.forcePathStyle", Kind: KindBool, Default: "false", Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Address the backup bucket in the URL path instead of the host name.", "Direccionar el bucket de respaldos en la ruta de la URL en vez del nombre de host."),
		},
		{
			Path: "backup.s3.useAmbientIdentity", Kind: KindBool, Default: "false", Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Use the cloud identity of the server to write backups.", "Usar la identidad de nube del servidor para escribir los respaldos."),
		},
		{
			Path: "backup.s3.serverSideEncryption", Kind: KindString, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Server-side encryption mode to request from the object store, for example aws:kms. Empty requests none.", "Modo de cifrado del lado del servidor que se pide al almacén de objetos, por ejemplo aws:kms. Vacío no pide ninguno."),
		},
		{
			Path: "backup.s3.accessKeyId", Kind: KindString, Secret: true, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Access key id for the backup bucket.", "Id de la clave de acceso del bucket de respaldos."),
		},
		{
			Path: "backup.s3.secretAccessKey", Kind: KindString, Secret: true, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Secret access key for the backup bucket.", "Clave secreta de acceso del bucket de respaldos."),
		},
		{
			Path: "backup.keys.target", Kind: KindString, Validate: validateBackupTarget, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Where the key artifact of a backup is written. It must be a different directory or bucket than backup.target, so data and keys are never stored together.",
				"Dónde se escribe el artefacto de claves de un respaldo. Debe ser otro directorio o bucket distinto de backup.target, para que datos y claves nunca se guarden juntos."),
		},
		{
			Path: "backup.keys.recipients", Kind: KindList, Validate: validateAgeRecipient, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Public keys (age1...) that can read the key artifact. Without recipients and with the file or env provider, backups contain no key material.",
				"Claves públicas (age1...) que pueden leer el artefacto de claves. Sin destinatarios y con el proveedor file o env, los respaldos no contienen material de claves."),
		},
		{
			Path: "backup.retention.daily", Kind: KindInt, Default: "7", HasRange: true, Min: 0, Max: 3650, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Number of days for which the last backup of each day is kept.", "Número de días de los que se conserva el último respaldo de cada día."),
		},
		{
			Path: "backup.retention.weekly", Kind: KindInt, Default: "4", HasRange: true, Min: 0, Max: 520, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Number of ISO weeks for which the last backup of each week is kept.", "Número de semanas ISO de las que se conserva el último respaldo de cada semana."),
		},
		{
			Path: "backup.retention.monthly", Kind: KindInt, Default: "0", HasRange: true, Min: 0, Max: 240, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Number of months for which the last backup of each month is kept.", "Número de meses de los que se conserva el último respaldo de cada mes."),
		},
		{
			Path: "backup.spoolDirectory", Kind: KindPath, DefaultFunc: underDataDir("tmp"), DefaultDoc: docUnderDataDir("tmp"), Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Local directory where a backup is staged before it is uploaded. It needs free space of about the size of the database.",
				"Directorio local donde se prepara un respaldo antes de subirlo. Necesita espacio libre de aproximadamente el tamaño de la base."),
		},
		{
			Path: "backup.postgres.parallelism", Kind: KindInt, Default: "1", HasRange: true, Min: 1, Max: 8, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text("Number of tables copied at the same time when backing up PostgreSQL.", "Número de tablas que se copian a la vez al respaldar PostgreSQL."),
		},
		{
			Path: "backup.beforeUpgrade", Kind: KindBool, DefaultFunc: defaultBackupBeforeUpgrade,
			DefaultDoc: text("true with store.driver sqlite; false with store.driver postgres", "true con store.driver sqlite; false con store.driver postgres"),
			Roles:      rolesScheduler,
			Description: text(
				"Take a local backup before a startup migration changes the schema.",
				"Hacer un respaldo local antes de que una migración al arrancar cambie el esquema."),
		},
		{
			Path: "backup.verify.schedule", Kind: KindString, Validate: validateCron, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Cron expression, in UTC, of the automatic verification of the latest backup. Empty means no scheduled verification.",
				"Expresión cron, en UTC, de la verificación automática del último respaldo. Vacío significa sin verificación programada."),
		},
		{
			Path: "backup.verify.postgres.dsn", Kind: KindString, Secret: true, Reload: ReloadLive, Roles: rolesScheduler,
			Description: text(
				"Connection string of a scratch PostgreSQL database where backups are restored to verify them. Orcon empties it before and after. Without it, PostgreSQL backups are verified only at archive level.",
				"Cadena de conexión de una base PostgreSQL de pruebas donde se restauran los respaldos para verificarlos. Orcon la vacía antes y después. Sin ella, los respaldos de PostgreSQL se verifican solo a nivel de archivo."),
		},
	}
}

func defaultBackupBeforeUpgrade(_ Platform, get Lookup) (string, error) {
	if get("store.driver") == "postgres" {
		return "false", nil
	}
	return "true", nil
}

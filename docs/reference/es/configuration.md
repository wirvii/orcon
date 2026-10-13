<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Generado por orcon docs generate a partir del esquema de configuración. No lo edite a mano. -->
# Referencia de configuración

Orcon lee su configuración de cuatro lugares, del más fuerte al más débil: las banderas de la línea de órdenes, las variables de entorno `ORCON_*`, el archivo YAML indicado con `--config` (o `ORCON_CONFIG`) y los valores por defecto. Cada clave se resuelve por separado.

- Las claves del YAML van en camelCase; una clave desconocida es un error.
- El nombre de la variable de entorno es `ORCON_` más la ruta de la clave en MAYÚSCULAS_CON_GUION_BAJO (`store.postgres.maxConnections` se convierte en `ORCON_STORE_POSTGRES_MAX_CONNECTIONS`). Excepciones: `ORCON_PUBLIC_URL`, `ORCON_MASTER_KEY`, `ORCON_MASTER_KEY_BACKUP_ACK`. Las listas se separan con comas. Una variable `ORCON_*` desconocida es un aviso, porque el cliente de línea de órdenes comparte el prefijo.
- Las banderas son la ruta de la clave en kebab-case (`--store-postgres-max-connections`).
- Los secretos se pueden leer desde archivos: `dsnFile` en el YAML, `ORCON_STORE_POSTGRES_DSN_FILE` en el entorno, `--store-postgres-dsn-file` como bandera. Los valores secretos nunca se aceptan como bandera. Se quita un único salto de línea final del archivo. Dar un secreto y su variante de archivo en el mismo nivel es un error.
- Ejecute `orcon config check` para ver cada valor efectivo, de dónde viene y cualquier error. Los secretos se muestran como `[REDACTED]`.
- El archivo de configuración del servidor nunca se llama `orcon.yaml`; ese nombre es de los repositorios de flujos como código.
- Las claves marcadas `en caliente` se releen cuando el proceso recibe `SIGHUP`; las demás necesitan reiniciar.

## Claves

### General

#### `role`

La parte de Orcon que ejecuta este proceso: standalone (todo, con SQLite), o api, scheduler y worker como procesos separados (PostgreSQL).

| | |
|---|---|
| Tipo | uno de `standalone`, `api`, `scheduler`, `worker` |
| Valor por defecto | `standalone` |
| Variable de entorno | `ORCON_ROLE` |
| Bandera | `--role` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `dataDir`

Directorio donde Orcon guarda su estado local: la base SQLite, los blobs en disco, el archivo de la clave maestra y los temporales.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | Linux: $XDG_DATA_HOME/orcon o ~/.local/share/orcon; macOS: ~/Library/Application Support/Orcon; Windows: %LOCALAPPDATA%\Orcon; imagen de contenedor: /var/lib/orcon |
| Variable de entorno | `ORCON_DATA_DIR` |
| Bandera | `--data-dir` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `publicUrl`

La dirección que escriben las personas para llegar a Orcon. Se usa en enlaces, cookies y en la URL de la primera ejecución. Fuera de localhost debe ser https://.

| | |
|---|---|
| Tipo | URL |
| Valor por defecto | `http://127.0.0.1:8080` |
| Variable de entorno | `ORCON_PUBLIC_URL` |
| Bandera | `--public-url` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api` |

### `http`

#### `http.address`

Dirección en la que escucha el servidor HTTP, como host:puerto. El valor por defecto escucha solo en la interfaz de bucle local.

| | |
|---|---|
| Tipo | host:puerto |
| Valor por defecto | `127.0.0.1:8080` |
| Variable de entorno | `ORCON_HTTP_ADDRESS` |
| Bandera | `--http-address` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api` |

#### `http.tls.certificateFile`

Ruta de la cadena de certificados en PEM. Junto con http.tls.keyFile, el servidor termina TLS por sí mismo.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_HTTP_TLS_CERTIFICATE_FILE` |
| Bandera | `--http-tls-certificate-file` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api` |

#### `http.tls.keyFile`

Ruta de la clave privada en PEM del certificado. El archivo contiene la clave; este valor es solo su ruta.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_HTTP_TLS_KEY_FILE` |
| Bandera | `--http-tls-key-file` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api` |

#### `http.trustedProxies`

Redes (CIDR) de los proxies inversos cuyos encabezados X-Forwarded-* se aceptan. Vacío no confía en ninguno.

| | |
|---|---|
| Tipo | lista |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_HTTP_TRUSTED_PROXIES` |
| Bandera | `--http-trusted-proxies` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api` |

#### `http.shutdownTimeout`

Cuánto espera el proceso a que terminen las peticiones y el trabajo en curso tras SIGTERM antes de salir.

| | |
|---|---|
| Tipo | duración |
| Valor por defecto | `30s` |
| Variable de entorno | `ORCON_HTTP_SHUTDOWN_TIMEOUT` |
| Bandera | `--http-shutdown-timeout` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api` |

### `ops`

#### `ops.address`

Dirección del escuchador de operaciones, que sirve solo /healthz, /readyz y /metrics. Vacío lo apaga. Los roles scheduler y worker lo necesitan bajo un orquestador.

| | |
|---|---|
| Tipo | host:puerto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_OPS_ADDRESS` |
| Bandera | `--ops-address` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

### `log`

#### `log.level`

Severidad mínima de los registros.

| | |
|---|---|
| Tipo | uno de `debug`, `info`, `warn`, `error` |
| Valor por defecto | `info` |
| Variable de entorno | `ORCON_LOG_LEVEL` |
| Bandera | `--log-level` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | todos |

#### `log.format`

Formato de los registros: json (un objeto JSON por línea), text (legible) o auto (texto en una terminal, json en otro caso).

| | |
|---|---|
| Tipo | uno de `auto`, `json`, `text` |
| Valor por defecto | `auto` |
| Variable de entorno | `ORCON_LOG_FORMAT` |
| Bandera | `--log-format` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

### `runtime`

#### `runtime.memoryLimitRatio`

Fracción del límite de memoria del contenedor que se usa como límite blando de memoria del runtime de Go, cuando GOMEMLIMIT no está fijado y el proceso corre bajo un límite de memoria de cgroup.

| | |
|---|---|
| Tipo | número, de 0.5 a 0.95 |
| Valor por defecto | `0.9` |
| Variable de entorno | `ORCON_RUNTIME_MEMORY_LIMIT_RATIO` |
| Bandera | `--runtime-memory-limit-ratio` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

### `expr`

#### `expr.sandboxSlots`

Llamadas del sandbox de expresiones que pueden ejecutar código invitado a la vez, de 1 a GOMAXPROCS - 1. 0 significa GOMAXPROCS - 1. El servidor rechaza al arrancar un valor mayor que GOMAXPROCS - 1.

| | |
|---|---|
| Tipo | entero, de 0 a 1024 |
| Valor por defecto | `0` |
| Variable de entorno | `ORCON_EXPR_SANDBOX_SLOTS` |
| Bandera | `--expr-sandbox-slots` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api`, `worker` |

#### `expr.maxAbandoned`

Número máximo de evaluaciones de expresiones abandonadas al vencer su plazo que pueden seguir ejecutándose. Al llegar al límite, las llamadas nuevas al sandbox fallan de inmediato y una saturación prolongada reinicia el proceso.

| | |
|---|---|
| Tipo | entero, de 1 a 64 |
| Valor por defecto | `8` |
| Variable de entorno | `ORCON_EXPR_MAX_ABANDONED` |
| Bandera | `--expr-max-abandoned` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `api`, `worker` |

### `store`

#### `store.driver`

La base de datos: sqlite (un proceso, el rol standalone) o postgres (roles separados, varios workers).

| | |
|---|---|
| Tipo | uno de `sqlite`, `postgres` |
| Valor por defecto | `sqlite` |
| Variable de entorno | `ORCON_STORE_DRIVER` |
| Bandera | `--store-driver` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `store.sqlite.path`

Ruta del archivo de la base de datos SQLite.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | <dataDir>/orcon.db |
| Variable de entorno | `ORCON_STORE_SQLITE_PATH` |
| Bandera | `--store-sqlite-path` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone` |

#### `store.sqlite.synchronous`

Modo síncrono de SQLite. FULL no pierde una transacción confirmada si se va la luz; NORMAL es más rápido y puede perder las últimas transacciones.

| | |
|---|---|
| Tipo | uno de `FULL`, `NORMAL` |
| Valor por defecto | `FULL` |
| Variable de entorno | `ORCON_STORE_SQLITE_SYNCHRONOUS` |
| Bandera | `--store-sqlite-synchronous` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone` |

#### `store.postgres.dsn`

Cadena de conexión de PostgreSQL (postgres://usuario:clave@host:5432/base?sslmode=verify-full). Contiene una contraseña, por eso es un secreto.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_STORE_POSTGRES_DSN` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |
| Secreto | sí. Variante de archivo: YAML `store.postgres.dsnFile`, entorno `ORCON_STORE_POSTGRES_DSN_FILE`, bandera `--store-postgres-dsn-file`. |

#### `store.postgres.maxConnections`

Número máximo de conexiones que este proceso abre a PostgreSQL.

| | |
|---|---|
| Tipo | entero, de 1 a 1000 |
| Valor por defecto | `20` |
| Variable de entorno | `ORCON_STORE_POSTGRES_MAX_CONNECTIONS` |
| Bandera | `--store-postgres-max-connections` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `store.postgres.declaredCapacity`

Capacidad en disco de la base, por ejemplo 500GiB, que se usa para avisar antes de que se llene. Vacío significa que Orcon no la conoce.

| | |
|---|---|
| Tipo | bytes |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_STORE_POSTGRES_DECLARED_CAPACITY` |
| Bandera | `--store-postgres-declared-capacity` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | todos |

#### `store.migrations.auto`

Aplicar las migraciones pendientes de la base al arrancar, cuando es seguro para los procesos que ya corren. Apáguelo para migrar con orcon store schema apply.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `true` |
| Variable de entorno | `ORCON_STORE_MIGRATIONS_AUTO` |
| Bandera | `--store-migrations-auto` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

### `blobs`

#### `blobs.backend`

Dónde se guardan los datos grandes (blobs): el sistema de archivos, la base PostgreSQL o un almacenamiento de objetos compatible con S3.

| | |
|---|---|
| Tipo | uno de `filesystem`, `postgres`, `s3` |
| Valor por defecto | filesystem con store.driver sqlite; postgres con store.driver postgres |
| Variable de entorno | `ORCON_BLOBS_BACKEND` |
| Bandera | `--blobs-backend` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.filesystem.path`

Directorio del almacén de blobs en disco. Con varios procesos debe ser una ruta que todos compartan.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | <dataDir>/blobs |
| Variable de entorno | `ORCON_BLOBS_FILESYSTEM_PATH` |
| Bandera | `--blobs-filesystem-path` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.bucket`

Bucket del almacén de blobs en S3.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BLOBS_S3_BUCKET` |
| Bandera | `--blobs-s3-bucket` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.prefix`

Prefijo de las claves dentro del bucket.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BLOBS_S3_PREFIX` |
| Bandera | `--blobs-s3-prefix` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.endpoint`

Endpoint de un servicio compatible con S3. Vacío usa Amazon S3.

| | |
|---|---|
| Tipo | URL |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BLOBS_S3_ENDPOINT` |
| Bandera | `--blobs-s3-endpoint` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.region`

Región del bucket.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BLOBS_S3_REGION` |
| Bandera | `--blobs-s3-region` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.forcePathStyle`

Direccionar el bucket en la ruta de la URL en vez del nombre de host. Lo necesitan algunos servicios compatibles con S3.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_BLOBS_S3_FORCE_PATH_STYLE` |
| Bandera | `--blobs-s3-force-path-style` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.useAmbientIdentity`

Usar la identidad de nube del servidor (rol de instancia, identidad de carga de trabajo) en vez de claves de acceso.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_BLOBS_S3_USE_AMBIENT_IDENTITY` |
| Bandera | `--blobs-s3-use-ambient-identity` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `blobs.s3.accessKeyId`

Id de la clave de acceso del almacén de blobs en S3.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BLOBS_S3_ACCESS_KEY_ID` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |
| Secreto | sí. Variante de archivo: YAML `blobs.s3.accessKeyIdFile`, entorno `ORCON_BLOBS_S3_ACCESS_KEY_ID_FILE`, bandera `--blobs-s3-access-key-id-file`. |

#### `blobs.s3.secretAccessKey`

Clave secreta de acceso del almacén de blobs en S3.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BLOBS_S3_SECRET_ACCESS_KEY` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |
| Secreto | sí. Variante de archivo: YAML `blobs.s3.secretAccessKeyFile`, entorno `ORCON_BLOBS_S3_SECRET_ACCESS_KEY_FILE`, bandera `--blobs-s3-secret-access-key-file`. |

### `keys`

#### `keys.master.provider`

Dónde vive la clave maestra: un archivo, el entorno (no recomendado) o un servicio de gestión de claves.

| | |
|---|---|
| Tipo | uno de `file`, `env`, `awskms`, `gcpkms`, `azurekv`, `vault-transit` |
| Valor por defecto | `file` |
| Variable de entorno | `ORCON_KEYS_MASTER_PROVIDER` |
| Bandera | `--keys-master-provider` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.file.path`

Ruta del archivo de la clave maestra, para el proveedor file.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | <dataDir>/keys/master.key |
| Variable de entorno | `ORCON_KEYS_MASTER_FILE_PATH` |
| Bandera | `--keys-master-file-path` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.key`

La clave maestra en base64, para el proveedor env. Prefiera keys.master.keyFile.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_MASTER_KEY` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |
| Secreto | sí. Variante de archivo: YAML `keys.master.keyFile`, entorno `ORCON_MASTER_KEY_FILE`, bandera `--keys-master-key-file`. |

#### `keys.master.backupAcknowledgement`

Huella de la clave maestra, que confirma que se hizo su respaldo. Completa la ceremonia de la clave sin el asistente web. Una huella equivocada impide arrancar.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_MASTER_KEY_BACKUP_ACK` |
| Bandera | `--keys-master-backup-acknowledgement` |
| Cuándo toma efecto | en el primer arranque de una instancia nueva |
| Roles | todos |

#### `keys.master.awskms.keyArn`

ARN de la clave de AWS KMS.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_AWSKMS_KEY_ARN` |
| Bandera | `--keys-master-awskms-key-arn` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.gcpkms.keyName`

Nombre del recurso de la clave de Google Cloud KMS.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_GCPKMS_KEY_NAME` |
| Bandera | `--keys-master-gcpkms-key-name` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.azurekv.keyId`

Identificador de la clave de Azure Key Vault.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_AZUREKV_KEY_ID` |
| Bandera | `--keys-master-azurekv-key-id` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.vaultTransit.address`

Dirección del servidor Vault.

| | |
|---|---|
| Tipo | URL |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_VAULT_TRANSIT_ADDRESS` |
| Bandera | `--keys-master-vault-transit-address` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.vaultTransit.mount`

Ruta de montaje del motor transit de Vault.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_VAULT_TRANSIT_MOUNT` |
| Bandera | `--keys-master-vault-transit-mount` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.vaultTransit.keyName`

Nombre de la clave transit de Vault.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_VAULT_TRANSIT_KEY_NAME` |
| Bandera | `--keys-master-vault-transit-key-name` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `keys.master.vaultTransit.token`

Token de Vault que se usa para llamar al motor transit.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_MASTER_VAULT_TRANSIT_TOKEN` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |
| Secreto | sí. Variante de archivo: YAML `keys.master.vaultTransit.tokenFile`, entorno `ORCON_KEYS_MASTER_VAULT_TRANSIT_TOKEN_FILE`, bandera `--keys-master-vault-transit-token-file`. |

#### `keys.recoveryPublicKey`

Clave pública (age1...) de una clave de recuperación guardada fuera de línea. Si se fija, cada respaldo también puede descifrarse con su mitad privada.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_KEYS_RECOVERY_PUBLIC_KEY` |
| Bandera | `--keys-recovery-public-key` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

### `setup`

#### `setup.owner.email`

Correo de la cuenta propietaria que se crea en el primer arranque, sin el asistente web. Se ignora cuando la instancia ya está preparada.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_SETUP_OWNER_EMAIL` |
| Bandera | `--setup-owner-email` |
| Cuándo toma efecto | en el primer arranque de una instancia nueva |
| Roles | `standalone`, `api` |

#### `setup.owner.name`

Nombre visible de la cuenta propietaria que se crea en el primer arranque.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_SETUP_OWNER_NAME` |
| Bandera | `--setup-owner-name` |
| Cuándo toma efecto | en el primer arranque de una instancia nueva |
| Roles | `standalone`, `api` |

#### `setup.owner.password`

Contraseña de la cuenta propietaria que se crea en el primer arranque. Quítela cuando la instancia ya esté preparada.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_SETUP_OWNER_PASSWORD` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | en el primer arranque de una instancia nueva |
| Roles | `standalone`, `api` |
| Secreto | sí. Variante de archivo: YAML `setup.owner.passwordFile`, entorno `ORCON_SETUP_OWNER_PASSWORD_FILE`, bandera `--setup-owner-password-file`. |

### `backup`

#### `backup.schedule`

Expresión cron, en UTC, de los respaldos automáticos. Vacío significa sin respaldos programados.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_SCHEDULE` |
| Bandera | `--backup-schedule` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.target`

Dónde se escriben los respaldos de datos: file:///ruta/absoluta o s3://bucket/prefijo.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_TARGET` |
| Bandera | `--backup-target` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.s3.endpoint`

Endpoint de un servicio compatible con S3 para los respaldos. Vacío usa Amazon S3.

| | |
|---|---|
| Tipo | URL |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_S3_ENDPOINT` |
| Bandera | `--backup-s3-endpoint` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.s3.region`

Región del bucket de respaldos.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_S3_REGION` |
| Bandera | `--backup-s3-region` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.s3.forcePathStyle`

Direccionar el bucket de respaldos en la ruta de la URL en vez del nombre de host.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_BACKUP_S3_FORCE_PATH_STYLE` |
| Bandera | `--backup-s3-force-path-style` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.s3.useAmbientIdentity`

Usar la identidad de nube del servidor para escribir los respaldos.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_BACKUP_S3_USE_AMBIENT_IDENTITY` |
| Bandera | `--backup-s3-use-ambient-identity` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.s3.serverSideEncryption`

Modo de cifrado del lado del servidor que se pide al almacén de objetos, por ejemplo aws:kms. Vacío no pide ninguno.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_S3_SERVER_SIDE_ENCRYPTION` |
| Bandera | `--backup-s3-server-side-encryption` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.s3.accessKeyId`

Id de la clave de acceso del bucket de respaldos.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_S3_ACCESS_KEY_ID` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |
| Secreto | sí. Variante de archivo: YAML `backup.s3.accessKeyIdFile`, entorno `ORCON_BACKUP_S3_ACCESS_KEY_ID_FILE`, bandera `--backup-s3-access-key-id-file`. |

#### `backup.s3.secretAccessKey`

Clave secreta de acceso del bucket de respaldos.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_S3_SECRET_ACCESS_KEY` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |
| Secreto | sí. Variante de archivo: YAML `backup.s3.secretAccessKeyFile`, entorno `ORCON_BACKUP_S3_SECRET_ACCESS_KEY_FILE`, bandera `--backup-s3-secret-access-key-file`. |

#### `backup.keys.target`

Dónde se escribe el artefacto de claves de un respaldo. Debe ser otro directorio o bucket distinto de backup.target, para que datos y claves nunca se guarden juntos.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_KEYS_TARGET` |
| Bandera | `--backup-keys-target` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.keys.recipients`

Claves públicas (age1...) que pueden leer el artefacto de claves. Sin destinatarios y con el proveedor file o env, los respaldos no contienen material de claves.

| | |
|---|---|
| Tipo | lista |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_KEYS_RECIPIENTS` |
| Bandera | `--backup-keys-recipients` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.retention.daily`

Número de días de los que se conserva el último respaldo de cada día.

| | |
|---|---|
| Tipo | entero, de 0 a 3650 |
| Valor por defecto | `7` |
| Variable de entorno | `ORCON_BACKUP_RETENTION_DAILY` |
| Bandera | `--backup-retention-daily` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.retention.weekly`

Número de semanas ISO de las que se conserva el último respaldo de cada semana.

| | |
|---|---|
| Tipo | entero, de 0 a 520 |
| Valor por defecto | `4` |
| Variable de entorno | `ORCON_BACKUP_RETENTION_WEEKLY` |
| Bandera | `--backup-retention-weekly` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.retention.monthly`

Número de meses de los que se conserva el último respaldo de cada mes.

| | |
|---|---|
| Tipo | entero, de 0 a 240 |
| Valor por defecto | `0` |
| Variable de entorno | `ORCON_BACKUP_RETENTION_MONTHLY` |
| Bandera | `--backup-retention-monthly` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.spoolDirectory`

Directorio local donde se prepara un respaldo antes de subirlo. Necesita espacio libre de aproximadamente el tamaño de la base.

| | |
|---|---|
| Tipo | ruta |
| Valor por defecto | <dataDir>/tmp |
| Variable de entorno | `ORCON_BACKUP_SPOOL_DIRECTORY` |
| Bandera | `--backup-spool-directory` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.postgres.parallelism`

Número de tablas que se copian a la vez al respaldar PostgreSQL.

| | |
|---|---|
| Tipo | entero, de 1 a 8 |
| Valor por defecto | `1` |
| Variable de entorno | `ORCON_BACKUP_POSTGRES_PARALLELISM` |
| Bandera | `--backup-postgres-parallelism` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.beforeUpgrade`

Hacer un respaldo local antes de que una migración al arrancar cambie el esquema.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | true con store.driver sqlite; false con store.driver postgres |
| Variable de entorno | `ORCON_BACKUP_BEFORE_UPGRADE` |
| Bandera | `--backup-before-upgrade` |
| Cuándo toma efecto | al reiniciar |
| Roles | `standalone`, `scheduler` |

#### `backup.verify.schedule`

Expresión cron, en UTC, de la verificación automática del último respaldo. Vacío significa sin verificación programada.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_VERIFY_SCHEDULE` |
| Bandera | `--backup-verify-schedule` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |

#### `backup.verify.postgres.dsn`

Cadena de conexión de una base PostgreSQL de pruebas donde se restauran los respaldos para verificarlos. Orcon la vacía antes y después. Sin ella, los respaldos de PostgreSQL se verifican solo a nivel de archivo.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_BACKUP_VERIFY_POSTGRES_DSN` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | `standalone`, `scheduler` |
| Secreto | sí. Variante de archivo: YAML `backup.verify.postgres.dsnFile`, entorno `ORCON_BACKUP_VERIFY_POSTGRES_DSN_FILE`, bandera `--backup-verify-postgres-dsn-file`. |

### `observability`

#### `observability.metrics.prometheus.enabled`

Servir las métricas en formato Prometheus en /metrics, en el escuchador de operaciones (ops.address).

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_OBSERVABILITY_METRICS_PROMETHEUS_ENABLED` |
| Bandera | `--observability-metrics-prometheus-enabled` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.otlp.endpoint`

Dirección del colector de OpenTelemetry que recibe métricas, trazas y registros.

| | |
|---|---|
| Tipo | texto |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_OBSERVABILITY_OTLP_ENDPOINT` |
| Bandera | `--observability-otlp-endpoint` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.otlp.protocol`

Protocolo que se usa para hablar con el colector: http (http/protobuf, el valor por defecto de la especificación de exportadores de OpenTelemetry) o grpc.

| | |
|---|---|
| Tipo | uno de `grpc`, `http` |
| Valor por defecto | `http` |
| Variable de entorno | `ORCON_OBSERVABILITY_OTLP_PROTOCOL` |
| Bandera | `--observability-otlp-protocol` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.otlp.insecure`

Hablar con el colector sin TLS. Úselo solo en una red de confianza.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_OBSERVABILITY_OTLP_INSECURE` |
| Bandera | `--observability-otlp-insecure` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.otlp.headers`

Encabezados que se envían al colector, como pares nombre=valor. A menudo llevan un token de API, por eso son un secreto.

| | |
|---|---|
| Tipo | mapa |
| Valor por defecto | ninguno |
| Variable de entorno | `ORCON_OBSERVABILITY_OTLP_HEADERS` |
| Bandera | ninguna (es secreto) |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |
| Secreto | sí. Variante de archivo: YAML `observability.otlp.headersFile`, entorno `ORCON_OBSERVABILITY_OTLP_HEADERS_FILE`, bandera `--observability-otlp-headers-file`. |

#### `observability.metrics.otlp.enabled`

Exportar las métricas al colector.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_OBSERVABILITY_METRICS_OTLP_ENABLED` |
| Bandera | `--observability-metrics-otlp-enabled` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.traces.otlp.enabled`

Exportar las trazas al colector.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_OBSERVABILITY_TRACES_OTLP_ENABLED` |
| Bandera | `--observability-traces-otlp-enabled` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.logs.otlp.enabled`

Exportar los registros al colector.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_OBSERVABILITY_LOGS_OTLP_ENABLED` |
| Bandera | `--observability-logs-otlp-enabled` |
| Cuándo toma efecto | al reiniciar |
| Roles | todos |

#### `observability.traces.sampleRatio`

Fracción de las trazas que se registran, de 0 (ninguna) a 1 (todas).

| | |
|---|---|
| Tipo | número, de 0 a 1 |
| Valor por defecto | `0.01` |
| Variable de entorno | `ORCON_OBSERVABILITY_TRACES_SAMPLE_RATIO` |
| Bandera | `--observability-traces-sample-ratio` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | todos |

### `updateCheck`

#### `updateCheck.enabled`

Buscar versiones nuevas de Orcon. Apagado por defecto: Orcon nunca contacta al proyecto por su cuenta. Encendido, lee la lista pública de versiones y no envía nada de la instalación.

| | |
|---|---|
| Tipo | booleano |
| Valor por defecto | `false` |
| Variable de entorno | `ORCON_UPDATE_CHECK_ENABLED` |
| Bandera | `--update-check-enabled` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | todos |

#### `updateCheck.url`

Dirección de la lista pública de versiones que lee la búsqueda de actualizaciones.

| | |
|---|---|
| Tipo | URL |
| Valor por defecto | `https://api.github.com/repos/wirvii/orcon/releases` |
| Variable de entorno | `ORCON_UPDATE_CHECK_URL` |
| Bandera | `--update-check-url` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | todos |

#### `updateCheck.interval`

Cada cuánto se busca una actualización.

| | |
|---|---|
| Tipo | duración |
| Valor por defecto | `24h` |
| Variable de entorno | `ORCON_UPDATE_CHECK_INTERVAL` |
| Bandera | `--update-check-interval` |
| Cuándo toma efecto | en caliente (con SIGHUP) |
| Roles | todos |


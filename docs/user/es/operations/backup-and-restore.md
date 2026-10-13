---
title: Respaldo y restauración
description: Respalda una instancia de Orcon mientras funciona, comprueba el respaldo y restáuralo de forma segura.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0048
  - ADR-0073
sourceHash: f86e138257f62c1971656e13cb884de6cc6897912fafda73c505db6ba2eec8bd
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Respaldo y restauración

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue `orcon backup` quitará esta nota.

Orcon se respalda a sí mismo mientras funciona, escribe el respaldo cifrado y guarda el material de claves en un lugar distinto al de los datos. Un respaldo de los datos sin las claves no sirve, y las claves sin los datos tampoco.

## Qué contiene un respaldo

Un respaldo se escribe en un destino como tres tipos de archivo.

| Archivo | Dónde | Qué contiene |
| --- | --- | --- |
| `manifest.json` y `manifest.json.mac` | `backup.target` | La descripción del respaldo en texto claro, autenticada con un MAC. Lista la versión, las partes con su SHA-256, los conteos de filas y los avisos. Se escribe al final: un respaldo sin manifiesto no existe para listar, retener ni restaurar. |
| `database.age` y `blobs-NNNN.age` | `backup.target` | Los datos: la base de datos y los blobs guardados en el sistema de archivos, comprimidos con zstd y cifrados con age. Los blobs de archivo se dividen en partes de 1 GiB. |
| `<backupId>.keys.age` y `<backupId>.keys.json` | `backup.keys.target` | El artefacto de claves: la huella, el proveedor y la referencia de la clave maestra y, para claves de archivo o de entorno, el material de la clave cifrado para los destinatarios age que listes en `backup.keys.recipients`. |

Los datos se cifran de modo que la clave maestra pueda abrirlos, y también la clave de recuperación si configuraste `keys.recoveryPublicKey`. El artefacto de claves solo pueden abrirlo las identidades age que tú conserves. La clave maestra en sí nunca se escribe junto a los datos. `backup.keys.target` debe ser un directorio, bucket o prefijo distinto de `backup.target`, y `orcon config check` informa un error si no lo es.

Si usas una clave maestra de archivo o de entorno y no listas destinatarios, el respaldo termina con el aviso `KEY_MATERIAL_NOT_EXPORTED`. En ese caso solo el kit de recuperación que guardaste en la primera ejecución te permite restaurar. El aviso aparece en el manifiesto, la API, las métricas y `orcon config check`.

## Elige un destino y una programación

Define estas claves en tu archivo de configuración (consulta el [resumen de configuración](../configuration/overview.md)).

| Clave | Significado |
| --- | --- |
| `backup.target` | Dónde van los datos: `file:///<ruta>` o `s3://<bucket>/<prefijo>`. |
| `backup.keys.target` | Dónde va el artefacto de claves. Un lugar distinto de `backup.target`. |
| `backup.keys.recipients` | Una lista de claves públicas age (`age1…`) que pueden abrir el artefacto de claves. |
| `backup.schedule` | Una expresión cron en UTC. Vacía significa sin respaldos programados. |
| `backup.retention.daily`, `weekly`, `monthly` | Cuántos días, semanas ISO y meses conservar. Por defecto 7, 4 y 0. |
| `backup.spoolDirectory` | Directorio local de la copia temporal. Por defecto `<dataDir>/tmp`. |

Los destinos S3 usan la API de S3 y aceptan claves estáticas leídas de archivos, o la identidad de la cuenta de nube del servidor en los destinos de instancia. Las subidas usan partes de 64 MiB. Los servidores compatibles con S3 distintos de AWS S3 deberían funcionar, pero no se prueban.

### Retención

La retención conserva el último respaldo correcto de cada uno de los últimos `daily` días UTC, de las últimas `weekly` semanas ISO y de los últimos `monthly` meses. El destino es la fuente de verdad, también después de restaurar. Orcon nunca borra el último respaldo correcto, ni uno que esté usando una verificación o una restauración. Cada borrado se audita como `backup.pruned` y elimina juntos los datos y las claves de ese respaldo. Un objeto que el proveedor haya bloqueado produce un aviso, no un error.

## Crea un respaldo

Mientras la instancia funciona, el rol `scheduler` (o `standalone`) ejecuta el respaldo, uno a la vez por instancia. Inicia uno a demanda con:

```sh
orcon backup create
```

El comando llama a la API (`POST /api/v1/backups`) y necesita el permiso `instance:backups`. Lista e inspecciona los respaldos con:

```sh
orcon backup list
```

Con la instancia detenida, ejecuta el mismo comando directamente sobre los archivos:

```sh
orcon backup create --offline --config <archivo>
```

`<archivo>` es el archivo de configuración de la instancia.

### Qué es consistente, exactamente

- **SQLite.** La parte de la base de datos es el estado de una transacción de lectura, copiada con la API de respaldo en línea de SQLite en un solo paso. No bloquea a los escritores. La copia se escribe en `<dataDir>/tmp`, se comprueba con `PRAGMA quick_check` y se cifra desde allí. Mientras dura la copia, el registro de escritura anticipada (WAL) no se puede compactar más allá de esa lectura, y el espacio temporal necesario es el tamaño de la base de datos. Orcon comprueba que el espacio libre sea al menos 1,1 veces el tamaño de la base y falla antes de empezar si no lo es.
- **PostgreSQL.** La parte de la base de datos es el estado de una sola instantánea `REPEATABLE READ, READ ONLY`. Orcon copia las filas de cada organización dentro de esa instantánea con el contexto de la organización fijado para la transacción, así que la seguridad a nivel de fila sigue vigente y no hace falta ningún rol que la eluda. Una función del motor cuenta las filas en la misma instantánea, y el respaldo falla si los conteos no coinciden con lo copiado. La instantánea se escribe en el directorio local temporal, la transacción termina y solo entonces se sube, de modo que ninguna transacción queda abierta durante una llamada externa. Mientras dura, la instantánea retiene la limpieza de versiones antiguas de filas de PostgreSQL.
- **Blobs.** Todo blob referenciado por una fila confirmada en la instantánea está en el respaldo, con su SHA-256 comprobado al leerlo. Los blobs escritos después de la instantánea no están. Mientras corre un respaldo, el borrado físico de archivos de blobs se aplaza. Con el backend de blobs de PostgreSQL, los blobs forman parte de la instantánea de la base de datos. Con el backend de blobs S3, el respaldo no copia los objetos; guarda su lista (clave, tamaño, SHA-256) y `verify` comprueba que existen. Activa el versionado del bucket y consérvalo al menos tanto como tus respaldos.

Para bases de datos PostgreSQL grandes, usa las herramientas nativas: un respaldo base con archivado de WAL, o `pg_dump` con un rol creado por tu administrador de bases de datos que eluda la seguridad a nivel de fila. `orcon db setup-roles --backup-role` imprime el SQL de ese rol. Estas herramientas no cubren el directorio de blobs de archivo (si lo usas) ni el material de la clave maestra, así que conserva también el respaldo de claves de Orcon. Para restaurar después de una restauración nativa, consulta «Después de restaurar la base de datos por tu cuenta» más abajo.

## Comprueba un respaldo

Un respaldo que no has comprobado es una esperanza. Verifica:

```sh
orcon backup verify <backupId>
```

La verificación comprueba el MAC del manifiesto, la suma de cada parte y el descifrado. En SQLite, además restaura en un archivo temporal y ejecuta `PRAGMA integrity_check` y `PRAGMA foreign_key_check`, compara las sumas del esquema con las de esa versión, compara los conteos de filas con el manifiesto, comprueba los testigos de claves y el SHA-256 de cada blob. Ese es el nivel `FULL`. En PostgreSQL, define `backup.verify.postgres.dsn` con una base de datos de pruebas que Orcon vacía antes y después, y la verificación hace lo mismo allí. Sin ella, la verificación se hace en el nivel `ARCHIVE` y lo dice.

Para comprobar un respaldo desde otra máquina, sin la instancia:

```sh
orcon backup verify --from <uri> --offline
```

`<uri>` es la ubicación del respaldo. Define `backup.verify.schedule` para verificar el último respaldo según una programación. Los resultados se registran, se auditan como `backup.verified` y se exponen como métricas; las alertas `OrconBackupMissing`, `OrconBackupFailed` y `OrconBackupNotVerified` las usan.

## Restaura

Restaurar es un comando local. Nunca pasa por la API. Ejecútalo en la máquina donde quieres la instancia.

```sh
orcon backup restore --from <uri> --keys <uri> --identity <archivo>
```

`--from` es la ubicación del respaldo, `--keys` es la ubicación del artefacto de claves e `--identity` es un archivo de identidad age que puede abrirlo. `--keys` e `--identity` son opcionales.

Antes de cambiar nada, el comando muestra el id del respaldo, su fecha, la instancia de origen, la versión del esquema y el destino, y te pide escribir el id del respaldo. En un script, pasa `--confirm <backupId>`.

Requisitos:

- **SQLite.** El directorio de datos está bloqueado y la base de datos de destino no existe. Con `--replace`, la base existente se mueve a `<ruta>.pre-restore-<fecha>`. Nunca se borra.
- **PostgreSQL.** La base de datos de destino está vacía y no hay ningún proceso en marcha.
- El binario debe contener migraciones hasta la versión del esquema del respaldo. Si no las contiene, el comando sale con el código 4 y nombra la versión mínima.

La restauración aplica el esquema, las filas y los blobs, comprueba los conteos de filas y los testigos de claves, y devuelve a ejecutable, con una época nueva, las fichas que estaban arrendadas. En PostgreSQL esto ocurre en una sola transacción; en SQLite usa un renombrado atómico.

### La instancia arranca retenida

Después de restaurar, la instancia queda retenida. No reclama trabajo ni dispara disparadores hasta que un administrador de instancia libere la retención. Los datos restaurados no conocen los efectos enviados después de tomar el respaldo, y Orcon no afirma enviar un efecto externo exactamente una vez a través de una restauración; la retención da a una persona la oportunidad de decidir. Cuando la instancia arranca, genera la alerta operativa `INSTANCE_RESTORED` para los administradores de instancia, y la interfaz web muestra una banda persistente con la acción «Release hold».

Libera la retención cuando estés listo. Necesita el permiso `instance:maintenance`, pide confirmación y se audita.

```sh
orcon instance hold release
```

Después de liberarla, las operaciones externas que se estaban enviando cuando se tomó el respaldo siguen la recuperación normal del diario de operaciones: se reconcilian, o pasan a ser operaciones en duda que una persona resuelve. Nada se envía de nuevo sin pasar por ese camino. Para simulacros automatizados de recuperación, `orcon backup restore --release-hold` libera la retención como parte de la restauración y se audita igual.

### Cadena de auditoría después de una restauración

Las anclas de auditoría se guardan fuera de la base de datos para detectar truncamientos. Una restauración mueve las anclas posteriores al respaldo a `<dataDir>/anchors/pre-restore-<fecha>/` y escribe el evento de auditoría `backup.restored`. El evento registra la última ancla incluida en el respaldo, cuántos eventos de la cadena le faltan al respaldo según las anclas archivadas, y el usuario del sistema operativo y el host que restauraron. `orcon audit verify` reconoce esa discontinuidad declarada y no la informa como manipulación.

### Después de restaurar la base de datos por tu cuenta

Si restauraste la base de datos con las herramientas nativas de PostgreSQL, ejecuta los pasos de restauración de Orcon que vienen después de la copia de datos:

```sh
orcon backup restore --database-restored-externally
```

Esto aplica solo los pasos posteriores: la retención de la instancia, las fichas, el evento de auditoría y las anclas.

## Respalda antes de actualizar

SQLite hace un respaldo local automático antes de migrar. Con PostgreSQL, haz un respaldo antes de cada actualización. Consulta [Actualización y vuelta atrás](upgrade-and-rollback.md).

---
title: Secretos en la configuración
description: Cómo dar a Orcon contraseñas, claves y tokens sin ponerlos en una línea de órdenes ni en un archivo de configuración.
sources:
  - ADR-0083
  - ADR-0042
sourceHash: d6d15a27a0a90cedcf8606eee2f75e035c22231aa9696c8b6e53edbaa4305701
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Secretos en la configuración

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue el esquema de configuración quitará esta nota.

Algunas claves de configuración contienen secretos: una cadena de conexión a la base de datos, una clave de acceso, una contraseña. Orcon trata estas claves de forma distinta a las demás para que un secreto no acabe en una lista de procesos, en el historial del shell o en un archivo que compartes.

Esta página trata de los secretos que Orcon necesita para arrancar. Las credenciales que usan tus flujos para llamar a otros servicios las guarda el propio Orcon y se describen en [Credenciales y almacenes de secretos](../security/credentials-and-secret-stores.md).

## Las reglas

- **Nunca como bandera.** Una clave secreta no acepta su valor en la línea de órdenes, porque el valor sería visible para otros usuarios del equipo. Una bandera secreta existe solo en su forma de archivo.
- **Todo secreto puede venir de un archivo.** Para una clave secreta, Orcon acepta tres formas de archivo, una por cada fuente de configuración:

  | Fuente | Forma | Ejemplo |
  | --- | --- | --- |
  | Archivo YAML | el nombre de la clave seguido de `File` | `dsnFile: /run/secrets/orcon/dsn` |
  | Entorno | el nombre de la variable seguido de `_FILE` | `ORCON_STORE_POSTGRES_DSN_FILE=/run/secrets/orcon/dsn` |
  | Bandera | el nombre de la bandera seguido de `-file` | `--store-postgres-dsn-file /run/secrets/orcon/dsn` |

- **Una forma a la vez.** Dar a la vez un valor y un archivo para la misma clave es un error.
- **Se quita un único salto de línea final** del contenido del archivo, así que un archivo escrito con `echo` funciona.
- **Permisos del archivo.** En Unix, Orcon avisa cuando otros usuarios pueden leer un archivo de secreto. Mantén estos archivos legibles solo por la cuenta que ejecuta Orcon.
- **Ocultos en todas partes.** `orcon config check` muestra un secreto como `[REDACTED]`. Dentro del proceso, los valores secretos se guardan en tipos que se ocultan solos al imprimirse.

## Qué claves son secretas

| Clave | Variable de entorno |
| --- | --- |
| `store.postgres.dsn` | `ORCON_STORE_POSTGRES_DSN` |
| `keys.master.key` (solo con el proveedor `env`) | `ORCON_MASTER_KEY` |
| `keys.master.vaultTransit.token` | `ORCON_KEYS_MASTER_VAULT_TRANSIT_TOKEN` |
| `blobs.s3.accessKeyId` y `blobs.s3.secretAccessKey` | `ORCON_BLOBS_S3_ACCESS_KEY_ID` y `ORCON_BLOBS_S3_SECRET_ACCESS_KEY` |
| `backup.s3.accessKeyId` y `backup.s3.secretAccessKey` | `ORCON_BACKUP_S3_ACCESS_KEY_ID` y `ORCON_BACKUP_S3_SECRET_ACCESS_KEY` |
| `backup.verify.postgres.dsn` | `ORCON_BACKUP_VERIFY_POSTGRES_DSN` |
| `observability.otlp.headers` | `ORCON_OBSERVABILITY_OTLP_HEADERS` |
| `setup.owner.password` | `ORCON_SETUP_OWNER_PASSWORD` |

Cada una tiene también la variable `_FILE`, por ejemplo `ORCON_MASTER_KEY_FILE`. La referencia de configuración generada lista cada clave con su tipo, su valor por defecto y si es secreta.

## Ejemplos

Una cadena de conexión a la base de datos guardada en un archivo:

```sh
export ORCON_STORE_POSTGRES_DSN_FILE=/run/secrets/orcon/dsn
orcon serve --role=api
```

El mismo secreto en el archivo YAML, como una ruta:

```yaml
store:
  driver: postgres
  postgres:
    dsnFile: /run/secrets/orcon/dsn
```

## En contenedores y clústeres

- **Docker Compose.** El ejemplo de PostgreSQL declara los secretos con la función `secrets:` de compose, que los monta como archivos, y los lee con las variables `*_FILE`. Consulta [Instalar con Docker Compose](../install/docker-compose.md).
- **Kubernetes.** El chart de Helm no acepta valores secretos. Refiere los Secrets de Kubernetes por nombre, los monta en `/run/secrets/orcon/` y los pasa con `*_FILE`. Consulta [Instalar en Kubernetes con Helm](../install/kubernetes-helm.md).

## Clave maestra

La clave maestra puede venir de una variable de entorno (`ORCON_MASTER_KEY`), pero es la opción más débil y Orcon avisa de ello. Prefiere un archivo con permisos restrictivos o un servicio de gestión de claves. Consulta [Clave maestra y KMS](master-key-and-kms.md).

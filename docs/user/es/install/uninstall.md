---
title: Desinstalar Orcon
description: Qué conservar y qué quitar cuando retiras de servicio una instalación de Orcon.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0048
sourceHash: e7dbc01d5af67ceaf0a3dc13ce5c20822584fe9faf7dda3de054463d0c886c02
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Desinstalar Orcon

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue los instaladores quitará esta nota. No hay ninguna orden de desinstalación especificada, así que la retirada es manual.

Retirar Orcon significa quitar el programa y decidir después qué hacer con los datos. Decide primero lo segundo, porque parte de los datos no se puede volver a crear.

## Antes de quitar nada

1. Detén todos los procesos de Orcon.
2. Haz un último [respaldo](../operations/backup-and-restore.md) si es posible que necesites los datos otra vez. Guarda el respaldo y su artefacto de claves en lugares distintos.
3. Conserva la clave maestra y el kit de recuperación mientras conserves cualquier respaldo o copia de la base de datos. Sin la clave maestra, los datos cifrados no se pueden leer. Consulta [Clave maestra y KMS](../configuration/master-key-and-kms.md).
4. Si debes conservar el registro de auditoría, expórtalo antes. Las anclas de la cadena de auditoría están en el directorio de datos.

## Qué contiene el directorio de datos

El directorio de datos es `dataDir` (consulta [Instalar el binario único](binary.md) para ver su valor por defecto, y `/var/lib/orcon` en la imagen). Con los valores por defecto contiene:

| Ruta dentro del directorio de datos | Qué contiene |
| --- | --- |
| `orcon.db` | La base de datos SQLite, cuando usas SQLite. |
| `blobs/` | Las cargas guardadas en el sistema de archivos. |
| `keys/master.key` | La clave maestra, con el proveedor `file`. |
| `setup-token` | El código de preparación, hasta que se completa la primera ejecución. |
| `backups/pre-upgrade/` | Los respaldos automáticos hechos antes de una actualización con SQLite. |
| `audit-anchors.log` y `anchors/` | Las anclas del registro de auditoría a prueba de manipulación, y las anclas archivadas tras una restauración. |
| `tmp/` | Archivos temporales, incluido el espacio de trabajo de los respaldos. |

Borrar el directorio de datos borra la base de datos, los blobs y, con el proveedor `file`, la clave maestra. Hazlo solo cuando hayas decidido que los datos ya no hacen falta, o cuando tengas un respaldo y sus claves en otro lugar.

## Quitar el programa

- **Binario único.** Borra el archivo `orcon` del directorio que usó el instalador: `/usr/local/bin` o `~/.local/bin`, o el directorio que indicaste con `--dir`. Si creaste una unidad de systemd, detenla, desactívala y bórrala.
- **Contenedor.** Quita el contenedor. Quita el volumen de datos solo si ya no necesitas los datos.
- **Docker Compose.** Baja el conjunto de servicios. Quita los volúmenes solo si ya no necesitas los datos.
- **Helm.** Ejecuta `helm uninstall` para la versión instalada. Los Secrets que creaste para el chart y tu base de datos PostgreSQL son tuyos y permanecen hasta que los quites.

## PostgreSQL y servicios externos

Orcon no elimina nada que no sea suyo. Si usaste PostgreSQL, almacenamiento S3, un destino de respaldos o un servicio externo de gestión de claves, quita tú esos recursos cuando estés seguro de que ya no hacen falta. Borrar una clave de un servicio de gestión de claves deja ilegibles los datos cifrados con ella.

---
title: Instalar con Docker Compose
description: Parte de los ejemplos oficiales de compose para un solo servicio con SQLite o para PostgreSQL con roles separados.
sources:
  - ADR-0083
  - ADR-0002
sourceHash: 4ca57ea8a47a45568eade31e13a6966bd1647d5aadb027f81955e54e951f34da
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Instalar con Docker Compose

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue los ejemplos de compose quitará esta nota.

Orcon ofrece dos ejemplos de compose. Son puntos de partida para leer y adaptar, no un producto gestionado. Necesitas Docker Engine 24 o posterior con Compose v2.

## Los dos ejemplos

| Ejemplo | Archivo en el repositorio | Qué inicia |
| --- | --- | --- |
| SQLite | `deploy/compose/sqlite/compose.yaml` | Un servicio `standalone`, un volumen de datos, y la configuración y los secretos montados como archivos. |
| PostgreSQL | `deploy/compose/postgres/compose.yaml` | PostgreSQL fijado por digest, un servicio de migración de una sola ejecución, un `api`, un `scheduler` y dos `worker`. Los secretos usan la función `secrets:` de compose y se leen con las variantes `*_FILE` de la configuración. |

## Obtener los archivos

Cada versión publica ambos ejemplos como `orcon-compose_<versión>.tar.gz`. El archivo incluye un `.env` que fija `ORCON_VERSION` a esa versión.

En el repositorio, los ejemplos te exigen definir `ORCON_VERSION` tú mismo. No hay un valor `latest` por defecto, así que un arranque nunca cambia de versión por accidente.

## Iniciar

```sh
docker compose up -d
```

Ejecuta la orden en el directorio del ejemplo que elegiste. Después completa la [primera ejecución](../configuration/first-run.md). Con PostgreSQL, el estado de la primera ejecución y el hash del código de preparación viven en la base de datos, así que da igual qué réplica de `api` responda.

## Comprobar

Pregunta a un proceso si está listo con `orcon health --ready`, ejecutado dentro de su contenedor. Para listar los procesos que están vivos, con su versión y su rol, ejecuta `orcon store schema status`.

## Conviene saber

- El ejemplo de PostgreSQL ejecuta dos workers, así que el trabajo se reparte entre procesos.
- Los servicios `api`, `scheduler` y `worker` necesitan el mismo almacenamiento de blobs. Con PostgreSQL lo normal es guardar los blobs en la base de datos; un backend de sistema de archivos exigiría una ruta compartida por todos los procesos, y `orcon config check` avisa de ello.
- Para actualizar, sigue [Actualización y vuelta atrás](../operations/upgrade-and-rollback.md): cambia `ORCON_VERSION`, ejecuta el plan e inicia la versión nueva.

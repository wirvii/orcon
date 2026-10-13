---
title: Resumen de configuración
description: Cómo lee Orcon sus ajustes y en qué orden, y cómo comprobar el resultado.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
sourceHash: acb4368c9b1075943fded391e5111d4c19f87f724b83fc32bde602f8c7b52561
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Resumen de configuración

Orcon lee sus ajustes de cuatro lugares, del más prioritario al menos: las banderas de la línea de órdenes, las variables de entorno `ORCON_*`, el archivo YAML indicado con `--config` (o `ORCON_CONFIG`) y los valores por defecto incorporados. Cada clave se resuelve por separado.

- Las claves YAML van en camelCase; una clave desconocida es un error.
- El nombre de la variable de entorno es `ORCON_` más la ruta de la clave en UPPER_SNAKE (`store.postgres.maxConnections` → `ORCON_STORE_POSTGRES_MAX_CONNECTIONS`). Excepciones: `ORCON_PUBLIC_URL`, `ORCON_MASTER_KEY`, `ORCON_MASTER_KEY_BACKUP_ACK`.
- Los secretos se pueden leer desde archivos: `dsnFile` en YAML, `ORCON_STORE_POSTGRES_DSN_FILE` en el entorno, `--store-postgres-dsn-file` como bandera. Los valores secretos nunca se aceptan como banderas.
- Ejecuta `orcon config check` para ver cada valor efectivo, de dónde viene y cualquier error. Los secretos se muestran como `[REDACTED]`.
- El archivo de configuración del servidor nunca se llama `orcon.yaml`; ese nombre pertenece a los repositorios de flujos como código.

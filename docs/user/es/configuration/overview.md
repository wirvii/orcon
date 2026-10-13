---
title: Resumen de configuración
description: Cómo lee Orcon sus ajustes y en qué orden, y cómo comprobar el resultado.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
sourceHash: 5c575608aea45093eeb5600d2aece592f247350b521ac5be476f9632a46cfc5c
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Resumen de configuración

Orcon lee sus ajustes de cuatro lugares, del más prioritario al menos: las banderas de la línea de órdenes, las variables de entorno `ORCON_*`, el archivo YAML indicado con `--config` (o `ORCON_CONFIG`) y los valores por defecto incorporados. Cada clave se resuelve por separado.

- Las claves YAML van en camelCase; una clave desconocida es un error.
- El nombre de la variable de entorno es `ORCON_` más la ruta de la clave en UPPER_SNAKE (`store.postgres.maxConnections` → `ORCON_STORE_POSTGRES_MAX_CONNECTIONS`). Excepciones: `ORCON_PUBLIC_URL`, `ORCON_MASTER_KEY`, `ORCON_MASTER_KEY_BACKUP_ACK`.
- Los secretos se pueden leer desde archivos: `dsnFile` en YAML, `ORCON_STORE_POSTGRES_DSN_FILE` en el entorno, `--store-postgres-dsn-file` como bandera. Los valores secretos nunca se aceptan como banderas.
- Ejecuta `orcon config check` para ver cada valor efectivo, de dónde viene y cualquier error. Los secretos se muestran como `[REDACTED]`.
- El archivo de configuración del servidor nunca se llama `orcon.yaml`; ese nombre pertenece a los repositorios de flujos como código.

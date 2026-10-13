---
title: Auditoría y SIEM
description: Qué registra Orcon sobre quién hizo qué, cómo detecta manipulaciones y cómo enviar los eventos a tus herramientas de seguridad.
sources:
  - ADR-0048
  - ADR-0083
sourceHash: 960b268fc9c3c56f81e3df5ef4f77a173e5b757285c282086baf593f5bf665af
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Auditoría y SIEM

> Esta página describe un comportamiento especificado por el ADR 0048 y el ADR 0083 que aún no está implementado. Los cortes que entreguen el registro de auditoría quitarán esta nota.

Orcon mantiene un registro de auditoría de las acciones que importan: inicios de sesión, cambios en flujos, credenciales, roles y políticas, operaciones con claves, cada uso de una credencial y cada consulta de datos de ejecución protegidos. El registro es a prueba de manipulación, y puedes enviarlo a tu sistema de información y eventos de seguridad (SIEM).

## Qué contiene un evento

Cada evento registra cuándo ocurrió, quién lo hizo (un usuario, una cuenta de servicio, un cliente SCIM, una clave de API o el sistema), la acción, el objetivo, el resultado (`SUCCESS`, `FAILURE` o `DENIED`), un motivo, detalles sin secretos ni datos de ejecución, y la petición (identificador, dirección y agente de usuario). Un evento escrito por una petición hecha con la API, la línea de órdenes, un SDK o MCP indica de qué canal vino. El formato del evento es `orcon/audit@1`.

El registro de auditoría se escribe en la misma transacción que la acción, así que una acción confirmada siempre tiene su registro. Las acciones denegadas también se registran.

## Prueba de manipulación

Los eventos están encadenados: cada uno lleva un hash del anterior, y los grupos de eventos se sellan con una suma con clave. Hay una cadena por cada organización y una para la instancia. Borrar o cambiar un evento, incluso directamente en la base de datos, rompe la cadena.

- Verifica en cualquier momento con `orcon audit verify`, y con `--org` para una organización. Orcon también verifica cada hora.
- Un fallo genera el evento `audit.integrity_violation`, un error en el registro, una métrica, una banda para los administradores y un evento al SIEM.
- Una vez por minuto, la cabeza de cada cadena se añade a un archivo del directorio de datos (`audit-anchors.log`) y se envía a cada destino de SIEM configurado. Una copia fuera del equipo es lo que permite a Orcon notar que alguien quitó los eventos más recientes. Mientras una cadena no tiene un destino externo, los administradores ven una advertencia permanente.
- Tras una restauración, las anclas más nuevas que el respaldo se mueven a un archivo y se registra el hueco, de modo que la verificación lo informa como una discontinuidad conocida y no como manipulación.

## Enviar eventos a tu SIEM

| Destino | Notas |
| --- | --- |
| Syslog (RFC 5424) | Sobre TLS (RFC 5425), TCP o UDP |
| Registros de OpenTelemetry | OTLP sobre HTTP o gRPC |
| Webhook HTTPS | Líneas JSON en lotes de hasta un segundo o 500 eventos, firmados con la cabecera `Orcon-Signature` (HMAC-SHA-256) |
| Archivo de líneas JSON | Un archivo local |

La entrega es al menos una vez. Orcon guarda un cursor por cada destino y cadena, apunta a un retraso p99 de 5 segundos y genera `siem.sink_lagging` si el retraso supera los 60 segundos. Un destino de organización (permiso `siem:manage`) recibe la cadena de su organización. Un destino de instancia recibe todas las cadenas. Los destinos de red usan el cliente de salida de Orcon con los servidores que configures.

## Retención

El valor por defecto es 365 días, y cada organización puede configurar más, por encima del piso de la instancia. La poda elimina bloques sellados completos y registra un punto de control para que la verificación continúe. La retención de la auditoría es independiente de la retención del historial de ejecuciones.

## Permisos

Leer el registro de auditoría necesita `audit:read` y exportarlo necesita `audit:export`. Consulta [Autorización](authorization.md).

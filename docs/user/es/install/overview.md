---
title: Resumen de la instalación
description: Las cuatro formas oficiales de instalar Orcon, qué tienen en común y cómo elegir una.
sources:
  - ADR-0083
  - ADR-0002
  - ADR-0081
sourceHash: 086f83669fb71c4f1056a6c2136ca8b0bac4cb112b7eaabb5ff60babe19ca581
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Resumen de la instalación

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue los instaladores, la imagen, los ejemplos de compose y el chart de Helm quitará esta nota.

Orcon es un único ejecutable que puede funcionar con varios roles (`standalone`, `api`, `scheduler` y `worker`). Hay cuatro formas oficiales de instalarlo. Todas instalan el mismo programa, y cada versión se prueba en todas ellas antes de publicarse.

## Elige una vía

| Vía | Base de datos | Roles | Elígela cuando |
| --- | --- | --- | --- |
| [Binario único](binary.md) | SQLite (incorporada) | `standalone` | Quieres un solo equipo y ningún otro servicio. Es la vía del [inicio rápido](../getting-started/quickstart.md). |
| [Imagen de contenedor](container.md) | SQLite por defecto; PostgreSQL por configuración | `standalone` por defecto | Ya ejecutas contenedores y quieres un proceso con un volumen de datos. |
| [Docker Compose](docker-compose.md) | SQLite, o PostgreSQL con dos workers | `standalone`, o `api`, `scheduler` y dos `worker` | Quieres un ejemplo listo de un solo servicio, o de una instalación distribuida pequeña, para adaptarlo. |
| [Kubernetes con Helm](kubernetes-helm.md) | Solo PostgreSQL | `api`, `scheduler` y `worker` en despliegues separados | Ejecutas Kubernetes y quieres escalar los roles por separado. |

SQLite funciona solo con el rol `standalone`. En cuanto quieras más de un servidor, o alta disponibilidad, usa PostgreSQL.

## Qué tienen en común todas las vías

- **Valores por defecto seguros.** Orcon escucha en la dirección de bucle local, y las métricas, los exportadores y la consulta de versiones están apagados. Todo lo demás lo activas tú. Consulta el [resumen de configuración](../configuration/overview.md).
- **Descargas verificadas.** Los instaladores de una línea verifican la firma del archivo de sumas y el SHA-256 del archivo comprimido, y no tienen ninguna opción para saltarse la comprobación. La imagen y el chart de Helm también están firmados. Consulta [Verificar publicaciones](verifying-releases.md).
- **La misma primera ejecución.** Hasta que la preparación termina, el servidor solo acepta las operaciones de preparación. Creas la clave maestra y la cuenta del propietario en el navegador o con comandos. Consulta [Primera ejecución](../configuration/first-run.md).
- **Ningún contacto con el proyecto.** Orcon no envía datos de uso ni identificadores de instalación. Consulta [Conexiones de red](../configuration/network-connections.md).

## Qué demuestra cada versión

Antes de publicar una versión, una prueba de instalación y ejecución la instala desde cero en cada vía, completa la primera ejecución sin interfaz web, ejecuta un flujo de muestra y comprueba que termina bien. La misma prueba se repite contra los activos ya publicados, y además actualiza desde la versión anterior. En las vías con PostgreSQL, la prueba comprueba que las ejecuciones se reparten entre al menos dos workers.

## Plataformas soportadas

Las plataformas, arquitecturas y versiones que se prueban están en la [matriz de soporte](../support/matrix.md). Windows se publica para evaluación y no se recomienda para producción.

## Fuera de la versión 1

Ejecutar Orcon como servicio de Windows, los paquetes para distribuciones de Linux o Homebrew y un operador de Kubernetes no forman parte de la primera versión.

## Siguientes pasos

1. Instala con una de las vías anteriores.
2. Completa la [primera ejecución](../configuration/first-run.md).
3. Lee el [resumen de configuración](../configuration/overview.md) y la [lista de comprobación de endurecimiento](../security/hardening-checklist.md).

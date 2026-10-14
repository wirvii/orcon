---
title: Matriz de soporte
description: Los sistemas operativos, bases de datos, herramientas de contenedores y de clústeres, y almacenamiento que soporta Orcon, y cuánto se prueba cada uno.
sources:
  - ADR-0083
sourceHash: 9fde55fac7421e5353e9d93f2cad105ce28265c665b363ce646feecb62a775fb
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Matriz de soporte

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El sitio de documentación generará esta página a partir de `docs/user/support-matrix.yaml`, el mismo archivo que alimenta las pruebas de instalación, y una prueba comparará ambos. Hasta entonces esta página se escribe a mano a partir de ese archivo. Si difieren, tiene razón el archivo.

## Niveles

| Nivel | Qué significa |
| --- | --- |
| 1 | La prueba de instalación y la batería completa de pruebas se ejecutan en cada versión. |
| 2 | Se publica el binario. La prueba de instalación se ejecuta cuando hay un ejecutor de CI disponible. No se ejecuta la batería completa. |
| Evaluación | Se publica, pero no se recomienda para producción. |

## Plataformas

| Plataforma | Artefactos | Nivel | Ejecutor de la prueba de instalación |
| --- | --- | --- | --- |
| Linux amd64 | Binario e imagen | 1 | `ubuntu-24.04` |
| Linux arm64 | Binario e imagen | 1 | `ubuntu-24.04-arm` |
| macOS arm64 | Binario | 1 | `macos-15` |
| macOS amd64 | Binario | 2 | Un ejecutor Intel si existe; si no, solo publicado |
| Windows amd64 | Binario | Evaluación | `windows-2025` |
| Windows arm64 | Binario | Evaluación | `windows-11-arm` si existe; si no, solo publicado |

Los requisitos mínimos del sistema son los de la cadena de herramientas de Go que construye cada versión. La página de cada versión los cita.

## PostgreSQL

| Versión | Estado | Notas |
| --- | --- | --- |
| 14 | Soportada | La comunidad termina su soporte el 2026-11-12. PostgreSQL 14 sale de la matriz con la primera versión menor publicada después del 2027-05-12. |
| 15 | Soportada | |
| 16 | Soportada | |
| 17 | Soportada | |

PostgreSQL 14 y 17 se prueban en cada solicitud de cambios (pull request) y cada noche. PostgreSQL 15 y 16 se prueban en la batería de publicación. PostgreSQL 18 se prueba en la batería nocturna y entra en la matriz solo después de que una versión lo haya probado en la batería de publicación. Una versión de PostgreSQL o de Kubernetes entra en la matriz solo cuando está en la matriz de pruebas, y sale con un aviso en las notas de una versión menor.

## SQLite

SQLite viene incorporada (el controlador `modernc.org/sqlite`). Funciona solo con el rol `standalone`.

## Kubernetes y Helm

- Kubernetes: las tres versiones menores con soporte de la comunidad en la fecha de la versión.
- Helm: 3.14 o posterior, y 4.x.

## Contenedores

- Docker Engine 24 o posterior.
- Docker Compose v2.

## Almacenamiento de objetos

- S3: probado con AWS S3 a través de su API. Las pruebas en CI usan un servidor compatible con licencia Apache-2.0 (SeaweedFS).
- Otros servicios compatibles con S3 deberían funcionar. No se prueban.

## Navegadores

Los navegadores soportados son los de los estándares de la aplicación web.

---
title: Obtener ayuda
description: Dónde hacer preguntas, informar de errores y proponer cambios, y qué incluir para que otros puedan ayudarte.
sources:
  - ADR-0081
  - ADR-0083
sourceHash: cbea432c1617db59b495356eae8391078a618ca22ef920c8e768ff16107002ab
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Obtener ayuda

Orcon es un proyecto de código abierto, y sus canales de ayuda son públicos.

## Antes de preguntar

1. Busca en esta documentación.
2. Si saltó una alerta, lee su sección en [Alertas](../operations/alerts.md).
3. Reúne lo que ayuda a otros a ver tu situación. Algunos de estos comandos los entregan cortes posteriores:

   ```sh
   orcon --version
   orcon config check
   orcon store schema status
   orcon health --ready
   ```

   `orcon config check` muestra cada ajuste y de dónde viene, con los secretos como `[REDACTED]`. Lee la salida antes de compartirla.

## Informar de un error o proponer una función

Usa los formularios de incidencias del repositorio, <https://github.com/wirvii/orcon/issues/new/choose>. Hay un formulario para informar de un error («algo no funciona como está documentado») y otro para solicitar una función («una capacidad nueva o un cambio de comportamiento»). Incluye la versión, el rol de ejecución y la base de datos, los pasos para reproducir y lo que esperabas.

Para cualquier cosa mayor que una corrección pequeña, abre primero una incidencia, para que los mantenedores confirmen la dirección antes de que dediques tiempo a un cambio.

## Contribuir

Las contribuciones son bienvenidas bajo la licencia Apache 2.0. Lee la guía de contribución en <https://github.com/wirvii/orcon/blob/main/CONTRIBUTING.md>. Todas las personas que participan siguen el código de conducta.

## Informar de un problema de seguridad

Nunca en una incidencia pública. Consulta [Informar de vulnerabilidades](../security/reporting-vulnerabilities.md).

---
title: Informar de vulnerabilidades
description: Cómo informar en privado de un problema de seguridad en Orcon y qué esperar después.
sources:
  - ADR-0081
sourceHash: aef4be05c1d5bc01d6049c83d879128c6b86595296d7fc2dcbf7ac7651372364
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Informar de vulnerabilidades

Si encontraste un problema de seguridad en Orcon, infórmalo en privado. No abras una incidencia, una solicitud de cambios (pull request) ni una discusión públicas.

## Cómo informar

Usa el informe privado de vulnerabilidades de GitHub: <https://github.com/wirvii/orcon/security/advisories/new>.

Incluye:

- la versión o el commit afectado,
- tu configuración (rol de ejecución y base de datos),
- los pasos para reproducir el problema,
- el impacto que observaste,
- la corrección que sugieras, si la tienes.

## Qué esperar

- Acusamos recibo de tu informe en un plazo de 5 días hábiles.
- Enviamos una primera evaluación en un plazo de 10 días hábiles.
- Acordamos contigo la corrección y la fecha de divulgación. Por defecto publicamos dentro de los 90 días siguientes al informe.
- Te reconocemos en el aviso, salvo que prefieras otra cosa.

## Qué versiones reciben correcciones

Antes de la versión 1.0, solo la última versión menor recibe correcciones de seguridad.

## Qué está dentro del alcance

El código fuente de Orcon, los binarios de las versiones y la imagen de contenedor `ghcr.io/wirvii/orcon`. Si el problema está en un servicio de terceros al que se conecta Orcon, infórmalo a ese servicio.

## Integridad de las versiones

Si una comprobación de firma falla en una versión oficial, o encuentras una firma que no debería existir, infórmalo de la misma forma. Para comprobar las versiones tú mismo, consulta [Verificar publicaciones](../install/verifying-releases.md).

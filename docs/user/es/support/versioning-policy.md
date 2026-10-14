---
title: Política de versiones
description: Cómo se numeran las versiones de Orcon, cuánto tiempo es estable la API y qué versiones reciben correcciones.
sources:
  - ADR-0081
  - ADR-0054
  - ADR-0015
  - ADR-0083
sourceHash: 9f0e046586eafadc8fbca98a740a1da5df53707b3fe7fbdd6dc76080e17d1780
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Política de versiones

> Las partes de esta página que se refieren a `orcon store schema`, a la línea de actualización en las notas de la versión y al sitio de documentación describen un comportamiento especificado por el ADR 0083 que aún no está implementado.

## Números de versión

Las versiones de Orcon usan versionado semántico: `MAJOR.MINOR.PATCH`. Cada versión tiene un registro de cambios, agrupado por tipo de cambio, y notas de la versión. Publicar una versión es un acto deliberado de los mantenedores. Consulta [Verificar publicaciones](../install/verifying-releases.md) para saber cómo se firman las versiones.

Orcon está en desarrollo temprano. Antes de la versión 1.0 nada está listo para producción, y solo la última versión menor recibe correcciones de seguridad. Cómo se mantienen las versiones antiguas después de la 1.0 todavía no está decidido. Cuando lo esté, esta página lo dirá.

## Actualizar entre versiones

Cada versión dice en una línea si puedes actualizar sin detener nada: `Upgrade: rolling (floor vX.Y.Z)` o `Upgrade: requires full stop (floor vX.Y.Z)`. El piso es la versión más antigua que funciona correctamente con la base de datos después de la actualización. Puedes saltar versiones menores cuando `orcon store schema plan` dice `ROLLING_SAFE`; si no, actualiza de una versión en una, o detén todo. Un cambio incompatible en la base de datos se reparte en dos versiones, de modo que la versión anterior sigue funcionando después de la primera. El procedimiento está en [Actualización y vuelta atrás](../operations/upgrade-and-rollback.md).

## La API

La API lleva la versión en su ruta, como `/v1`.

- Dentro de una versión mayor, solo se hacen cambios compatibles hacia atrás en las operaciones `stable` y `beta`. Cada operación declara su estabilidad como `stable`, `beta` o `experimental`.
- Un cambio incompatible va a una versión mayor nueva, `/v2`. Incompatible significa, por ejemplo, quitar o renombrar algo, añadir un campo obligatorio, restringir los valores aceptados o cambiar un tipo. La construcción compara el contrato con la última versión publicada y falla si se cuela un cambio incompatible.
- Un elemento obsoleto sigue funcionando dentro de su versión mayor, y se marca en el contrato y con las cabeceras de respuesta `Deprecation` y `Sunset` y un enlace a una guía.
- Una versión mayor antigua se soporta durante al menos 12 meses y al menos dos versiones menores de Orcon, lo que sea más largo, después de que la siguiente versión mayor sea estable. Las notas de la versión y un evento lo anuncian.

## Bases de datos y plataformas

Una versión de PostgreSQL o de Kubernetes entra en la matriz de soporte cuando está en la matriz de pruebas, y sale con un aviso en las notas de una versión menor. PostgreSQL 14 sale con la primera versión menor publicada después del 2027-05-12. Consulta la [matriz de soporte](matrix.md).

## La documentación

El sitio de documentación tiene una versión por cada línea menor de Orcon, una versión `next` construida desde la rama principal y `latest`, que es una copia de la línea menor más nueva. Una versión de parche reemplaza la carpeta de su línea menor. Las versiones antiguas no se reconstruyen.

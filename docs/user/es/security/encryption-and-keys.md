---
title: Cifrado y claves
description: Qué cifra Orcon, con qué claves, y qué debes hacer para que los datos sigan siendo recuperables.
sources:
  - ADR-0042
  - ADR-0049
  - ADR-0083
sourceHash: 03a929c7eb13d10f1573e90967461a5b5e185e2a7e2ac717e38ae9e1d8737401
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Cifrado y claves

> Esta página describe un comportamiento especificado por el ADR 0042, el ADR 0049 y el ADR 0083 que aún no está implementado. Los cortes que entreguen la jerarquía de claves quitarán esta nota.

Orcon cifra los datos sensibles que guarda. Esta página dice qué se cifra y con qué clave. Para las decisiones que tomas sobre la clave maestra y los procedimientos, consulta [Clave maestra y KMS](../configuration/master-key-and-kms.md).

## Qué se cifra

- **Credenciales.** Los campos secretos y los tokens OAuth, por versión de credencial.
- **Secretos en la configuración de una organización.** Por ejemplo, el secreto de cliente de una fuente de identidad y la configuración de un almacén de secretos externo, con la clave de la organización.
- **Secretos de la instancia.** Por ejemplo, las semillas de los códigos de dos factores, con la clave de la instancia.
- **Estado de trabajo de las ejecuciones sensibles.** Cifrado con una clave por cada ejecución, que se destruye al terminar la ejecución, para que Orcon pueda recuperarse de una caída sin volver a ejecutar las mismas acciones externas y para que los datos no se puedan leer después. El historial de ejecuciones siempre está enmascarado.
- **Respaldos.** El artefacto de datos está cifrado. Consulta [Respaldo y restauración](../operations/backup-and-restore.md).

Orcon no cifra toda la base de datos. Si lo necesitas, usa además el cifrado de tu almacenamiento o de tu servicio de base de datos.

## Las capas

1. La **clave maestra** viene de un proveedor que eliges y nunca está en la base de datos.
2. Las **claves de organización** y la **clave de instancia** se guardan envueltas por la clave maestra.
3. Las **claves de datos** protegen credenciales y estados de ejecución concretos, y las envuelve una clave de organización o la clave de instancia.

El algoritmo es AES-256-GCM. Cada valor cifrado lleva información autenticada sobre su organización, objeto, versión y campo, de modo que un valor no se puede mover a otro lugar y seguir descifrándose.

## Por qué te importan las capas

- Rotar la clave maestra reescribe decenas de registros de claves, no tus datos.
- Cada organización tiene su propia clave. Borrar una organización, tras su periodo de gracia, destruye sus claves, y sus credenciales y su estado de trabajo quedan ilegibles incluso en respaldos antiguos.
- Con un servicio de gestión de claves, desactivar la clave allí impide que Orcon desenvuelva en una hora.

## Las claves se comprueban al arrancar

Antes de cualquier escritura, Orcon comprueba que cada clave activa puede descifrar un valor de prueba conocido. Un proceso nunca escribe datos bajo una clave que no ha verificado. Si la comprobación falla, Orcon termina y nombra el proveedor, la referencia de la clave y la huella esperada.

## Datos de ejecución protegidos

Ver los datos de una ejecución es un permiso distinto de ver que una ejecución se realizó. Consulta [Autorización](authorization.md).

## Tus obligaciones

- Guarda el kit de recuperación durante la [primera ejecución](../configuration/first-run.md), fuera de este equipo.
- Guarda los respaldos de los datos y los de las claves en lugares distintos.
- Protege la clave maestra tan estrictamente como los datos que protege: un archivo legible solo por la cuenta de Orcon, o un servicio de gestión de claves con protección contra borrado y rotación de claves.

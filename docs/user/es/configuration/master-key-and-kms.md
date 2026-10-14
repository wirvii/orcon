---
title: Clave maestra y KMS
description: Elige dónde vive la clave maestra de Orcon, guarda a salvo el kit de recuperación y entiende qué pasa si pierdes la clave.
sources:
  - ADR-0042
  - ADR-0083
  - ADR-0043
sourceHash: 6b826ffb854c9f92742abab389bf341c7b700d3976c4a8878db4bcdb0856c573
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Clave maestra y KMS

> Esta página describe un comportamiento especificado por el ADR 0042 y el ADR 0083 que aún no está implementado. Los cortes que entreguen la jerarquía de claves y los comandos de primera ejecución quitarán esta nota.

La clave maestra es el único secreto que protege a todos los demás. Orcon nunca la guarda en su base de datos. Si la pierdes, los datos que protege no los puede leer nadie, tampoco el proyecto Orcon. Esta página explica tus opciones y tus obligaciones.

## Cómo encajan las claves

Orcon cifra los secretos en capas, de modo que la clave maestra rara vez toca tus datos:

- La **clave maestra** viene de un proveedor que tú eliges.
- Cada organización tiene su propia **clave de organización**, y la instancia tiene una **clave de instancia**. Se guardan cifradas con la clave maestra.
- Cada versión de una credencial, y el estado de trabajo sensible de cada ejecución, tiene su propia **clave de datos**, cifrada con la clave de organización o con la clave de instancia.

Todo el cifrado usa AES-256-GCM. Gracias a las capas, reemplazar la clave maestra reescribe solo unos pocos registros de claves, nunca tus credenciales ni tus datos. Borrar una organización destruye sus claves, lo que deja ilegibles sus datos cifrados incluso en respaldos antiguos, salvo que ese respaldo se restaure junto con una clave maestra capaz de desenvolver una copia de respaldo de la clave de la organización.

## Elige un proveedor

Define el proveedor con `keys.master.provider`. El valor por defecto es `file`.

| Proveedor | Dónde vive la clave maestra | Ajustes |
| --- | --- | --- |
| `file` | Un archivo de 32 bytes (en bruto o en base64). Orcon se niega a arrancar si otros usuarios pueden leerlo. | `keys.master.file.path`, por defecto `<dataDir>/keys/master.key` |
| `env` | La variable de entorno `ORCON_MASTER_KEY` (base64). La opción más débil; Orcon avisa al arrancar. | `keys.master.key`, también `ORCON_MASTER_KEY_FILE` |
| `awskms` | AWS Key Management Service | `keys.master.awskms.keyArn` |
| `gcpkms` | Google Cloud KMS | `keys.master.gcpkms.keyName` |
| `azurekv` | Azure Key Vault (o Managed HSM) | `keys.master.azurekv.keyId` |
| `vault-transit` | HashiCorp Vault u OpenBao, motor transit | `keys.master.vaultTransit.address`, `.mount`, `.keyName` y el secreto `.token` |

Con un servicio de gestión de claves (KMS), la clave maestra nunca sale del servicio. Orcon le pide que envuelva y desenvuelva, guarda en memoria las claves de organización y de instancia ya desenvueltas (bloqueadas en memoria donde el sistema lo permite) y las vuelve a desenvolver cada hora. Por eso, desactivar la clave en el servicio surte efecto en una hora. La identidad de nube del servidor puede autenticarse ante el servicio; esa identidad pertenece a la instancia, no a ninguna organización.

## La huella y la comprobación al arrancar

La huella identifica una clave maestra: para una clave que tú guardas es el SHA-256 de la clave, abreviado; para un KMS es la referencia de la clave. Al arrancar, antes de cualquier migración o escritura, Orcon usa la clave maestra para abrir un pequeño valor de prueba de cada clave y comprueba que se descifra. Si no, Orcon se detiene y te dice qué proveedor, qué referencia de clave y qué huella esperaba. Orcon nunca escribe datos bajo una clave que no ha verificado.

## El kit de recuperación

El kit de recuperación contiene el material de la clave (o la referencia a tu clave del KMS), la huella, las instrucciones para restaurar y una advertencia de que la pérdida es irreversible. Lo obtienes durante la [primera ejecución](first-run.md) y debes confirmar que lo guardaste. No hay forma de saltarse la confirmación.

Guarda el kit donde un fallo de este equipo no pueda alcanzarlo: un gestor de contraseñas, una caja fuerte, un segundo sitio. No lo guardes solo junto a los datos o a los respaldos.

## Clave de recuperación opcional

Puedes añadir una segunda forma de recuperar. Define `keys.recoveryPublicKey` con una clave pública age (`age1…`, X25519). Entonces Orcon envuelve también cada clave de organización con ella. Guarda la clave privada correspondiente fuera de línea. Si pierdes la clave maestra, `orcon key recover` usa la clave privada para recuperar. Está desactivada por defecto.

## Rotación

- **Rota la clave maestra** con `orcon key rotate-master --to <configuración del proveedor>`. Orcon vuelve a envolver cada clave de organización y de instancia con la clave nueva, en lotes pequeños, con ambas claves configuradas durante el cambio. Tus datos no se tocan, la orden puede reanudarse tras una caída, y las pruebas de verificación confirman el resultado. Quita la clave antigua solo cuando termine.
- **Rota una clave de organización** con `orcon key rotate-org <org>`. Una versión nueva se usa para todas las escrituras nuevas, y una tarea en segundo plano vuelve a envolver las claves de datos en lotes. Por defecto esto ocurre también cada año.

Las rotaciones no mantienen transacciones largas, se frenan bajo carga y se auditan.

## Con un KMS: tu parte del trabajo

Cuando validas una clave del KMS, Orcon avisa si la clave no tiene protección contra borrado o rotación de claves. Corrige ambas cosas en el servicio. Borrar la clave allí destruye el acceso a todo lo que protege.

## Respaldos y clave maestra

Un respaldo de los datos no sirve sin la clave maestra, y la clave maestra nunca se escribe junto a los datos. Para una clave `file` o `env`, un respaldo puede guardar además el material de la clave cifrado para los destinatarios age que listes. Consulta [Respaldo y restauración](../operations/backup-and-restore.md).

## Si pierdes la clave maestra

Sin la clave maestra, y sin una clave de recuperación ni un respaldo del material de la clave, las credenciales cifradas y el estado de trabajo de las ejecuciones sensibles no los puede recuperar nadie.

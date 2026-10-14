---
title: Primera ejecución
description: Crea la clave maestra y la cuenta del propietario de una instancia nueva de Orcon, en el navegador o con comandos.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
  - ADR-0038
sourceHash: 2ab8a21383a56c0d23a869077961ddabade538a1d4e66683b50796c4d1612136
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Primera ejecución

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue los comandos y las pantallas de primera ejecución quitará esta nota.

Una instancia nueva de Orcon no se puede usar hasta que existen dos cosas: una clave maestra que protege todos los secretos y una cuenta de propietario. Las creas una sola vez, con el asistente web o sin navegador. No se escribe ningún otro dato tuyo antes de que la clave esté verificada.

## Qué hace el servidor hasta que termina la preparación

Hasta que la preparación termina, el servidor acepta solo las operaciones de preparación, protegidas por un código de preparación de un solo uso. Una instancia está en uno de tres estados:

| Estado | Significado |
| --- | --- |
| `NEEDS_KEY_CEREMONY` | Todavía no hay una clave maestra verificada. |
| `NEEDS_OWNER` | La clave está verificada y ningún usuario es todavía propietario de la instancia. |
| `COMPLETE` | La preparación terminó. |

Antes de que el estado sea `COMPLETE`, el servidor aplica las migraciones y escribe solo lo que la preparación necesita. No crea ninguna organización, ningún usuario ni datos cifrados hasta que las claves están verificadas.

## El código de preparación

Cuando el servidor arranca en un estado distinto de `COMPLETE`, crea un código de preparación aleatorio. Es válido durante 24 horas y deja de serlo cuando la preparación termina.

- Si la salida estándar es una terminal, el servidor imprime `Open <publicUrl>/setup?token=<código>`.
- El código nunca se escribe en los registros. Los registros dicen solo cómo obtenerlo.
- Para obtenerlo más tarde, ejecuta `orcon setup token`. Con SQLite lo lee de un archivo llamado `setup-token` en el directorio de datos, que solo puede leer su propietario. Con PostgreSQL, `orcon setup token --rotate` crea un código nuevo y guarda solo su hash en la base de datos.

Para ver en qué punto está la instancia, ejecuta `orcon setup status`.

## Opción 1: el asistente web

Abre la dirección con el código, o abre `/setup` e introdúcelo. El asistente tiene cuatro pasos, en este orden, y funciona con teclado y con lectores de pantalla. Está disponible en inglés y en español, y nunca muestra un estado solo con color.

1. **Código.** Introduce el código de preparación. Esto inicia una sesión de preparación que dura 15 minutos.
2. **Clave maestra.** Con el proveedor `file`, el asistente genera la clave maestra, o usa la que ya existe en `keys.master.file.path`. Con un servicio de gestión de claves, comprueba que Orcon puede envolver y desenvolver con tu clave, y avisa si allí no están activadas la protección contra borrado o la rotación de claves. Después:
   - Ofrece descargar el kit de recuperación: el material de la clave o la referencia a tu servicio de gestión de claves, la huella, las instrucciones para restaurar y una advertencia clara de que perder la clave deja irrecuperables los datos cifrados.
   - La descarga se ofrece solo por HTTPS o en la dirección de bucle local. En cualquier otro caso el botón está desactivado y el asistente te remite a los comandos de abajo.
   - Debes escribir los últimos 8 caracteres de la huella para confirmar que guardaste el kit. No hay forma de saltarse este paso.
   - Ofrece, desactivada por defecto, una clave pública de recuperación opcional. Se configura en `keys.recoveryPublicKey`; consulta [Clave maestra y KMS](master-key-and-kms.md).
3. **Cuenta del propietario.** Introduce un correo, un nombre y una contraseña. La contraseña debe cumplir la política de contraseñas: al menos 12 caracteres, sin reglas de composición.
4. **Fin.** Orcon crea la organización llamada `default`, crea tu usuario como propietario de la instancia y de esa organización, registra el evento `setup.completed`, invalida el código y te lleva al inicio de sesión.

Después de la preparación, toda operación de preparación salvo leer el estado responde con el problema `setup-already-complete`, y la página `/setup` redirige al inicio de sesión.

## Opción 2: sin navegador

Usa este camino para instalaciones con scripts, contenedores y Kubernetes.

1. Crea la clave maestra. Con el proveedor `file` el comando la genera; con un servicio de gestión de claves valida tu clave. Imprime la huella y escribe el kit de recuperación.

   ```sh
   orcon key init
   ```

   `--out <ruta>` elige dónde se escribe la clave y `--kit-out <ruta>` dónde se escribe el kit. Copia el kit a un lugar seguro que no sea este equipo.

2. Confirma que guardaste el kit dando la huella al servidor.

   ```sh
   export ORCON_MASTER_KEY_BACKUP_ACK=<huella>
   ```

   `<huella>` es el valor que imprimió `orcon key init`. El servidor registra la confirmación como el evento `key.backup_acknowledged` con el método `HEADLESS`. Si el valor no coincide con la clave, el servidor no arranca y termina con el código 4.

3. Crea la cuenta del propietario de una de estas dos formas.

   - Con el servidor detenido (SQLite), ejecuta:

     ```sh
     orcon setup owner --email <correo> --name <nombre> --password-file <archivo>
     ```

     `<correo>` y `<nombre>` identifican al propietario. `<archivo>` contiene solo la contraseña.
   - O define las claves `setup.owner.email`, `setup.owner.name` y `setup.owner.password` (un secreto, así que también acepta `ORCON_SETUP_OWNER_PASSWORD_FILE`). El servidor las usa solo mientras el estado es `NEEDS_OWNER`, y avisa si siguen presentes después, así que quítalas cuando la preparación termine.

## Con PostgreSQL y varias réplicas

El estado y el hash del código viven en la base de datos, así que cualquier réplica de `api` puede servir el asistente, y un candado garantiza que solo se ejecuta una ceremonia a la vez. El chart de Helm exige el camino por comandos de arriba. Consulta [Instalar en Kubernetes con Helm](../install/kubernetes-helm.md).

## Si algo se interrumpe

Una interrupción durante la preparación deja el estado en `NEEDS_KEY_CEREMONY` o en `NEEDS_OWNER`. Una clave generada sin comprobación confirmada no se usa. Si el archivo de la clave ya se escribió, el siguiente intento lo reutiliza y muestra su huella. Ningún estado deja datos cifrados con una clave que Orcon no reconoce.

## Siguientes pasos

- Lee [Clave maestra y KMS](master-key-and-kms.md) y guarda el kit de recuperación en un lugar seguro.
- Configura los [respaldos](../operations/backup-and-restore.md).
- Repasa la [lista de comprobación de endurecimiento](../security/hardening-checklist.md).

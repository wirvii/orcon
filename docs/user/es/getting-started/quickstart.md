---
title: Inicio rápido
description: Instala Orcon en un equipo y ejecuta tu primer flujo en unos cinco minutos.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0039
sourceHash: a5bdf114e147cc9a52c4fd60fd89a2c412505d8663b8a0917c5fff62e52f258a
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Inicio rápido

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue el instalador y los comandos de primera ejecución quitará esta nota.

En unos cinco minutos instalas Orcon en un equipo, lo configuras y ejecutas un primer flujo. Este camino usa el binario único con su base de datos SQLite incorporada, así que no necesitas ningún otro servicio.

Necesitas un equipo con Linux o macOS con `curl` o `wget`, una terminal y un navegador web.

## Pasos

1. Instala Orcon con el instalador de una línea. Descarga la versión, comprueba la firma del archivo de sumas y el SHA-256 del archivo comprimido, e instala solo si ambas comprobaciones pasan. No existe ninguna opción para saltarse las comprobaciones.

   <!-- install-test -->
   ```sh
   curl -fsSL https://github.com/wirvii/orcon/releases/latest/download/install.sh | sh
   ```

   El instalador deja `orcon` en `/usr/local/bin` si puede escribir allí, y en `~/.local/bin` si no. Para elegir otro directorio, añade `--dir <directorio>` después de `sh -s --`.

2. Comprueba la instalación.

   <!-- install-test -->
   ```sh
   orcon --version
   ```

3. Crea la clave maestra. Protege todos los secretos que guarda Orcon, así que perderla deja irrecuperables los datos cifrados. El comando imprime la huella de la clave y escribe un kit de recuperación.

   <!-- install-test -->
   ```sh
   orcon key init
   ```

   Copia el kit de recuperación a un lugar seguro que no sea este equipo, por ejemplo un gestor de contraseñas.

4. Dile a Orcon que guardaste el kit definiendo la huella que imprimió el paso 3.

   ```sh
   export ORCON_MASTER_KEY_BACKUP_ACK=<huella>
   ```

   `<huella>` es la huella que imprimió `orcon key init`. Si el valor no coincide con la clave, el servidor se niega a arrancar y sale con el código 4.

5. Crea la cuenta del propietario. Ejecuta este comando antes de que arranque el servidor.

   ```sh
   orcon setup owner --email <correo> --name <nombre> --password-file <archivo>
   ```

   `<correo>` y `<nombre>` identifican al propietario. `<archivo>` es un archivo que contiene solo la contraseña, que debe cumplir la política de contraseñas de tu instalación.

6. Arranca el servidor.

   <!-- install-test -->
   ```sh
   orcon serve
   ```

   Sin argumentos, `orcon serve` ejecuta todos los roles en un solo proceso (el rol `standalone`) y escucha únicamente en `127.0.0.1:8080`.

7. Abre `http://127.0.0.1:8080` en tu navegador e inicia sesión con el correo y la contraseña del paso 5.

8. Crea un flujo con un disparador manual y un paso, y ejecútalo con el botón de prueba, o pulsa Ctrl+Enter (Cmd+Enter en macOS).

9. Abre la lista de ejecuciones. Tu corrida aparece con el estado `SUCCEEDED`.

## ¿Prefieres configurar desde el navegador?

Omite los pasos 3 a 5 y ejecuta `orcon serve`. Cuando la terminal muestre una dirección que empieza por `Open`, ábrela. Un asistente crea la clave maestra, muestra el kit de recuperación, te pide escribir los últimos ocho caracteres de la huella y después crea la cuenta del propietario. El kit solo se ofrece por HTTPS o en el equipo local. Si no puedes usar el asistente, usa los pasos anteriores.

## Qué sigue

- Respalda tus datos: [Respaldo y restauración](../operations/backup-and-restore.md).
- Pasa a una versión nueva: [Actualización y vuelta atrás](../operations/upgrade-and-rollback.md).
- Cambia un ajuste: [Resumen de configuración](../configuration/overview.md).
- Vigila la instancia: [Alertas](../operations/alerts.md).

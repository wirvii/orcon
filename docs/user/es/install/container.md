---
title: Ejecutar la imagen de contenedor
description: Ejecuta Orcon desde su imagen oficial, con un volumen de datos y una comprobación de salud.
sources:
  - ADR-0083
  - ADR-0081
  - ADR-0002
sourceHash: 7c0ca46cc6f4721cbe52b493a3b841425781f8156d991cf16fba504d4e1a35f5
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Ejecutar la imagen de contenedor

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue la imagen quitará esta nota.

La imagen oficial es `ghcr.io/wirvii/orcon:<versión>`. Se construye para `linux/amd64` y `linux/arm64`, se ejecuta con un usuario sin privilegios y está firmada con cosign. `<versión>` es un número de versión como `0.3.0`.

## Qué contiene la imagen

| Ajuste | Valor |
| --- | --- |
| Base | Una imagen distroless estática, `nonroot`. No tiene shell ni `curl`. |
| Usuario | `65532` |
| Directorio de datos | `/var/lib/orcon`, propiedad del usuario `65532` y declarado como volumen. `ORCON_DATA_DIR` apunta a él. |
| Puertos | `8080` para la aplicación (`ORCON_HTTP_ADDRESS=:8080`) y `9464` para el escuchador de operaciones (`ORCON_OPS_ADDRESS=:9464`) |
| Punto de entrada | `/usr/local/bin/orcon`, con la orden por defecto `serve --role=standalone` |
| Comprobación de salud | `orcon health --ready` |

La imagen funciona con el sistema de archivos raíz de solo lectura: Orcon escribe únicamente en `/var/lib/orcon` y `/tmp`. La imagen cambia solo las direcciones de escucha respecto a los valores por defecto seguros; las métricas de Prometheus siguen apagadas hasta que las actives.

## Ejecutarla

```sh
docker run --name orcon -p 8080:8080 -v orcon-data:/var/lib/orcon ghcr.io/wirvii/orcon:<versión>
```

Sustituye `<versión>` por la versión que quieras. Fija siempre una versión; no dependas de una etiqueta móvil. El volumen contiene la base de datos SQLite, el archivo de la clave maestra y los blobs, así que créalo antes de guardar en Orcon algo importante e inclúyelo en tus [respaldos](../operations/backup-and-restore.md).

Para usar PostgreSQL o cualquier otro ajuste, pasa variables de entorno `ORCON_*` o monta un archivo YAML e indica su ruta en `ORCON_CONFIG`. Los secretos van en archivos leídos con las variantes `*_FILE`. Consulta [Secretos](../configuration/secrets.md).

## Primera ejecución

Cuando el servidor arranca por primera vez, espera a que se complete la [primera ejecución](../configuration/first-run.md). El código de preparación se muestra en la terminal solo si la salida es una terminal. En un contenedor normalmente lo lees con una orden:

```sh
docker exec orcon orcon setup token
```

Después abre `/setup` en la dirección de tu Orcon e introduce el código. Si llegas a Orcon por una dirección distinta de `http://127.0.0.1:8080`, define `ORCON_PUBLIC_URL` con ella y lee [TLS y proxy inverso](../configuration/tls-and-reverse-proxy.md).

## Comprobar que está sano

```sh
docker exec orcon orcon health --ready
```

La orden termina con 0 cuando el proceso está listo y con 1 en caso contrario. Existe porque la imagen no trae `curl`.

## Verificar la imagen

Comprueba la firma y la procedencia de la imagen antes de ejecutarla. Consulta [Verificar publicaciones](verifying-releases.md).

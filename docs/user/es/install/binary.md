---
title: Instalar el binario único
description: Instala Orcon en un equipo con el instalador de una línea y sabe dónde guarda sus datos.
sources:
  - ADR-0083
  - ADR-0081
  - ADR-0002
sourceHash: 1ba0b4ea5c5114bc3312882ee5d70032707d57f4bcca0678b3fb9976d52ae8e2
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Instalar el binario único

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue los instaladores quitará esta nota.

El ejecutable `orcon` se publica para Linux, macOS y Windows, en amd64 y arm64. Funciona solo, con su base de datos SQLite incorporada, y no necesita ningún otro servicio.

## Instalar con el instalador de una línea

En Linux y macOS el instalador es un script de shell POSIX que necesita `curl` o `wget`.

```sh
curl -fsSL https://github.com/wirvii/orcon/releases/latest/download/install.sh | sh
```

En Windows el instalador es un script de PowerShell, `install.ps1`, para PowerShell 5.1 o posterior. Ambos scripts se publican como activos de cada versión y figuran en su `checksums.txt`.

El instalador sigue estos pasos, en orden:

1. Detecta tu sistema operativo y tu arquitectura.
2. Descarga el archivo comprimido de la versión pedida (por defecto, la última que no es preliminar), el archivo `checksums.txt` y su paquete de firma.
3. Verifica con cosign la firma de `checksums.txt`, contra la identidad y el emisor descritos en [Verificar publicaciones](verifying-releases.md). Si cosign no está o es más viejo que la versión mínima, el instalador descarga un cosign fijado, comprueba su SHA-256 (que está escrito dentro del instalador) y lo usa desde un directorio temporal.
4. Verifica el SHA-256 del archivo comprimido.
5. Extrae el archivo e instala `orcon` en `/usr/local/bin` si puede escribir allí, y en `~/.local/bin` si no. La opción `--dir <directorio>` elige otro directorio.
6. Imprime la salida de `orcon --version`.

Si falla cualquier paso, el instalador termina con un código distinto de 0 y no instala nada. No existe ninguna opción para saltarse la verificación.

El instalador contacta tres servidores y ninguno más: `github.com`, `objects.githubusercontent.com` y el servidor de la raíz de confianza de Sigstore que usa cosign.

Si prefieres descargar y comprobar los archivos tú mismo, sigue [Verificar publicaciones](verifying-releases.md) y coloca el ejecutable `orcon` en cualquier lugar de tu ruta de búsqueda.

## Iniciar el servidor

```sh
orcon serve
```

Sin argumentos, `orcon serve` inicia el rol `standalone`. Escucha en `127.0.0.1:8080` y guarda sus datos en el directorio de datos por defecto de tu sistema operativo:

| Sistema | Directorio de datos por defecto |
| --- | --- |
| Linux | `$XDG_DATA_HOME/orcon`, o `~/.local/share/orcon` si `XDG_DATA_HOME` no está definida |
| macOS | `~/Library/Application Support/Orcon` |
| Windows | `%LOCALAPPDATA%\Orcon` |

Cambia el directorio con la clave `dataDir`, la variable de entorno `ORCON_DATA_DIR` o la bandera correspondiente. La primera vez que arranca, el servidor espera a que se complete la [primera ejecución](../configuration/first-run.md).

## Roles

La bandera `--role` (o la clave `role`) acepta `standalone`, `api`, `scheduler` y `worker`. El valor por defecto es `standalone`. SQLite funciona solo con `standalone`; `orcon config check` informa un error con cualquier otro rol sobre SQLite. Para ejecutar los roles como procesos separados necesitas PostgreSQL. Consulta [Docker Compose](docker-compose.md) y [Kubernetes con Helm](kubernetes-helm.md) para ver ejemplos completos.

## Ejecutarlo como servicio con systemd

El repositorio incluirá un archivo de unidad de ejemplo, endurecido, en `deploy/systemd`, para ejecutar el rol `standalone` como servicio de systemd en Linux. Orcon no se ejecuta como servicio de Windows en la versión 1.

## Siguientes pasos

- Completa la [primera ejecución](../configuration/first-run.md).
- Lee el [resumen de configuración](../configuration/overview.md).
- Planea tus [respaldos](../operations/backup-and-restore.md) antes de guardar algo que no puedas perder.

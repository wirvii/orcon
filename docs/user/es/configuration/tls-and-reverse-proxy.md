---
title: TLS y proxy inverso
description: Sirve Orcon por HTTPS, con su propio certificado o detrás de un proxy, y define la dirección pública.
sources:
  - ADR-0083
  - ADR-0038
  - ADR-0039
  - ADR-0052
sourceHash: caa6698efbcfd389b865aa2dbdd046da028111b6627d9a9b9d215c3df82f36ff
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# TLS y proxy inverso

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue el esquema de configuración quitará esta nota.

Por defecto Orcon escucha solo en la dirección de bucle local, `127.0.0.1:8080`, y llegas a él desde el mismo equipo. Para usarlo desde otros equipos debes servirlo por HTTPS. Esta página explica por qué y las dos formas de hacerlo.

## Por qué hace falta HTTPS

La cookie de sesión de Orcon es `__Host-orcon_session`, marcada `Secure` y `HttpOnly`. Un navegador envía una cookie así solo por HTTPS, o en `localhost`. Por eso el inicio de sesión por HTTP sin cifrar funciona solo en la dirección de bucle local. El mismo motivo hace que el asistente de primera ejecución no ofrezca el kit de recuperación en ningún otro lugar: el kit no debe viajar sin cifrar. Las claves de acceso (passkeys) también dependen de la dirección: el identificador de la parte confiable es el host de tu dirección pública.

## Define la dirección pública

`publicUrl` (variable de entorno `ORCON_PUBLIC_URL`) es la dirección que escriben tus usuarios. Por defecto es `http://127.0.0.1:8080`. Defínela con la dirección real, por ejemplo `https://orcon.example.com`.

`orcon config check` informa un error cuando `publicUrl` empieza por `http://`, su host no es una dirección de bucle local y no hay TLS configurado. El error te pide definir `http.tls.*`, o terminar el TLS en un proxy y usar `https://`.

## Opción 1: Orcon sirve el TLS él mismo

Dale a Orcon un certificado y su clave privada:

```yaml
publicUrl: https://orcon.example.com
http:
  address: ":8443"
  tls:
    certificateFile: /etc/orcon/tls/tls.crt
    keyFile: /etc/orcon/tls/tls.key
```

`http.address` es la dirección y el puerto en que escuchar. Las dos claves toman rutas de archivos, y la cuenta que ejecuta Orcon debe poder leerlos.

## Opción 2: un proxy inverso o un balanceador termina el TLS

Ejecuta Orcon en una dirección privada y deja que un proxy o balanceador delante de él presente el certificado. Define `publicUrl` con la dirección HTTPS que sirve el proxy, y lista los proxies que están delante de Orcon en `http.trustedProxies`, como una lista de rangos CIDR.

Con varios procesos `api` detrás de un balanceador, el balanceador no necesita afinidad de sesión.

## Otros escuchadores

El escuchador de operaciones (`ops.address`, apagado por defecto) es independiente. Sirve solo `/healthz`, `/readyz` y `/metrics`, y nunca la API, la interfaz web ni la entrada de disparadores. No lo expongas a internet. Consulta la [lista de comprobación de endurecimiento](../security/hardening-checklist.md).

## Apagado

`http.shutdownTimeout` (30 segundos por defecto) es el tiempo que `api` espera a las peticiones en curso cuando recibe una señal de parada.

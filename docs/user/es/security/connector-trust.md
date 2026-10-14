---
title: Confianza en conectores
description: Cómo decide Orcon qué paquetes de conectores se pueden instalar, qué significan los sellos y cuándo una instancia contacta un catálogo remoto.
sources:
  - ADR-0033
  - ADR-0083
sourceHash: bff82281a89a7b753a666f227d86cc95caf1e8f5ba77dc9f22a4753b08d04836
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Confianza en conectores

> Esta página describe un comportamiento especificado por el ADR 0033 y el ADR 0083 que aún no está implementado. Los cortes que entreguen el catálogo de conectores quitarán esta nota.

Un conector es código que habla con otro servicio en nombre de un flujo. Orcon te deja decidir qué conectores pueden ejecutarse y te muestra de dónde viene cada uno.

## Sellos

Cada paquete tiene exactamente un sello.

| Sello | Significado |
| --- | --- |
| `OFFICIAL` | Escrito por Orcon y firmado por su canal de publicación. Los conectores incorporados en el binario son oficiales. |
| `VERIFIED` | Revisado por Orcon y firmado por el servicio de catálogo tras una revisión aprobada. |
| `UNREVIEWED` | Todo lo demás: paquetes de un catálogo privado (mostrados como «firmado por» el nombre del catálogo), archivos locales y la salida del conversor. |

Solo el catálogo de Orcon tiene las claves de los dos primeros sellos. El conversor público y las herramientas locales nunca los producen.

## Política de confianza

La política es configuración, no código. Tiene tres perfiles.

| Perfil | Qué permite |
| --- | --- |
| `OPEN` (el valor por defecto) | Todos los sellos. El catálogo público de Orcon más los catálogos privados que añada un administrador. Los paquetes sin firma y los archivos locales siguen apagados hasta que el administrador los permita. |
| `STRICT` | Solo paquetes `OFFICIAL` y `VERIFIED`, solo del catálogo público de Orcon, sin paquetes sin firma ni archivos locales. Está bloqueado. |
| `CUSTOM` | Tu propia elección de sellos y catálogos, y los dos interruptores para paquetes sin firma y archivos locales. |

La política se lee de `connectors.trust`. Cuando está `locked`, solo se puede leer del archivo de configuración o del entorno al arrancar, y la interfaz web y la API la muestran como de solo lectura. La política se comprueba al instalar, de nuevo al publicar y de nuevo en cada envío. Si la endureces, los paquetes instalados que ya no la cumplen quedan bloqueados por política: sus nodos fallan al validarse y al enviarse, con un mensaje explícito.

Todos los perfiles están en el mismo binario Apache-2.0 y son gratuitos en toda instalación. Ninguna capacidad depende de una edición, una clave de licencia o un pago.

## Qué apruebas al instalar

Al instalar, el administrador ve el sello, el publicador, la licencia, hasta qué punto están verificadas las garantías de seguridad de cada operación, los dominios de red, los tipos de credencial, cualquier acceso en bruto a credenciales y los campos sensibles. La aprobación se guarda con un resumen (digest) de esos permisos. Si una actualización los cambia, hace falta una aprobación nueva, y hasta entonces los nodos se quedan en la versión anterior.

## Verificación

Los catálogos siguen The Update Framework (TUF), de modo que un espejo es solo un medio de transporte y no puede falsificar un paquete. Instalar ejecuta una actualización TUF completa, comprueba longitudes y hashes, y comprueba la política. Los workers verifican el digest del paquete antes de cargarlo. Los metadatos de frescura caducados bloquean nuevas instalaciones y actualizaciones, salvo en modo sin conexión.

## Revocación

Un catálogo puede revocar una versión de un paquete, como `CRITICAL` o `ADVISORY`.

- `CRITICAL`: se bloquea el envío del nodo afectado. Los flujos siguen publicados y activos, y cada envío falla con `ConnectorRevoked`, que nombra el conector, la versión, el motivo y la versión a la que actualizar. Cada organización afectada recibe una alerta y un evento `connector.revoked`, dirigidos a sus Owners, Admins y a los propietarios de los proyectos afectados, hasta que se confirme. Un administrador puede registrar una excepción supervisada para un paquete y una versión, con un motivo y una caducidad. Se audita.
- `ADVISORY`: una advertencia en el editor y en el panel. El envío continúa.

Ningún ajuste rebaja `CRITICAL` a una advertencia.

## Cuándo una instancia contacta un catálogo

Una instancia que nunca instaló un paquete de un catálogo remoto nunca contacta con uno. Orcon obtiene los metadatos del catálogo cada vez que instalas o actualizas desde un catálogo remoto. Mientras siga instalado al menos un paquete que un administrador instaló desde un catálogo remoto, también refresca esos metadatos cada `metadataRefresh` (6 horas por defecto). Definir `metadataRefresh` en `0` detiene el refresco periódico.

Los paquetes preinstalados en una imagen, o instalados desde un archivo de paquete, nunca programan un refresco. Su información de revocación es tan reciente como el paquete o la versión que los trajo.

## Instalación sin conexión

`orcon connector install --bundle <archivo.orconbundle>` instala un paquete junto con toda su cadena de metadatos, sin red. Los metadatos de frescura caducados se aceptan solo con `--offline` y la confirmación del administrador, y la interfaz web dice que la información de revocación es tan reciente como el paquete.

## Catálogos privados

Puedes ejecutar tu propio catálogo. `orcon catalog init`, `orcon catalog add`, `orcon catalog sign` y `orcon catalog serve` lo crean, lo llenan, lo firman y lo sirven como archivos estáticos. Los paquetes que salen de él llevan el sello `UNREVIEWED`, etiquetado con el nombre de tu catálogo.

---
title: Conexiones de red
description: Todos los motivos por los que Orcon abre una conexión de salida, y cómo saber que una instalación nueva no abre ninguna por su cuenta.
sources:
  - ADR-0083
  - ADR-0033
  - ADR-0044
sourceHash: c335342b8c40f6d6562f6101d76d9e77e0a8218d1656e6282554fc51d2273af2
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Conexiones de red

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue el registro de conexiones de salida quitará esta nota. Cuando se entregue, la tabla de propósitos de abajo se generará a partir de ese registro y esta página se sustituirá por la versión generada.

Orcon nunca envía datos de uso ni un identificador de instalación al proyecto ni a nadie más. Esta página lista todos los motivos por los que un proceso de Orcon abre una conexión de salida, para que puedas escribir una regla de cortafuegos o comprobar una afirmación.

## La garantía

Toda conexión de salida tiene una de estas tres causas:

1. **Tu configuración nombra el destino.** Una base de datos, un servicio de gestión de claves, un servidor de correo, un destino de telemetría, un bucket S3, un destino de auditoría, un almacén de secretos, un proveedor de identidad o la consulta de versiones, una vez que la activas.
2. **Tus flujos, credenciales y disparadores.** Un flujo que llama a un servicio se conecta a él.
3. **Una acción explícita de un administrador.** Instalar o actualizar un conector desde un catálogo remoto. Mientras siga instalado al menos un paquete instalado así, Orcon también refresca de vez en cuando los metadatos de ese catálogo.

Una instalación nueva, con los valores por defecto, no hace ninguna conexión de salida que nadie haya pedido.

## Los propósitos

Cada componente que puede abrir una conexión está registrado con un propósito de esta lista cerrada, y con la clave o la acción que lo activa.

| Propósito | Para qué sirve | Qué lo activa |
| --- | --- | --- |
| `STORE` | La base de datos PostgreSQL | `store.driver: postgres` y su cadena de conexión |
| `KMS` | Un servicio de gestión de claves para la clave maestra | Un `keys.master.provider` distinto de `file` y de `env` |
| `SMTP` | Enviar correo | Los ajustes del servidor de correo |
| `OTLP` | Exportar métricas, trazas o registros con OpenTelemetry | `observability.otlp.*` |
| `BACKUP_TARGET` | Escribir respaldos en almacenamiento compatible con S3 | `backup.target` con una dirección `s3://` |
| `BLOB_S3` | Guardar blobs en almacenamiento compatible con S3 | `blobs.backend: s3` |
| `SIEM` | Enviar eventos de auditoría a un sistema de información de seguridad | Un destino de auditoría que configuras |
| `SECRET_STORE` | Leer secretos de un almacén externo | Un almacén de secretos que configura una organización |
| `IDENTITY_PROVIDER` | Iniciar sesión con OIDC, SAML o LDAP | Una fuente de autenticación que configura una organización |
| `UPDATE_CHECK` | Buscar una versión más nueva | `updateCheck.enabled: true` |
| `CATALOG` | Instalar desde un catálogo remoto de conectores y refrescar sus metadatos | Un administrador instala un paquete de un catálogo remoto |
| `FLOW` | Lo que hacen tus flujos | Tus flujos, credenciales y disparadores |

Las conexiones a la base de datos SQLite, y cualquier cosa en la dirección de bucle local, no son conexiones de salida.

## La consulta de versiones

Está apagada por defecto (`updateCheck.enabled: false`). Solo puedes activarla en el archivo de configuración, en el entorno o con una bandera; no hay ningún interruptor en la interfaz web. Cuando está activada, más o menos cada 24 horas (con una variación aleatoria de una hora) Orcon envía una petición `GET` simple a la lista pública de versiones (`updateCheck.url`) con el agente de usuario `orcon`, sin versión, sin cookies, sin parámetros de consulta y sin ningún dato de tu instalación. Compara la respuesta en local y avisa a los administradores de la instancia cuando existe una versión estable más nueva. El servidor al que preguntas puede ver la dirección desde la que llega tu petición.

## El catálogo de conectores

Una instancia que nunca instaló un paquete de un catálogo remoto nunca contacta con uno. Los paquetes que vienen preinstalados en la imagen, o que instalas desde un archivo de paquete, nunca programan un refresco. Su información de revocación es tan reciente como el paquete o la versión que los trajo.

Cuando un administrador instala un paquete de un catálogo remoto, Orcon refresca los metadatos de ese catálogo cada `connectors.trust.metadataRefresh` (6 horas por defecto) mientras siga instalado cualquier paquete así. El refresco no lleva ningún identificador de instalación. Con el valor `0` el refresco se apaga, y entonces las revocaciones llegan solo cuando actualizas a mano. Consulta [Confianza en conectores](../security/connector-trust.md).

## Instaladores

Los instaladores de una línea contactan `github.com`, `objects.githubusercontent.com` y el servidor de la raíz de confianza de Sigstore que usa cosign. Consulta [Instalar el binario único](../install/binary.md).

## Cómo se prueba

Pruebas automáticas comprueban la garantía en cada candidata a versión, tras publicar y cada noche:

- Una captura de paquetes de la imagen de la versión, en una red aislada, durante una instalación nueva, la primera ejecución, la carga de la interfaz web, la ejecución de un flujo de muestra y 15 minutos en reposo, falla si el proceso de Orcon inicia una sola conexión hacia una dirección que no sea de bucle local ni del arnés de pruebas.
- Un registro de peticiones del navegador comprueba que la interfaz web no pide nada a otro origen: ni fuentes, ni scripts, ni imágenes externas.
- Una ejecución en proceso con el reloj adelantado 48 horas, con los valores por defecto, no registra ningún intento de conexión de salida. La misma prueba comprueba que cada clave que activa un propósito activa exactamente ese propósito, incluidos los casos del catálogo de arriba.

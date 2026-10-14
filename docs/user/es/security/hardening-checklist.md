---
title: Lista de comprobación de endurecimiento
description: Una lista de comprobación para revisar antes de que una instalación de Orcon contenga datos reales.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0046
  - ADR-0048
  - ADR-0033
sourceHash: 1ebfe51073dd6efc67ea4d4daa1ec2f7d3ae73a791516ec3a4a31db46569327a
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Lista de comprobación de endurecimiento

> Esta página describe un comportamiento especificado por el diseño de seguridad aceptado (del ADR 0038 al ADR 0051) y por el ADR 0083 que aún no está implementado. Algunos puntos se refieren a comandos y ajustes que entregan cortes posteriores. Cada punto nombra la página que lo explica.

Repasa esta lista una vez antes de que la instalación contenga datos reales, y de nuevo tras cambios importantes. Los valores por defecto de Orcon ya son seguros: el escuchador está en la dirección de bucle local y las métricas, los exportadores y la consulta de versiones están apagados. La lista trata de lo que añades tú.

## Instalación

- [ ] La versión que instalaste fue verificada. [Verificar publicaciones](../install/verifying-releases.md)
- [ ] Usas una versión fija de la imagen o del chart, no una etiqueta móvil.
- [ ] `orcon config check` termina sin errores y sin avisos que no entiendas. [Resumen de configuración](../configuration/overview.md)
- [ ] Los secretos se dan en archivos, no en valores, banderas ni en el entorno. Los archivos de secretos solo los puede leer la cuenta que ejecuta Orcon. [Secretos](../configuration/secrets.md)

## Red

- [ ] Los usuarios llegan a Orcon solo por HTTPS, y `publicUrl` es la dirección HTTPS. [TLS y proxy inverso](../configuration/tls-and-reverse-proxy.md)
- [ ] El escuchador de operaciones (`ops.address`) es accesible solo desde tu sistema de monitoreo, nunca desde internet.
- [ ] Si hay un proxy delante de Orcon, `http.trustedProxies` lista exactamente esos proxies.
- [ ] Sabes a qué se conecta Orcon. [Conexiones de red](../configuration/network-connections.md)
- [ ] La consulta de versiones sigue apagada salvo que la quieras. Si la activas, aceptas que el servidor al que preguntas ve tu dirección.

## Claves

- [ ] El kit de recuperación está guardado lejos de este equipo, y más de una persona de confianza sabe dónde. [Clave maestra y KMS](../configuration/master-key-and-kms.md)
- [ ] El proveedor de la clave maestra es `file` con un archivo que solo Orcon puede leer, o un servicio de gestión de claves. Evita `env`.
- [ ] Con un servicio de gestión de claves, la clave tiene activadas la protección contra borrado y la rotación de claves.
- [ ] Decidiste si configurar una clave pública de recuperación (`keys.recoveryPublicKey`).
- [ ] La rotación de las claves de organización sigue su calendario por defecto.

## Respaldos

- [ ] Hay un calendario de respaldos configurado, y `backup.keys.target` es un lugar distinto de `backup.target`. [Respaldo y restauración](../operations/backup-and-restore.md)
- [ ] `backup.keys.recipients` lista los destinatarios age que tienes, para que los respaldos lleven material de claves y no generen el aviso `KEY_MATERIAL_NOT_EXPORTED`.
- [ ] Verificaste un respaldo restaurándolo en un entorno de pruebas.

## Personas y permisos

- [ ] Cada organización exige un segundo factor (`require2FA`), o inicio de sesión único, como política. [Autenticación y SSO](authentication-and-sso.md)
- [ ] Existen como máximo dos usuarios de emergencia (break-glass), cada uno con una clave de acceso.
- [ ] Los operadores no tienen `execution:read-data` salvo que lo necesiten. [Autorización](authorization.md)
- [ ] Pocas personas son propietarias o administradoras de la instancia.
- [ ] Las cuentas de servicio y las claves de API están limitadas a los permisos que necesitan.

## Aislamiento

- [ ] Con más de una organización, usas PostgreSQL, no SQLite. [Varias organizaciones](multi-organization.md)

## Conectores y credenciales

- [ ] El perfil de confianza en conectores se ajusta a tu política. Usa `STRICT` y `locked` si solo pueden ejecutarse conectores revisados. [Confianza en conectores](connector-trust.md)
- [ ] El nodo HTTP genérico tiene una lista explícita de servidores permitidos en cada credencial.
- [ ] Los secretos que ya viven en un almacén propio se referencian desde allí. [Credenciales y almacenes de secretos](credentials-and-secret-stores.md)

## Auditoría

- [ ] Hay al menos un destino de auditoría externo configurado, para poder detectar que se quitaron los eventos más recientes. [Auditoría y SIEM](audit-and-siem.md)
- [ ] Las alertas del kit de monitoreo están cargadas, en particular `OrconAuditVerificationFailed`, `OrconBackupMissing` y `OrconKeyUnwrapFailures`. [Alertas](../operations/alerts.md)

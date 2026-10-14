---
title: Resumen de seguridad
description: Cómo protege Orcon tus datos, quién puede hacer qué y dónde leer los detalles.
sources:
  - ADR-0038
  - ADR-0042
  - ADR-0046
  - ADR-0048
  - ADR-0083
sourceHash: b9195b00d9dd6c83de1d54bfd4f7560a48711b12e06a9d52b20fb985dc38a670
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Resumen de seguridad

> Esta página describe un comportamiento especificado por el diseño de seguridad aceptado (del ADR 0038 al ADR 0051) y por el ADR 0083 que aún no está implementado. Los cortes que entreguen cada parte quitarán esta nota.

Esta página es un mapa del modelo de seguridad de Orcon. Cada sección dice qué promete Orcon y te envía a la página con los detalles y los ajustes.

## Ninguna capacidad se reserva

Toda capacidad de seguridad descrita aquí viene en el motor con licencia Apache-2.0: inicio de sesión único, roles, auditoría, cifrado y aislamiento. Ninguna depende de una edición, una clave de licencia o un pago. Una plataforma construida sobre Orcon solo puede endurecer la política con la misma configuración que usas tú.

## Los bloques

| Área | Qué hace Orcon | Más información |
| --- | --- | --- |
| Organizaciones | Una instancia contiene organizaciones, y una organización contiene proyectos. Las organizaciones son fronteras firmes: un miembro de una no puede ver otra, y cada fila de datos de cliente lleva su organización. | [Varias organizaciones](multi-organization.md) |
| Inicio de sesión | Correo y contraseña con un segundo factor (una app de autenticación o una clave de acceso), más OIDC, SAML, LDAP y SCIM para las organizaciones que quieren inicio de sesión único. | [Autenticación y SSO](authentication-and-sso.md) |
| Permisos | Roles a nivel de organización y de proyecto, roles personalizados y un permiso aparte para ver los datos de las ejecuciones. Se deniega lo que no se concede. | [Autorización](authorization.md) |
| Credenciales | Los secretos que guardas no se vuelven a mostrar. Un flujo usa una credencial sin poder leerla. Se admiten almacenes de secretos externos. | [Credenciales y almacenes de secretos](credentials-and-secret-stores.md) |
| Cifrado | Capas de claves bajo una clave maestra que controlas tú. | [Cifrado y claves](encryption-and-keys.md) |
| Auditoría | Un registro a prueba de manipulación de quién hizo qué, exportable a tus herramientas de seguridad. | [Auditoría y SIEM](audit-and-siem.md) |
| Conectores | Los paquetes de conectores están firmados y revisados, y tú eliges la política de confianza. | [Confianza en conectores](connector-trust.md) |
| Red | Orcon no hace ninguna conexión de salida que nadie haya pedido. | [Conexiones de red](../configuration/network-connections.md) |

## Administradores de la instancia y datos de las organizaciones

Las personas que operan la instancia son distintas de las que son propietarias de una organización. Los roles de instancia (`instanceOwner` e `instanceAdmin`) controlan la clave maestra, la confianza en conectores, el ciclo de vida de las organizaciones, los límites y los respaldos. No dan acceso a los datos de una organización. Para verlos, un administrador de la instancia debe unirse explícitamente a la organización, y eso se audita.

## Política en capas

Los ajustes de seguridad vienen en tres capas: los valores por defecto del motor, luego los pisos que fija la instancia y luego los ajustes de cada organización. Una organización solo puede endurecer lo que fija la instancia. Un ajuste marcado `locked` se lee solo del archivo de configuración o del entorno al arrancar, y no se puede cambiar en la interfaz web ni con la API.

## Integridad de las versiones

Cada versión está firmada y trae procedencia de construcción. Consulta [Verificar publicaciones](../install/verifying-releases.md).

## Qué hacer después

- Aplica la [lista de comprobación de endurecimiento](hardening-checklist.md) a tu instalación.
- Para informar de una vulnerabilidad, consulta [Informar de vulnerabilidades](reporting-vulnerabilities.md).

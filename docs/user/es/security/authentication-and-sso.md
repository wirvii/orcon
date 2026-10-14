---
title: Autenticación y SSO
description: Cómo inician sesión las personas en Orcon, y cómo una organización puede exigir autenticación de dos factores o inicio de sesión único.
sources:
  - ADR-0038
  - ADR-0039
  - ADR-0040
sourceHash: e91c271b3024be3a17abefda45aa55a719e5062bbc0bf3e36018c40048d74cdf
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Autenticación y SSO

> Esta página describe un comportamiento especificado por el ADR 0038, el ADR 0039 y el ADR 0040 que aún no está implementado. Los cortes que entreguen el inicio de sesión y la identidad federada quitarán esta nota.

Una persona tiene una sola cuenta de Orcon para toda la instancia, y puede ser miembro de varias organizaciones. Cada organización decide cómo deben demostrar quiénes son sus miembros.

## Inicio de sesión local

El inicio de sesión local funciona sin ningún servicio externo, y también sin conexión.

- **Contraseñas.** Se guardan con argon2id. La longitud mínima es de 12 caracteres y no hay reglas de composición. Orcon comprueba contra una lista local de contraseñas comunes. Existe una comprobación contra un servicio de contraseñas filtradas, pero está apagada por defecto porque necesita acceso a la red.
- **Protección contra adivinación.** Tras 5 fallos en una cuenta, la espera antes del siguiente intento crece de forma exponencial hasta 15 minutos, y se aplica un límite por dirección. Las cuentas nunca se bloquean de forma permanente. La respuesta es la misma para un correo desconocido, una contraseña incorrecta y una cuenta desactivada.
- **Segundo factor.** Un código de un solo uso basado en el tiempo (TOTP, los códigos de seis dígitos de las apps de autenticación) o una clave de acceso. También recibes 10 códigos de recuperación de un solo uso. Regenerarlos cancela el conjunto anterior.
- **Claves de acceso (passkeys).** Inicia sesión sin contraseña con una clave de acceso que exija verificación del usuario. Un inicio de sesión con clave de acceso cuenta como de dos factores.
- **Flujos por correo.** La verificación del correo y el restablecimiento de la contraseña usan enlaces de un solo uso válidos durante 30 minutos. Si no hay un servidor de correo configurado, un administrador de la instancia puede generar un enlace de restablecimiento, y esa acción se audita.
- **Segundo factor perdido.** Un administrador de la instancia ejecuta `orcon user reset-2fa <correo>`. La acción se audita y termina las sesiones del usuario.

## Sesiones

Las sesiones se guardan en el servidor, así que se pueden terminar al instante. La cookie es `__Host-orcon_session`. Por defecto una sesión termina tras 8 horas sin actividad o tras 7 días en total; una organización puede exigir tiempos más cortos. Cambiar una contraseña, restablecer el segundo factor, quitar a una persona de una organización o suspender una pertenencia termina las sesiones afectadas. Las peticiones que cambian estado necesitan un origen coincidente y un token CSRF.

## Inicio de sesión único para una organización

Cada organización configura sus propias fuentes de identidad, con los secretos guardados cifrados con la clave de la organización.

| Fuente | Notas |
| --- | --- |
| OIDC | Código de autorización con PKCE. El token de identidad se valida, incluidos emisor, audiencia, caducidad y firma con un algoritmo permitido. |
| SAML 2.0 | Empieza en Orcon por defecto. El inicio de sesión que empieza en el proveedor de identidad está apagado por defecto; activarlo muestra una advertencia de riesgo y se audita. Las aserciones deben estar firmadas, se aceptan una sola vez y se comprueban contra repeticiones. El cierre de sesión único no forma parte de la versión 1. |
| LDAP y Active Directory | Exige LDAPS o StartTLS. Orcon se autentica como el usuario y nunca guarda la contraseña. La pertenencia a grupos se sincroniza cada hora por defecto. |
| SCIM 2.0 | Aprovisionamiento en `/scim/v2/orgs/{orgId}/`, con un token portador por organización. Desactivar a un usuario termina su acceso a la organización y desactiva sus claves de API allí en menos de un segundo. |

Orcon vincula una identidad externa a una cuenta por la propia identidad, o por correo solo cuando el correo está verificado y la organización tiene verificado el dominio (con un registro DNS TXT). Los usuarios se pueden crear en el primer inicio de sesión solo para dominios verificados, con el rol Member por defecto. La página de inicio de sesión de una organización está en `/login/<orgSlug>`.

## Política de la organización

Una organización puede definir:

- `require2FA`: los miembros necesitan un segundo factor. Durante un periodo de gracia (7 días por defecto) a los miembros sin él se les pide en cada visita, y pasado ese periodo se les deniega el acceso.
- `requireSSO` y `acceptedAuthSources`: qué fuentes se aceptan.
- `trustIdpMfaClaim` (apagado por defecto), `maxSessionAge` e `idleTimeout`.
- `breakGlassUsers`: hasta dos propietarios con clave de acceso, exentos de `requireSSO` para que un fallo del proveedor de identidad no deje a todos fuera.

Una sesión del proveedor de identidad de una organización no da acceso a otra organización, a menos que esa organización acepte la fuente.

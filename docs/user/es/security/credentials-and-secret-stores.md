---
title: Credenciales y almacenes de secretos
description: Cómo guarda Orcon las credenciales que usan tus flujos, por qué nadie puede volver a leerlas y cómo guardar los secretos en un almacén externo.
sources:
  - ADR-0043
  - ADR-0044
  - ADR-0045
sourceHash: f915666efc98a4a0261dbdcc4e6c5da724fd09f7529217e4c420208651fba99c
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Credenciales y almacenes de secretos

> Esta página describe un comportamiento especificado por el ADR 0043, el ADR 0044 y el ADR 0045 que aún no está implementado. Los cortes que entreguen el almacén de credenciales quitarán esta nota.

Una credencial contiene lo que un flujo necesita para llamar a otro servicio: un token, una contraseña, una clave de API. Orcon guarda los secretos para que se puedan usar, y nunca para mostrarlos.

## Solo escritura

Después de guardar un campo secreto, ninguna pantalla, respuesta de la API ni exportación se lo devuelve a nadie. Orcon muestra solo que el campo está definido, cuándo se actualizó y quién lo hizo. Actualizar un campo reemplaza el valor y crea una versión nueva de la credencial. Los campos públicos, como un nombre de usuario o una dirección base, pueden leerlos los usuarios con el permiso `credential:read`.

Ni siquiera los administradores de la instancia pueden leer los secretos a través de Orcon.

## Usar sin leer

Un proyecto puede usar una credencial sin poder leerla. Cuando se ejecuta un nodo de un flujo, el motor comprueba que la credencial pertenece al proyecto del flujo o se compartió con él, descifra el valor en memoria y entrega al conector un identificador de acceso. La comprobación se repite en cada uso. Los valores se guardan en una caché por proceso durante 60 segundos como máximo y se borran al salir de ella.

## Ligadas a servidores

Cada credencial tiene una lista de servidores a los que puede enviarse. Para un conector, por defecto son los servidores de ese servicio. Para el nodo HTTP genérico, el propietario debe definirlos. Orcon aplica la credencial solo a las peticiones cuyo servidor coincide; cualquier otra petición falla con `CredentialHostNotAllowed`. El acceso en bruto a una credencial, que se salta esta vinculación, debe aprobarse para la organización.

## Enmascarado

Todo secreto que resuelve un nodo se añade al conjunto de enmascarado de esa ejecución. Las coincidencias exactas se sustituyen en registros, mensajes de error, eventos, historial y contenido de notificaciones.

## Auditoría

Orcon registra un evento `credential.used` por cada ejecución de nodo y credencial, con el número de peticiones y los servidores de destino. No registra un evento por cada petición.

## Credenciales OAuth

Orcon ejecuta el flujo de consentimiento (código de autorización con PKCE donde el proveedor lo admite, o credenciales de cliente) y guarda los tokens como campos secretos. Un token se renueva cuando caduca en menos de 60 segundos. Exactamente un proceso del clúster renueva una credencial dada cada vez, y los demás esperan el resultado, de modo que los proveedores que rotan los tokens de renovación no se invalidan entre sí. Si el proveedor responde `invalid_grant`, la credencial pasa a `NEEDS_REAUTH`, se avisa a su propietario y los envíos fallan con un error que nombra la credencial.

Queda un riesgo que Orcon no puede cerrar: si el proveedor rota el token de renovación y el proceso se cae después de que el proveedor respondió pero antes de guardar el token nuevo, el token de renovación nuevo se pierde y debes dar el consentimiento otra vez.

## Almacenes de secretos externos

Una organización puede guardar secretos en su propio almacén y hacer que Orcon los lea cuando los necesite. Orcon no guarda de forma persistente ningún valor que lea así.

| Almacén | Autenticación |
| --- | --- |
| HashiCorp Vault u OpenBao (KV v1 y v2) | Token, AppRole, Kubernetes, JWT u OIDC |
| AWS Secrets Manager | Claves estáticas o un rol asumido |
| Google Cloud Secret Manager | Clave de cuenta de servicio o identidad de carga de trabajo |
| Azure Key Vault | Secreto de cliente o identidad administrada |
| Infisical | Identidad de máquina (autenticación universal) |

Cada almacén tiene un alias en la organización. Los alias por defecto son `vault`, `awsSecretsManager`, `gcpSecretsManager`, `azureKeyVault` e `infisical`. Gestionar los almacenes necesita el permiso `secret-store:manage`, y referenciarlos necesita `secret-store:reference`.

Referencia un secreto en un campo de una credencial con `{{ $secrets.<alias>.<nombre> }}`, seguido opcionalmente de `.<versión>`. Solo puedes usar la referencia en campos de credenciales. Publicar un flujo falla si aparece una referencia en un parámetro de nodo o en una expresión de datos.

Los valores se guardan en memoria durante 5 minutos por defecto (0 apaga la caché). Si el almacén no está disponible, se sirve el valor guardado hasta una hora por defecto (0 lo desactiva) y después se produce un error transitorio. Los valores resueltos se suman al conjunto de enmascarado. Una resolución fallida se audita.

Un servidor que se ejecuta en una nube puede autenticarse con su propia identidad de nube. Esa identidad pertenece a la instancia. Una organización puede usarla solo después de que un administrador de la instancia se la conceda de forma explícita, y la concesión se audita.

<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Conectar <Service> con Orcon

<!-- Copiar a docs/connection-guide.es.md en la carpeta del conector,
sustituir cada marcador <...> y conservar todos los encabezados: el linter de
admisión los exige (DC-4). Si el servicio no usa OAuth, conservar el
encabezado "URL de redirección" y escribir "No hace falta: esta conexión usa
una clave de API." -->

## Antes de empezar

Necesitas una cuenta de <Service> con permiso para <crear claves de API | registrar aplicaciones>. <Indica si hace falta un plan de pago.>

## Crear la credencial en <Service>

1. Inicia sesión en <Service> y abre <ruta exacta del menú>.
2. Elige <nombre del botón> y da a la clave o a la aplicación un nombre que indique que la usa Orcon.
3. Copia <la clave | el ID y el secreto de cliente>. <Service> los muestra una sola vez.

## Alcances o permisos que conceder

Concede solo lo que necesitan las operaciones que usas:

| Operación | Alcance o permiso |
|---|---|
| <resource.verb> | <alcance> |

## URL de redirección

Introduce `https://<your-orcon-host>/oauth/callback` como URL de redirección de la aplicación.

## Introducirla en Orcon

1. En Orcon, abre **Credenciales** y elige **Nueva credencial**.
2. Elige **<Service>** y el tipo de credencial <nombre visible del tipo>.
3. Pega los valores de los pasos anteriores y guarda. Orcon los guarda cifrados y no vuelve a mostrarlos.

## Probar la conexión

Elige **Probar**. Orcon ejecuta <nombre visible de la operación de prueba>; un mensaje de éxito indica que la credencial funciona.

## Resolución de problemas

- **AuthInvalid**: la clave se revocó o se copió incompleta. Crea una nueva.
- **PermissionDenied**: falta un alcance de la tabla anterior. Añádelo y vuelve a autorizar.
- <Otro problema frecuente y qué hacer.>

<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Conectar Acme con Orcon

## Antes de empezar

Necesitas una cuenta de Acme con permiso para crear claves de API. Sirve cualquier plan.

## Crear la credencial en Acme

1. Inicia sesión en Acme y abre **Settings > API keys**.
2. Elige **New key** y ponle un nombre que indique que la usa Orcon.
3. Copia la clave. Acme la muestra una sola vez.

## Alcances o permisos que conceder

Concede solo lo que necesitan las operaciones que usas:

| Operación | Alcance o permiso |
|---|---|
| contact.create, contact.update, contact.delete | `contacts:write` |
| contact.get, contact.getMany | `contacts:read` |
| message.send | `messages:send` |

## URL de redirección

No hace falta: esta conexión usa una clave de API.

## Introducirla en Orcon

1. En Orcon, abre **Credenciales** y elige **Nueva credencial**.
2. Elige **Acme** y el tipo de credencial **Clave de API de Acme**.
3. Pega la clave y guarda. Orcon la guarda cifrada y no vuelve a mostrarla.

## Probar la conexión

Elige **Probar**. Orcon lista un contacto; un mensaje de éxito indica que la clave funciona.

## Resolución de problemas

- **AuthInvalid**: la clave se revocó o se copió incompleta. Crea una nueva.
- **PermissionDenied**: falta un alcance de la tabla anterior. Edita la clave en Acme y añádelo.
- **RateLimited**: Acme admite 10 peticiones por segundo por clave. Orcon espera y reintenta automáticamente.

<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Conectar WhatsApp Cloud API con Orcon

## Antes de empezar

Necesitas una cuenta de Meta Business con una cuenta de WhatsApp Business y un número de teléfono registrado para la Cloud API. Los mensajes que envías a clientes siguen la política de mensajería de WhatsApp Business: el cliente debe haber dado su consentimiento (opt-in), y una conversación iniciada por la empresa debe usar una plantilla aprobada fuera de la ventana de atención de 24 horas. Orcon no aplica estas reglas; el flujo que envía el mensaje debe aplicarlas.

## Crear la credencial en Meta

1. En Configuración de Meta Business, abre **Usuarios > Usuarios del sistema** y crea un usuario del sistema con el rol **Administrador**.
2. Asigna al usuario del sistema la cuenta de WhatsApp Business y la aplicación, con los permisos `whatsapp_business_messaging` y `whatsapp_business_management`.
3. Selecciona **Generar nuevo token**, elige la aplicación y marca los mismos dos permisos. Copia el token. Meta lo muestra una sola vez.
4. En WhatsApp Manager, abre el número de teléfono y copia su **ID del número de teléfono** (solo dígitos, no el número en sí).

## Permisos que debes conceder

| Operación | Permiso |
|---|---|
| message.send, templateMessage.send, imageMessage.send, documentMessage.send, message.update | `whatsapp_business_messaging` |
| phoneNumber.get | `whatsapp_business_management` |

## URL de redirección

No se necesita: esta conexión usa un token de usuario del sistema.

## Introducirla en Orcon

1. En Orcon, abre **Credenciales** y selecciona **Nueva credencial**.
2. Elige **WhatsApp Cloud API** y el tipo de credencial **Token de acceso del usuario del sistema**.
3. Pega el token y guarda. Orcon lo almacena cifrado y no lo vuelve a mostrar.
4. En cada nodo, completa **ID del número de teléfono** con el valor copiado en el paso 4 anterior.

## Probar la conexión

Selecciona **Probar**. Orcon consulta el número de teléfono con su ID; el número visible y la calificación de calidad aparecen cuando el token y el ID son correctos.

## Solución de problemas

- **AuthInvalid**: el token venció, fue revocado o se copió incompleto. Genera un nuevo token para el usuario del sistema.
- **PermissionDenied**: el usuario del sistema no tiene el número o el permiso de la tabla anterior. Asígnalos en Configuración de Meta Business.
- **ValidationFailed**: Meta rechazó la petición, por ejemplo un destinatario fuera de la ventana de 24 horas sin plantilla, una plantilla que falta o un enlace de medio incorrecto. Lee el mensaje en el registro de ejecución.
- **RateLimited**: Meta pidió reducir el ritmo. Orcon espera antes de la siguiente petición; un mensaje nunca se reenvía automáticamente.
- **UNCERTAIN (suspendido)**: la conexión falló después de que el mensaje pudo haberse enviado. Comprueba el chat en WhatsApp Business como indica el registro de ejecución y decide si el mensaje se envió. Orcon no lo reenvía por su cuenta.

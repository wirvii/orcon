<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Acme: equivalencia con n8n

Este conector sustituye al nodo de n8n `n8n-nodes-base.acme` (versión de tipo 1). Los flujos importados siguen funcionando sin cambios, salvo por las diferencias siguientes.

## Operaciones

| n8n | Orcon | Diferencia |
|---|---|---|
| Contact: Create | `contact.create` | ninguna |
| Contact: Get | `contact.get` | ninguna |
| Contact: Get All | `contact.getMany` | verbo renombrado; `returnAll` y `limit` conservan su significado |
| Contact: Update | `contact.update` | ninguna |
| Contact: Delete | `contact.delete` | ninguna |
| Message: Send | `message.send` | ninguna |

## Parámetros y valores por defecto

- `limit` vale 50 por defecto, como en n8n.
- `updatedAfter` es nuevo; los flujos importados lo dejan vacío.

## Seguridad al repetir

n8n reintenta todas las operaciones de la misma forma. En Orcon, `message.send` nunca se repite automáticamente: si Acme responde con un error del servidor después de enviar el mensaje, se pide a una persona que confirme si se entregó.

## Operaciones que faltan

Ninguna.

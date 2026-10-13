---
title: Autorización
description: Los roles y permisos de Orcon, cómo funcionan los roles personalizados y cómo se separan los datos de las ejecuciones de su estado.
sources:
  - ADR-0041
  - ADR-0038
  - ADR-0049
sourceHash: 4d4b77581ac27a03ddad25d1131d419194a87b50dc858873862d3c5c8d74397c
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Autorización

> Esta página describe un comportamiento especificado por el ADR 0041 y el ADR 0049 que aún no está implementado. Los cortes que entreguen la autorización quitarán esta nota.

Orcon deniega todo lo que no se ha concedido. En la versión 1 no hay reglas de denegación explícitas. Un permiso se comprueba en cuatro niveles, del más amplio al más estrecho: la instancia, la organización, el proyecto y el recurso.

## Roles

Los roles predefinidos no se pueden editar.

| Nivel | Rol | Qué puede hacer |
| --- | --- | --- |
| Organización | Owner | Todo en la organización y en todos sus proyectos, incluido eliminar la organización y transferir la propiedad. |
| Organización | Admin | Lo mismo que Owner, sin eliminar la organización ni transferir la propiedad. |
| Organización | Member | Ver la organización, sus miembros y roles, crear proyectos (una organización puede desactivarlo) y referenciar almacenes de secretos externos. El acceso a los proyectos viene solo de los roles de proyecto. |
| Proyecto | Project Admin | Todos los permisos en ese proyecto. |
| Proyecto | Editor | Crear, editar, publicar, activar, revertir y ejecutar flujos; trabajar con ejecuciones salvo borrarlas; resolver tareas humanas; gestionar las credenciales y las variables del proyecto. |
| Proyecto | Operator | Ver flujos y ejecuciones; activar flujos; reintentar y cancelar ejecuciones; resolver tareas humanas. No puede editar y, por defecto, no puede ver los datos de las ejecuciones. |
| Proyecto | Viewer | Solo lectura: el proyecto, los flujos, las ejecuciones, las credenciales (solo metadatos) y las variables. |

## Los datos de las ejecuciones son un permiso aparte

Ver que una ejecución se realizó no es lo mismo que ver lo que procesó. El permiso `execution:read` muestra estado, tiempos, intentos y errores depurados. El permiso `execution:read-data` hace falta para ver elementos, entradas y salidas, cargas, valores de variables y blobs, para cargar datos pasados en el editor y para exportar. Sin él, las respuestas de la API omiten los datos y lo indican con `dataHidden: true`. Cada consulta o exportación de datos protegidos se audita.

El rol Operator no tiene `execution:read-data`. Si tus operadores lo necesitan, crea un rol personalizado: por ejemplo, «Operator with data» es Operator más `execution:read-data`.

## Roles personalizados

Una organización puede crear roles a partir del catálogo de permisos. Nadie puede crear, cambiar ni asignar un rol que incluya un permiso que no tiene él mismo en ese ámbito, así que los roles no sirven para escalar privilegios. Una instancia puede desactivar los roles personalizados, o quitar permisos de todos los roles, pero solo para endurecer.

## Grupos y compartición

Los roles se pueden asignar a usuarios, a grupos y a cuentas de servicio. Una credencial pertenece a un proyecto. Su propietario puede compartirla con otro proyecto de la misma organización, y el otro proyecto obtiene el derecho de usarla y nada más. Compartir nunca cruza organizaciones. La comprobación se repite en cada uso.

## Permisos de instancia

Los roles de instancia son independientes de los roles de organización. `instanceOwner` e `instanceAdmin` tienen los permisos de instancia: `instance:orgs`, `instance:keys`, `instance:connectors`, `instance:policy`, `instance:limits`, `instance:siem`, `instance:users`, `instance:backups` e `instance:maintenance`. Estos roles no dan acceso a los datos de las organizaciones. En una instalación de una sola organización, el primer usuario recibe `instanceOwner` y es el Owner de la organización.

## Cuándo surten efecto los cambios

Cambiar un rol, una asignación, un grupo, una compartición o la política de una organización surte efecto en todo el clúster en menos de un segundo. Las operaciones sensibles vuelven a comprobar la base de datos dentro de su propia transacción.

## Acciones desde notificaciones

Un enlace de un correo o de un webhook nunca autoriza una acción por sí solo. Una persona que actúa sobre una tarea humana desde una notificación debe iniciar sesión en Orcon y tener el permiso para esa acción en el proyecto de la ejecución. Las notificaciones llevan identificadores, nombres y un enlace, no valores secretos ni, por defecto, datos de ejecución.

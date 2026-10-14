---
title: Varias organizaciones
description: Cómo mantiene Orcon separados los datos de una organización y de otra en la misma instancia, y qué puede garantizar cada base de datos.
sources:
  - ADR-0038
  - ADR-0046
  - ADR-0047
sourceHash: 407bbd8911b432cd2b9b9582b41fc5334be8ccb5e777a6f9bcd671952542783a
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Varias organizaciones

> Esta página describe un comportamiento especificado por el ADR 0038 y el ADR 0046 que aún no está implementado. Los cortes que entreguen las organizaciones y el aislamiento quitarán esta nota.

Una instancia de Orcon puede servir a varias organizaciones. Una organización es una frontera firme: sus flujos, credenciales, ejecuciones y claves son invisibles para las demás.

## Organizaciones, proyectos y personas

La estructura es instancia, organización, proyecto, recursos. Una instalación nueva de una sola organización crea una organización llamada `default`, y la interfaz web oculta la elección de organización hasta que existe una segunda. Cada organización tiene un proyecto por defecto. Una persona tiene una cuenta en la instancia y una pertenencia en cada organización a la que pertenece. Un usuario sin una pertenencia activa no puede ver la organización ni saber que existe.

Las organizaciones las crean los administradores de la instancia, salvo que la instancia active `orgs.selfService`. Una organización se puede suspender: sus disparadores se detienen, se deniegan las sesiones y los datos se conservan. Eliminarla inicia un periodo de gracia de 30 días, durante el cual un administrador de la instancia puede cancelar. Pasado ese plazo se destruyen sus claves y se eliminan sus filas y blobs por lotes.

## Cómo se impone el aislamiento

Cada fila de datos de cliente pertenece a una organización, y la base de datos rechaza una fila que apunte a un padre de otra organización. Además, Orcon aplica cuatro comprobaciones independientes:

1. **La capa de datos.** Todas las consultas pasan por una sola capa, y no se puede escribir una consulta de datos de una organización sin indicar la organización.
2. **Comprobaciones estáticas en la construcción.** La construcción falla si el código consulta la base de datos fuera de esa capa o si se lee una tabla de organización sin restringirla a su organización.
3. **Seguridad a nivel de fila en PostgreSQL.** Cada tabla impone la organización en la propia base de datos. El rol de ejecución no puede eludirla y no es propietario de ninguna tabla.
4. **Un conjunto de pruebas de aislamiento** que intenta leer, cambiar y referenciar los recursos de una organización desde otra, por todas las rutas y todos los métodos del almacén, en SQLite y en PostgreSQL. Forma parte de las compuertas de seguridad de cada versión.

Otras fronteras siguen la misma regla: las cachés, las rutas de blobs, el índice de rutas de webhooks y las llamadas a subflujos nunca cruzan organizaciones.

## Qué puede y qué no puede hacer SQLite

SQLite no tiene seguridad a nivel de fila, así que en SQLite solo se aplican las comprobaciones 1, 2 y 4. Crear una segunda organización en SQLite muestra una advertencia en la interfaz web, se registra y se audita. Para más de una organización, usa PostgreSQL. También es posible ejecutar una instancia por cada cliente. Cuando una instancia tiene dos o más organizaciones en PostgreSQL, Orcon se niega a arrancar, y se niega a crear la segunda organización, si el rol de ejecución puede eludir la seguridad a nivel de fila, es superusuario, es propietario de tablas, o si a una tabla de organización le falta la política. Con una sola organización, avisa.

La seguridad a nivel de fila protege contra un filtro ausente o incorrecto en las propias consultas de Orcon. No protege contra SQL arbitrario ejecutado como el rol de la aplicación.

## Roles de PostgreSQL

Orcon usa un rol propietario que ejecuta las migraciones, un rol de aplicación para el trabajo normal y un rol de motor para unas pocas funciones internas. Si tu usuario de base de datos puede crear roles, Orcon crea los otros dos. Si no, `orcon db setup-roles` imprime el SQL para tu administrador de bases de datos.

## Ajustes por organización

Las fuentes de autenticación, la política de seguridad, los destinos de auditoría, los almacenes de secretos, los límites y las claves son por organización. Una organización solo puede endurecer los pisos que fija la instancia. Consulta [Autenticación y SSO](authentication-and-sso.md).

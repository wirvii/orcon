---
title: Actualización y vuelta atrás
description: Pasa una instancia a una versión nueva de Orcon, con poca o ninguna parada, y vuelve atrás si lo necesitas.
sources:
  - ADR-0083
  - ADR-0015
  - ADR-0004
sourceHash: 254ce1febfd08c9cd8b600e3b79150d331c22f5175da607445b057d21032dbb9
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Actualización y vuelta atrás

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue `orcon store schema` quitará esta nota.

Esta página muestra cómo pasar una instancia en marcha a una versión nueva y cómo volver atrás. El esquema de la base de datos solo avanza. Lo que decide si puedes actualizar sin parar nada, y si puedes volver atrás ejecutando el binario anterior, es el piso de compatibilidad.

## El piso de compatibilidad

Cada migración declara la versión de Orcon más antigua que funciona correctamente con el esquema de la base de datos una vez aplicada esa migración. La más alta de esas versiones entre todas las migraciones aplicadas es el piso de compatibilidad efectivo. Un proceso más antiguo que el piso no debe seguir trabajando sobre la base de datos.

- Si todos los procesos en marcha están en el piso nuevo o por encima, la actualización es gradual: reemplazas los procesos de uno en uno y nada se detiene.
- Si algún proceso en marcha quedaría por debajo del piso nuevo, la actualización exige una parada completa: detienes todo, migras y vuelves a arrancar.

Las notas de cada versión dicen cuál es el caso, en una línea: `Upgrade: rolling (floor vX.Y.Z)` o `Upgrade: requires full stop (floor vX.Y.Z)`.

## Comprueba antes de actualizar

Ejecuta estos comandos con el binario nuevo contra tu base de datos. En SQLite, detén antes la instancia.

```sh
orcon store schema status
orcon store schema plan
```

`status` muestra la versión del esquema, el piso efectivo, las migraciones ya aplicadas y los procesos en marcha con su versión y su rol. `plan` muestra lo que cambiaría el binario nuevo y da un veredicto. Por ejemplo:

```text
Schema: 0042 (floor v0.6.0)  Binary: v0.8.0 (migrations up to 0047)
Pending: 0043 additive floor v0.7.0; 0044 expand floor v0.7.0; 0045..0047 additive floor v0.7.0
Resulting floor: v0.7.0
Live processes: api v0.7.2 (2), scheduler v0.7.2 (2), worker v0.6.4 (1)
Verdict: REQUIRES_FULL_STOP — worker on host w-3 (v0.6.4) is below v0.7.0
```

El veredicto es `ROLLING_SAFE` cuando todos los procesos en marcha quedan en el piso resultante o por encima, y `REQUIRES_FULL_STOP` en caso contrario. En el segundo caso la salida lista los procesos que quedarían por debajo.

## Actualiza

1. Lee las notas de la versión.
2. Haz un respaldo. Consulta [Respaldo y restauración](backup-and-restore.md). Con SQLite, Orcon hace por sí mismo un respaldo local antes de migrar (ver abajo); con PostgreSQL, haz tu propio respaldo antes.
3. Ejecuta `orcon store schema plan` con el binario nuevo.
4. Si el veredicto es `ROLLING_SAFE`, reemplaza los procesos de uno en uno, en cualquier orden. El primer proceso nuevo aplica las migraciones al arrancar.
5. Si el veredicto es `REQUIRES_FULL_STOP`, detén todos los procesos, ejecuta el comando siguiente y arranca la versión nueva.

   ```sh
   orcon store schema apply --full-stop
   ```

   Con `--full-stop` el comando exige que no haya ningún proceso en marcha. Sin esa bandera, el comando se niega a ejecutarse (código de salida 4) cuando el veredicto es `REQUIRES_FULL_STOP`.
6. Comprueba que `/readyz` responde 200 en cada proceso y que la alerta `OrconVersionSkew` se apaga cuando todos los procesos ejecutan la versión nueva.

Puedes saltar versiones menores cuando `orcon store schema plan` dice `ROLLING_SAFE`. Si no lo dice, actualiza versión por versión o con una parada completa.

### Qué pasa al arrancar

La migración automática al arrancar solo se aplica cuando el veredicto es `ROLLING_SAFE`. En otro caso el proceso no arranca, y su mensaje nombra los procesos que quedarían por debajo del piso y el comando `orcon store schema apply --full-stop`. Poner `store.migrations.auto` en `false` desactiva la migración automática.

Cada proceso compara su versión con el piso efectivo en cada latido. Un proceso que queda por debajo deja de tomar trabajo nuevo y peticiones nuevas, termina el trabajo en curso como al apagarse y sale con el código 75. El registro indica la causa.

### SQLite: respaldo automático antes de migrar

Con SQLite, cuando hay migraciones pendientes y `backup.beforeUpgrade` es `true` (el valor por defecto en SQLite), el proceso hace un respaldo local sin conexión en `<dataDir>/backups/pre-upgrade/` antes de migrar. El respaldo es local aunque `backup.target` apunte a S3, para que el arranque nunca dependa de la red. Se conservan los tres más recientes. Con PostgreSQL el valor por defecto es `false`, así que haz antes tu propio respaldo.

### Kubernetes con Helm

El chart ejecuta `orcon store schema apply` como un trabajo antes de cada instalación y actualización, y las réplicas arrancan con `store.migrations.auto` en `false`. Cuando el veredicto es `REQUIRES_FULL_STOP`, el trabajo falla con el mensaje anterior y la actualización se detiene sin tocar los despliegues. Para continuar, escala los despliegues a cero, pon `migrations.fullStop` en `true` y actualiza.

## Vuelta atrás

Cómo vuelves atrás depende del piso.

- **La versión anterior está en el piso efectivo o por encima.** Vuelve a ejecutar la versión anterior. Las migraciones solo avanzan, y la versión anterior acepta el esquema más nuevo. `orcon store schema status` muestra el piso. Con Helm, `helm rollback` cubre este caso.
- **La versión anterior está por debajo del piso efectivo.** Restaura el respaldo que hiciste antes de actualizar, como se describe en [Respaldo y restauración](backup-and-restore.md). Se pierde todo lo escrito después de ese respaldo.

Comprueba el piso antes de actualizar, no después de un fallo: volver atrás restaurando es decidir perder datos recientes.

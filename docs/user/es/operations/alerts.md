---
title: Alertas
description: Qué significa cada alerta de Orcon y qué hacer cuando se dispara.
sources:
  - ADR-0083
  - ADR-0012
  - ADR-0013
  - ADR-0017
  - ADR-0021
  - ADR-0025
  - ADR-0042
  - ADR-0048
  - ADR-0073
sourceHash: 117a3e5ac36e0ce737a1a3992dbb21e0915a49d1a82d83864bbb63b655db8c15
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Alertas

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue el kit de monitoreo quitará esta nota.

El kit de monitoreo de `deploy/monitoring/prometheus/orcon-alerts.yaml` define 29 reglas de alerta. Esta página tiene una sección por regla, con el mismo nombre que la regla. La anotación `runbook_url` de cada regla apunta a su sección, con el nombre de la regla en minúsculas como ancla.

Cada sección da la gravedad, la condición en palabras sencillas y qué hacer. Las expresiones suponen que tu trabajo de Prometheus se llama `orcon`, que es el valor por defecto documentado. Los nombres de las métricas están en `docs/reference/metrics.md`.

Dos comandos ayudan con muchas alertas. `orcon health --ready` pregunta a un proceso local si está listo. `orcon store schema status` lista los procesos que están vivos, con su versión y su rol.

## OrconInstanceDown

Gravedad: crítica. Se dispara cuando Prometheus no puede alcanzar un proceso durante 5 minutos.

Qué hacer:

1. Comprueba que el proceso o el contenedor está en marcha, con tu gestor de servicios o con `kubectl get pods`.
2. En la máquina, ejecuta `orcon health`. Si responde pero Prometheus no, el problema es la ruta de red o la dirección del escuchador de operaciones (`ops.address`).
3. Lee el registro del último arranque. Un proceso que sale con el código 75 quedó por debajo del piso de compatibilidad de la base de datos y debe actualizarse; consulta [Actualización y vuelta atrás](upgrade-and-rollback.md).

## OrconNotReady

Gravedad: advertencia. Se dispara cuando un proceso informa que no está listo durante 5 minutos.

`/readyz` responde 503 con una lista de comprobaciones. Ejecuta `orcon health --ready`, o pide `/readyz`, y lee la comprobación que falla: no se alcanza el almacén, el esquema es más antiguo que el piso de compatibilidad, las claves no están verificadas, el proceso se está apagando o (en un worker o un scheduler) el proceso no tiene un latido reciente. Corrige la comprobación que falla. Un proceso que está drenando a propósito no está listo por diseño.

## OrconHTTPErrorRateHigh

Gravedad: advertencia. Se dispara cuando más del 5 % de las peticiones HTTP respondidas en los últimos 5 minutos tuvieron un estado 5xx, durante 10 minutos.

Qué hacer: mira los registros de los procesos `api` para ver errores en el mismo momento (cada respuesta lleva un id de petición), comprueba que el almacén responde (el tablero de almacenamiento muestra la latencia de transacciones y los errores) y comprueba si los errores empezaron con un despliegue o un cambio de configuración.

## OrconExecutionFailureRatioHigh

Gravedad: advertencia. Se dispara cuando más del 20 % de las ejecuciones que terminaron en los últimos 15 minutos acabaron en `FAILED`, durante 15 minutos.

Qué hacer: lista las ejecuciones fallidas y agrúpalas por flujo para ver si un flujo o un servicio externo explica los fallos. Un fallo amplio suele apuntar a una dependencia compartida: un servicio caído, una credencial vencida o un cambio en la instancia. Las reglas de error y los reintentos de cada flujo deciden qué cuenta como fallo.

## OrconClaimLatencyHigh

Gravedad: advertencia. Se dispara cuando el percentil 99 del tiempo que un paso ejecutable espera a ser reclamado supera los 5 segundos, durante 10 minutos.

Hay trabajo listo, pero los workers tardan en tomarlo. Comprueba que los workers están en marcha y no en pausa (consulta `OrconWorkerAdmissionPaused`), añade workers si todos están ocupados y revisa la latencia de las transacciones del almacén en el tablero de almacenamiento.

## OrconRunnableBacklogGrowing

Gravedad: advertencia. Se dispara cuando más de 1.000 pasos están listos para ejecutarse y el número sigue creciendo, durante 15 minutos.

Los workers no dan abasto con la llegada de trabajo. Añade workers, revisa `OrconClaimLatencyHigh` y `OrconWorkerAdmissionPaused`, y busca un servicio externo lento que retenga pasos mucho tiempo. En Kubernetes puedes escalar los workers con esta métrica, por ejemplo con KEDA.

## OrconLeaseExpirations

Gravedad: advertencia. Se dispara cuando vencen más de 0,1 reservas por segundo durante 10 minutos, durante 10 minutos.

Un worker toma una reserva sobre el trabajo que ejecuta y la renueva. Las reservas vencen cuando un worker deja de renovarlas porque lo mataron, se quedó sin memoria, perdió la conexión con el almacén o se detuvo. Revisa los reinicios y las muertes por falta de memoria de los workers, la latencia del almacén y la carga de los equipos de los workers. Las reservas vencidas se recuperan solas; la alerta trata de por qué vencen.

## OrconPoisonTokens

Gravedad: advertencia. Se dispara en cuanto un paso fue suspendido como venenoso en la última hora.

Un paso abandonado tres veces (el valor por defecto) se suspende con el motivo `POISON`, y se abre una solicitud de atención con la causa `POISON_TOKEN`. Lo habitual es que un fallo del proceso lo cause los datos de una ejecución. Abre la solicitud de atención, mira la ejecución y su entrada, corrige el flujo o los datos, y luego continúa o cancela la ejecución.

## OrconUncertainOperationsOpen

Gravedad: advertencia. Se dispara cuando al menos una operación externa tiene un resultado desconocido durante 5 minutos.

Una operación pasó a `UNCERTAIN` cuando Orcon no puede saber si el sistema externo recibió el cambio. Orcon nunca vuelve a enviar una operación así sin una decisión. Lista los elementos que esperan acción con `orcon attention list`, comprueba en el sistema externo si el cambio ocurrió y resuelve el elemento con esa evidencia. La resolución registra quién decidió, la evidencia y si se aceptó un riesgo.

## OrconAttentionOverdue

Gravedad: advertencia. Se dispara cuando la solicitud de atención humana abierta más antigua tiene más de una hora, durante 5 minutos.

Una persona tiene que actuar y nadie lo hizo. Ejecuta `orcon attention list` y actúa sobre los elementos más antiguos. Si no se avisó a las personas, revisa los canales de notificación y `OrconNotificationDeliveriesDead`.

## OrconRetentionPressure

Gravedad: advertencia. Se dispara cuando los datos de ejecución guardados usan más del 95 % de su tope de tamaño, durante 30 minutos.

Los datos de ejecución se podan según la política de retención, pero los flujos con la política `ALWAYS` conservan sus datos y pueden llenar el tope. Revisa qué flujos usan `ALWAYS`, acorta la retención de los demás, o amplía el almacenamiento y sube el tope con él. Consulta la guía de retención y almacenamiento de tu versión.

## OrconDiskProtectionMode

Gravedad: crítica. Se dispara cuando la instancia entró en modo de protección de disco y siguió en él durante 1 minuto.

El volumen de datos está casi lleno. En este modo, los flujos cuya política no es `ALWAYS` no guardan detalle nuevo de ejecución más allá de lo que necesita una ejecución en curso, los flujos `ALWAYS` siguen guardando y todos los flujos siguen ejecutándose. Libera espacio o amplía el volumen. El modo termina cuando el espacio libre supera el umbral por 2 puntos porcentuales o 1 GiB.

## OrconDataVolumeLow

Gravedad: advertencia. Se dispara cuando queda libre menos del 10 % del volumen de datos, durante 15 minutos.

Es espacio libre antes de que empiece el modo de protección de disco. Busca qué creció: los datos de ejecución (consulta `OrconRetentionPressure`), los respaldos locales, el directorio temporal, el registro de escritura anticipada de SQLite (consulta `OrconSQLiteWALLarge`). Amplía el volumen o reduce lo que se guarda.

## OrconBackupMissing

Gravedad: crítica. Se dispara cuando hay una programación de respaldos configurada y el último respaldo correcto tiene más de 26 horas, durante 10 minutos.

Ejecuta `orcon backup list` para ver las últimas corridas y sus errores. Si hubo corridas fallidas, consulta `OrconBackupFailed`. Si no hay corridas, comprueba que un proceso `scheduler` o `standalone` está en marcha y que `backup.schedule` está definido. Haz un respaldo ahora con `orcon backup create`. Consulta [Respaldo y restauración](backup-and-restore.md).

## OrconBackupFailed

Gravedad: advertencia. Se dispara en cuanto un respaldo falló en las últimas 24 horas.

Ejecuta `orcon backup list` y lee el error de la corrida fallida. Las causas habituales son un destino inalcanzable o credenciales del destino erróneas, poco espacio libre para la copia temporal (en SQLite el espacio libre debe ser al menos 1,1 veces el tamaño de la base de datos) y, en PostgreSQL, un conteo de filas de la instantánea que no coincide con las filas copiadas.

## OrconBackupNotVerified

Gravedad: advertencia. Se dispara cuando la última verificación tiene más de 8 días, durante 1 hora.

Ejecuta `orcon backup verify <backupId>` sobre el último respaldo y define `backup.verify.schedule` para que ocurra solo. Un respaldo que nunca se verificó puede no restaurarse.

## OrconAuditVerificationFailed

Gravedad: crítica. Se dispara en cuanto la verificación del registro de auditoría falló en la última hora.

El registro de auditoría es a prueba de manipulación, y la verificación encontró una ruptura en la cadena: un evento que falta, un evento cambiado, un sello de bloque incorrecto o una cabeza de cadena detrás de su ancla. Ejecuta `orcon audit verify` (añade `--org` para comprobar una organización) para ver qué falló. Deja la base de datos y los registros como están, y trata la alerta como un incidente de seguridad hasta conocer la causa. Una restauración que movió anclas queda registrada como una discontinuidad declarada y no se informa como fallo.

## OrconSIEMLagHigh

Gravedad: advertencia. Se dispara cuando un destino de exportación de auditoría va más de 60 segundos atrasado, durante 10 minutos.

Los eventos se entregan al menos una vez y cada destino conserva su posición, así que un destino atrasado no pierde nada, pero el SIEM ve los eventos tarde. Comprueba que el destino es alcanzable y acepta los eventos (red, credenciales, capacidad en el destino).

## OrconKeyUnwrapFailures

Gravedad: crítica. Se dispara en cuanto no se pudo desenvolver una clave en los últimos 15 minutos.

Orcon abre sus claves de trabajo con la clave maestra. Con un proveedor KMS, las claves se vuelven a desenvolver cada hora, así que desactivar la clave del KMS, quitar el permiso de la identidad del servidor o perder la conexión con el KMS se nota aquí. Comprueba que el KMS es alcanzable, que la clave está habilitada y que la identidad puede usarla. Sin KMS, comprueba que el archivo o la variable de la clave maestra es el mismo con el que se configuró la instancia.

## OrconOutboxBacklog

Gravedad: advertencia. Se dispara cuando la entrada pendiente más antigua de la bandeja de salida tiene más de 300 segundos, durante 10 minutos.

La bandeja de salida entrega los cambios confirmados a sus consumidores, como los eventos públicos y los commits de Git. Una acumulación significa que esos consumidores están detenidos. Comprueba que los procesos `scheduler` y `worker` están en marcha, la latencia del almacén y si un consumidor falla de forma continua.

## OrconNotificationDeliveriesDead

Gravedad: advertencia. Se dispara en cuanto una entrega de notificación agotó todos sus intentos y falló en la última hora.

Revisa el destino del canal: la dirección del webhook, las credenciales, el servidor de correo. Corrígelo. En los canales de webhook, vuelve a enviar la entrega con la operación de reentrega de la API (`POST /deliveries/{deliveryId}/redeliver`). Mientras las entregas fallan, las personas pueden no enterarse de las ejecuciones en espera, así que mira también `OrconAttentionOverdue`.

## OrconIngressRejecting

Gravedad: advertencia. Se dispara cuando se rechaza más de un evento entrante por segundo durante 5 minutos, durante 10 minutos.

El motivo más común es un tope de eventos pendientes, por flujo o para la instancia, que responde 429 con un `Retry-After`. Los eventos de disparador `ingress.rejected` llevan el motivo. Reduce la acumulación añadiendo workers (consulta `OrconRunnableBacklogGrowing`), o sube el tope si la carga es esperada.

## OrconScheduleLag

Gravedad: advertencia. Se dispara cuando los horarios se disparan con más de 60 segundos de retraso, durante 10 minutos.

Un horario que llega tarde dentro del margen de gracia se dispara igual. Pasado el margen, se registra como perdido y la política del flujo decide si se omite o se pone al día. Comprueba que los procesos `scheduler` están en marcha y que el almacén responde rápido.

## OrconWorkerAdmissionPaused

Gravedad: advertencia. Se dispara cuando un worker dejó de reclamar trabajo nuevo, durante 10 minutos.

Un worker deja de reclamar cuando su uso de memoria llega al 85 % de su presupuesto. Revisa el límite de memoria del worker y qué consume la memoria, como cargas grandes o muchos pasos concurrentes. Añade memoria o workers.

## OrconPostgresPoolSaturated

Gravedad: advertencia. Se dispara cuando un proceso no tiene ninguna conexión libre a PostgreSQL, durante 10 minutos.

Compara `store.postgres.maxConnections` con la carga. Las conexiones de todos los procesos, más una para notificaciones y una para candados en cada proceso, más las que usan las migraciones y los respaldos, deben caber en el `max_connections` del servidor, menos las reservadas. Sube el valor si el servidor lo permite, o añade capacidad. Si usas un agrupador de conexiones, debe estar en modo sesión.

## OrconVersionSkew

Gravedad: advertencia. Se dispara cuando procesos de más de una versión de Orcon llevan 2 horas en marcha.

Una actualización no terminó. Ejecuta `orcon store schema status` para ver cada proceso con su versión y su rol, y reemplaza los que van atrasados. Consulta [Actualización y vuelta atrás](upgrade-and-rollback.md).

## OrconInstanceOnHold

Gravedad: crítica. Se dispara en cuanto la instancia queda retenida.

La instancia se restauró desde un respaldo. No reclama trabajo ni dispara disparadores hasta que un administrador de instancia libere la retención. Comprueba que la restauración es la que querías y luego libera la retención desde la banda de la interfaz web o con `orcon instance hold release`. Consulta [Respaldo y restauración](backup-and-restore.md).

## OrconSQLiteWALLarge

Gravedad: advertencia. Se dispara cuando el registro de escritura anticipada de SQLite supera 1 GB, durante 30 minutos.

El registro no se puede compactar mientras haya abierta una transacción de lectura más antigua que su final. Un respaldo o una verificación en curso lo hacen por diseño mientras copian la base de datos. Si no hay ningún respaldo en marcha, busca otro lector de larga duración.

## OrconUpdateAvailable

Gravedad: informativa. Se dispara en cuanto existe una versión estable más nueva.

Esta alerta existe solo cuando la consulta de versiones está activada (`updateCheck.enabled`). Lee las notas de la versión y luego planifica la actualización; consulta [Actualización y vuelta atrás](upgrade-and-rollback.md).

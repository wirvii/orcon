---
title: Instalar en Kubernetes con Helm
description: Despliega los roles api, scheduler y worker en Kubernetes con el chart oficial de Helm y PostgreSQL.
sources:
  - ADR-0083
  - ADR-0002
  - ADR-0042
sourceHash: a5b439ab7d2676cc0f72ea9b41fa18310a681979e2483c1c0a432dfee4f36359
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Mascota de Orcon" width="120" align="right">
# Instalar en Kubernetes con Helm

> Esta página describe un comportamiento especificado por el ADR 0083 que aún no está implementado. El corte que entregue el chart de Helm quitará esta nota.

El chart de Helm despliega los roles `api`, `scheduler` y `worker` como tres despliegues separados desde la misma imagen. Funciona solo con PostgreSQL. Necesitas una versión de Kubernetes y una de Helm de las que figuran en la [matriz de soporte](../support/matrix.md): Helm 3.14 o posterior, o Helm 4.

## Dónde está el chart

El chart se publica con cada versión como artefacto OCI en `oci://ghcr.io/wirvii/charts/orcon`. Su versión es la misma que la de Orcon, y está firmado con cosign sin claves, igual que la imagen. Consulta [Verificar publicaciones](verifying-releases.md).

## Los secretos nunca son valores

El chart no acepta valores secretos en `values.yaml`, porque los valores quedan en el historial de la versión de Helm. Tú creas los Secrets de Kubernetes y el chart los refiere por nombre (`existingSecret`). Se montan como archivos en `/run/secrets/orcon/` y se pasan a Orcon con las variantes `*_FILE` de la configuración. Los ajustes que no son secretos se generan en un ConfigMap montado en `/etc/orcon/config.yaml`.

## La primera ejecución es sin interfaz

Una instalación con el chart necesita la primera ejecución sin interfaz, para no depender de que una persona abra un navegador. `helm install` falla cuando el esquema de valores detecta que falta alguno de estos, y el mensaje remite a `orcon key init`:

- `masterKey.existingSecret`, un Secret que contiene la clave con el nombre `master.key` (para el proveedor `file`).
- `masterKey.backupAcknowledgement`, la huella que imprime `orcon key init`.
- `setup.owner.existingSecret`, un Secret con las claves `email`, `name` y `password` de la cuenta del propietario.

Ejecuta primero `orcon key init` en un equipo de confianza, guarda el kit de recuperación en un lugar seguro y crea los Secrets a partir de la clave que escribió. Consulta [Primera ejecución](../configuration/first-run.md) y [Clave maestra y KMS](../configuration/master-key-and-kms.md).

## Instalar

Escribe un archivo de valores con al menos los valores obligatorios e instala:

```yaml
publicUrl: https://orcon.example.com
postgres:
  existingSecret: orcon-postgres
masterKey:
  existingSecret: orcon-master-key
  backupAcknowledgement: <huella>
setup:
  owner:
    existingSecret: orcon-owner
```

```sh
helm install orcon oci://ghcr.io/wirvii/charts/orcon --version <versión> --values values.yaml
```

`<versión>` es la versión de Orcon. `<huella>` es la huella que imprime `orcon key init`. Los nombres que siguen a `existingSecret` son los nombres de tus propios Secrets. El Secret de `postgres.existingSecret` contiene la cadena de conexión con la clave `dsn` por defecto (`postgres.dsnKey`).

## Qué crea el chart

- Tres despliegues, `api`, `scheduler` y `worker`, iniciados con `serve --role=<rol>`. Los valores por defecto son 2 réplicas de cada uno.
- Un Service para `api` en el puerto 8080, y un Service sin IP de clúster para el escuchador de operaciones de cada rol en el puerto 9464.
- Un Ingress o HTTPRoute opcional (apagado por defecto), una NetworkPolicy opcional (apagada por defecto) y un HorizontalPodAutoscaler opcional para `worker` por CPU (apagado por defecto).
- Un trabajo de migración que ejecuta `orcon store schema apply` antes de cada instalación y actualización. Las réplicas arrancan con la migración automática al inicio apagada.
- Sondas en el puerto de operaciones: una sonda de arranque y una de vida en `/healthz`, y una de preparación en `/readyz`.
- Presupuestos de interrupción de pods: `api` mantiene al menos 1 pod disponible solo cuando tiene 2 o más réplicas, `scheduler` admite 1 no disponible y `worker` admite el 25 por ciento. Un presupuesto nunca impide vaciar un nodo que ejecuta una sola réplica.
- Un periodo de gracia de terminación de 45 segundos, que es el plazo de apagado más 15 segundos.

El chart fija la dirección de operaciones en cada réplica y activa las métricas de Prometheus. Crear un `ServiceMonitor` y una `PrometheusRule` es posible y está apagado por defecto. Tienen el mismo contenido que el kit de monitoreo.

## Ajustes de seguridad

Los pods cumplen el perfil `restricted` de Pod Security de Kubernetes: se ejecutan con usuario y grupo sin privilegios `65532`, usan el perfil seccomp `RuntimeDefault`, descartan todas las capacidades, prohíben la escalada de privilegios y usan un sistema de archivos raíz de solo lectura, con un directorio vacío en `/tmp`. El token de la cuenta de servicio no se monta salvo que definas `serviceAccount.automountToken: true`, algo que haces solo para usar una identidad de nube con un servicio de gestión de claves o con S3.

## Escalar los workers

Define `worker.replicas` o activa `worker.autoscaling`. Escalar según el número de fichas ejecutables, por ejemplo con KEDA, es posible; la métrica es `orcon_tokens_runnable`, y el chart no incluye KEDA.

## Actualizar y volver atrás

Actualiza con `helm upgrade`. El trabajo de migración comprueba primero el piso de compatibilidad. Cuando el veredicto es `REQUIRES_FULL_STOP`, el trabajo falla con el motivo y la actualización se detiene sin tocar los despliegues. En ese caso, escala los despliegues a 0, define `migrations.fullStop: true` y actualiza otra vez.

`helm rollback` funciona cuando la versión anterior está en el piso de compatibilidad efectivo o por encima. Si no, restaura el respaldo que hiciste antes de actualizar. Consulta [Actualización y vuelta atrás](../operations/upgrade-and-rollback.md).

## Probar la instalación

`helm test` ejecuta `orcon health --ready` contra el Service `api`.

---
title: Migrar desde n8n, resumen
description: Qué pasa de n8n a Orcon, qué cambia y qué debes revisar antes de activar los flujos importados.
sources:
  - ADR-0007
  - ADR-0018
  - ADR-0022
  - ADR-0023
  - ADR-0035
  - ADR-0037
  - ADR-0051
  - ADR-0071
sourceHash: ca8da7fe8c937eb6c87ee6cb91d7b0c30a4859027b8c4b6f35471a640ca2beba
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Migrar desde n8n, resumen

n8n is a trademark of its owner; Orcon is an independent project. Esta guía nombra a n8n solo para decir qué es.

> Esta guía describe el comportamiento especificado por las decisiones aceptadas del proyecto. Una función está disponible solo en las versiones que la incluyen.

Orcon busca una interacción familiar y la importación de flujos de n8n. No promete ser un reemplazo directo. Todo lo que difiere se te muestra antes de activar algo, y nada se sustituye en silencio.

## Qué pasa a Orcon

- **Flujos.** El importador lee las exportaciones de flujos de n8n y clasifica cada parte como compatible exactamente, compatible con una adaptación, o no compatible. Un flujo es totalmente importable cuando cada nodo y operación está mapeado, cada expresión es aceptada, cada disparador está mapeado y los lenguajes y bibliotecas del nodo Código que usa están disponibles.
- **Expresiones.** Orcon usa expresiones `{{ }}` al estilo de n8n escritas en JavaScript, con los mismos nombres `$`. Las expresiones importadas se copian sin cambios, y el importador informa las que no puede aceptar. Las expresiones se ejecutan en un entorno aislado sin acceso a la red, a archivos ni a credenciales, y con límites de tiempo y de memoria.
- **Rutas de webhook.** Los disparadores de webhook importados conservan sus rutas exactas y su comportamiento de respuesta, de modo que mover tu tráfico consiste en apuntar un dominio a Orcon. Los flujos nuevos usan direcciones de Orcon.
- **Nodos núcleo.** Cada tipo de nodo de lógica y utilidad de n8n está mapeado a un nodo de Orcon. Una opción sin equivalente, como el modo de consulta SQL del nodo Merge, se informa.
- **Integraciones.** Orcon elige los conectores a partir del uso medido en la biblioteca pública de plantillas de n8n. La meta de cada versión es que al menos el 90 % de esas plantillas se importe por completo, contando todas las plantillas por igual. El informe de cobertura publicado dice qué tan cerca está una versión. Para lo que no tiene conector, el nodo HTTP genérico es la alternativa.
- **Nodos de IA.** Cada nodo de IA de n8n se mapea a un nodo de Orcon, a una opción de un nodo de Orcon, a un tipo de conector, o se informa como no compatible con una sugerencia.

## Qué cambia

- **Los flujos importados quedan inactivos.** Los revisas primero. Una llamada de escritura que Orcon no puede probar que sea segura de repetir, como una petición HTTP que cambia datos, se importa marcada como insegura, y el flujo no se puede activar hasta que revises cada nodo marcado.
- **Los secretos no se importan.** Las credenciales llegan como marcadores con su nombre y tipo pero sin valor secreto. Escribes los secretos una sola vez en Orcon. Los usuarios se importan como invitaciones; las contraseñas y los segundos factores nunca se importan.
- **Algunas cosas se informan, no se convierten.** Una construcción que el importador no puede mapear aparece en el informe de importación con el nodo y el campo. Un ejemplo es el nodo LangChain Code, que ejecuta código arbitrario de ese marco.
- **Esperar un resultado tiene un límite de tiempo.** Un webhook que espera el resultado de un flujo responde a los 300 segundos en una dirección que imita a n8n, y a los 30 segundos en una dirección nativa, con el id de la ejecución, y la ejecución continúa. n8n no tiene ese límite; el informe de importación lista esta diferencia.
- **El tiempo se registra una vez por paso.** `$now`, `$today` y `Date.now()` devuelven el tiempo registrado cuando empezó el paso, así que no difieren entre los ítems de un mismo paso. En n8n difieren por microsegundos.
- **Los nodos de comunidad no se ejecutan tal cual.** Orcon no tiene una capa de compatibilidad para los nodos de comunidad de n8n. Un conversor público convierte en conectores de Orcon los nodos de comunidad escritos en el estilo declarativo, e informa los que necesitan código.
- **Los comandos y archivos del anfitrión están apagados por defecto.** Los nodos que ejecutan comandos o leen y escriben archivos en el anfitrión existen, y un administrador de la instancia debe habilitarlos con una lista de permitidos explícita.
- **Los agentes de IA importados redactan datos por defecto.** Un agente importado que tiene un punto de entrada público también pide la aprobación de Orcon para sus herramientas sensibles. El informe de importación lista estas diferencias.
- **Aún sin cobertura.** El nodo Data Table de n8n no tiene equivalente; el importador lo informa como no compatible.

## Cómo suele ser una migración

1. Importa tus flujos y lee el informe de importación.
2. Escribe los secretos de los marcadores de credenciales y envía las invitaciones de los usuarios.
3. Revisa cada flujo, en particular los nodos marcados como inseguros y cada diferencia informada.
4. Prueba los flujos con la ejecución de prueba del editor.
5. Activa los flujos y, cuando estés listo, apunta tu dominio a Orcon.

[Importar flujos de n8n](importing-flows.md) profundiza en la importación, y el [informe de importación de n8n](../../../reference/n8n-import-report.md) (en inglés) explica cada código del informe.

---
title: Importar flujos de n8n
description: Importa archivos de flujos de n8n a Orcon desde la web, la API o la línea de comandos, y lee el informe de importación.
sources:
  - ADR-0007
  - ADR-0037
  - ADR-0051
  - ADR-0084
sourceHash: a4c11970676ad96645774e28818ef48d0a6a2b2306ab09fdfb5f205d407461a6
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Importar flujos de n8n

n8n is a trademark of its owner; Orcon is an independent project. Esta guía nombra a n8n solo para decir qué es.

> Esta guía describe el comportamiento especificado para la importación de flujos de n8n. Una función está disponible solo en las versiones que la incluyen. La referencia de cada código del informe está en [informe de importación de n8n](../../../reference/n8n-import-report.md) (en inglés).

Exportas tus flujos de n8n como archivos JSON y se los das a Orcon. Orcon convierte cada flujo, lo crea inactivo y te entrega un informe que dice qué pasó, qué cambió y qué debes hacer antes de poder publicar o activar el flujo. Nada se sustituye en silencio. Importar nunca ejecuta nada y nunca lee un secreto.

## Qué puedes importar

Cada archivo puede contener:

- un flujo, tal como lo exporta n8n;
- un flujo dentro de un miembro `workflow`, o la doble envoltura de la biblioteca de plantillas de n8n;
- una lista de flujos, como la que produce exportar todos los flujos;
- una página de la API pública de n8n, `{ "data": [ ... ] }`;
- el fragmento que n8n pone en el portapapeles cuando copias nodos en su editor.

Un elemento que no es un flujo no detiene la importación: se omite, y el informe dice su posición y el motivo.

## Importar desde la web

1. Abre un proyecto y elige **Importar de n8n** en el selector que crea un flujo (o pulsa `I`).
2. Pega el JSON, o elige uno o más archivos `.json`, o suéltalos en la página. Puedes dar hasta 20 archivos y 8 MiB en total.
3. Orcon muestra un informe de cada flujo antes de crear nada: cuántos nodos están listos, ajustados, por revisar y sin equivalente, cuántos nodos nunca envían dos veces, las diferencias de comportamiento y lo que bloquea publicar o activar.
4. Para cada credencial que usa el flujo, elige **Crear pendiente** (por defecto) o una credencial existente del mismo tipo. Si no puedes leer credenciales, solo ves que se creará una pendiente.
5. Elige **Importar y abrir** para un flujo, o **Importar N flujos** para varios. Con un flujo se abre el editor en el flujo nuevo; con varios obtienes una lista con un enlace a cada flujo y el motivo de cada uno omitido.

Un flujo importado muestra una franja arriba del editor, "Importado de n8n", con el número de nodos por revisar, los nodos sin equivalente y las credenciales sin secreto, y un enlace al informe guardado. Las cifras salen del borrador actual, así que bajan a medida que corriges.

Cuando pegas nodos de n8n en el lienzo de un flujo abierto, Orcon los convierte y renombra todo nodo cuyo nombre o id ya se use en el flujo. Las expresiones que se refieren a un nodo renombrado se reescriben con el nombre nuevo.

## Importar con la API

`POST /api/v1/projects/{projectId}/imports` importa archivos enviados en el cuerpo de la petición. Exige la cabecera `Idempotency-Key` y el permiso de importar flujos en el proyecto.

- `sources` tiene de 1 a 20 elementos, cada uno con un `name` opcional y el `content` JSON.
- `target` es `{ "kind": "NEW_FLOWS" }`, o el borrador de un flujo existente con `{ "kind": "EXISTING_FLOW", "flowId", "baseDraftRevision", "mode" }`. `mode` es `APPEND`, que añade los nodos a la derecha de los existentes, o `REPLACE`, que reemplaza el documento y conserva el id y el nombre del flujo. Un flujo existente acepta un solo flujo de n8n. Si el borrador cambió desde `baseDraftRevision`, la petición falla con `412` e indica la revisión actual.
- `credentialChoices` asocia, de forma opcional, una `key` de credencial del informe con una credencial existente del proyecto o con una pendiente.

La respuesta es `201` con el recurso `Import`: totales y, por cada flujo, su estado, su flujo y su informe. Todo se guarda junto o no se guarda nada: si la petición falla, no queda ningún flujo, borrador ni credencial, y repetirla con la misma clave crea los flujos una sola vez. `GET /api/v1/imports/{importId}` vuelve a leer la importación; los mensajes del informe siguen la cabecera `Accept-Language` (inglés o español).

`POST /api/v1/projects/{projectId}/n8n-converter/evaluate` convierte un flujo sin guardar nada y devuelve el fragmento y el informe. Acepta `reservedNodeNames` y `reservedNodeIds` para que los nodos nuevos no choquen con los que ya tienes.

## Convertir en la línea de comandos

`orcon n8n convert <archivo...>` convierte sin conexión de red, con el mismo conversor y el mismo informe que el servidor:

```
orcon n8n convert export.json --out-dir flows --report report.md --report-json report.json
```

Escribe un archivo `orcon/flow@1` por cada flujo en `--out-dir`, un informe legible con `--report` y el informe JSON `orcon/n8n-import-report@1` con `--report-json`. Usa `--id` para elegir el id del flujo. El código de salida es 0 cuando la conversión terminó, aunque algunos nodos hayan quedado como marcadores, 2 por un uso incorrecto del comando y 5 por una entrada no válida. Los comandos que envían archivos a un servidor (`orcon n8n import` y `orcon imports get`) llegan con el cliente de red de la línea de comandos.

## Qué te dice el informe

Cada nodo termina en uno de cuatro resultados:

- **Convertido**: corre igual.
- **Ajustado**: corre, y el informe indica una diferencia.
- **Por revisar**: una persona tiene que mirarlo. Está marcado como inseguro (por ejemplo una petición HTTP que cambia datos), usa una credencial sin secreto, o alguna de sus expresiones tiene hallazgos.
- **Sin equivalente**: Orcon no tiene un nodo para él. Mira más abajo.

El informe también lista diferencias de comportamiento de todo el flujo, por ejemplo que el flujo estaba activo en n8n y aquí queda inactivo, que las ramas corren en orden, o que no se importaron los datos fijados. El flujo conserva el nombre que tenía en n8n; si otro flujo del proyecto lo tiene, se añade un número.

Las expresiones se copian sin cambios y se revisan, nunca se ejecutan. Un hallazgo nombra el campo del nodo que contiene la expresión.

## Credenciales

Los secretos nunca se leen, guardan ni muestran. Para cada credencial que usa un nodo:

- si el proyecto ya tiene una credencial con el mismo nombre y el mismo tipo, el nodo la usa;
- si no, Orcon crea una credencial pendiente con ese nombre y tipo, sin secreto y con el estado `NEEDS_SECRET`;
- si no puedes crear credenciales, el nodo conserva el nombre y el informe dice que alguien que pueda debe crearla.

También puedes elegir, al importar, cualquier credencial existente del mismo tipo.

Un flujo que usa una credencial sin secreto no se puede publicar hasta que escribas el secreto. Una prueba que llega al nodo falla con un error que nombra la credencial. Si la credencial es de autenticación del nodo HTTP genérico, el informe también te pide fijar los anfitriones a los que se puede enviar.

## Nodos sin equivalente

Un nodo que Orcon no puede convertir pasa a ser un marcador. El marcador conserva el nodo original (sin ningún valor que parezca un secreto), sus conexiones y su posición, y siempre está desactivado.

- Puedes probar el flujo. Cuando un ítem llega a un marcador, la prueba se detiene en ese nodo con el error `UNSUPPORTED_NODE`, que nombra el nodo y su tipo de n8n, y la ejecución termina según el ajuste de error del nodo. Las ramas que no llegan a él corren con normalidad. Un marcador nunca deja pasar ítems, así que un correo o un cobro omitido nunca parece enviado.
- No puedes publicar el flujo mientras tenga un marcador.
- Si el nodo era un disparador, el flujo no tiene un disparador que lo pueda iniciar, y el informe lo dice.

## Nodos desactivados

Un nodo que estaba apagado en n8n sigue desactivado. Cuando el flujo corre, un nodo desactivado no hace nada: los ítems que entran salen sin cambios por su salida principal, y nunca envía nada al exterior. Un disparador desactivado no registra su dirección al activar el flujo.

## Límites

- 8 MiB por petición, 20 archivos y 50 flujos por importación. Una migración más grande se importa en varias peticiones.
- Como máximo 2.000 nodos y 10.000 conexiones por flujo, y un documento convertido de máximo 8 MiB. Un flujo que excede estos límites se omite y se importan los demás.
- Convertir un flujo tiene un presupuesto de 10 segundos. Como máximo corren dos conversiones a la vez en una organización; una petición que excede ese límite recibe `429` y debe repetirse después de dos segundos.
- La conversión al pegar en el editor acepta hasta 200 nodos.

## El límite del detector de secretos

Las exportaciones de n8n no contienen los secretos de las credenciales. Como segunda protección, Orcon revisa cada nodo convertido en busca de valores que parezcan un secreto, y un nodo que tenga uno pasa a ser un marcador, con el valor reemplazado por `NEEDS_SECRET`. La revisión es heurística: un token guardado en un campo con un nombre neutro podría no reconocerse. Mira tus archivos exportados antes de compartirlos y no pegues archivos que no guardarías.

El informe y la importación guardada nunca contienen un valor de parámetro, una expresión ni un secreto. El archivo original no se guarda.

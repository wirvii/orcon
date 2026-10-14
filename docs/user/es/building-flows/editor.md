---
title: El editor de flujos
description: Abre un flujo, cámbialo en el lienzo, copia y pega nodos, ordénalo y usa el teclado.
sources:
  - ADR-0007
  - ADR-0082
sourceHash: 32e4cdb461a62557ac380da5715e625f49cfcc3b89d201bca5456a8352283134
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# El editor de flujos

> Esta página describe la primera versión del editor, que trabaja con un flujo de ejemplo guardado en la pestaña de tu navegador. Todavía no lee ni guarda flujos en el servidor, no edita lo que hace un nodo (sus parámetros) y no ejecuta pruebas. Las partes que entreguen esas funciones actualizarán esta página.

El editor muestra un flujo como nodos unidos por conexiones. Cambias el flujo añadiendo nodos, moviéndolos, uniéndolos y renombrándolos. Todo lo que haces con el puntero también lo puedes hacer con el teclado.

## Abrir el editor

Elige **Editor de flujos** en la navegación. Abre un flujo de ejemplo en `/flows/01930000-0000-7000-8000-00000000ed5a`: un webhook, un nodo Si que revisa un pedido, dos solicitudes HTTP y una nota adhesiva.

La página dice **Vista previa con un flujo de ejemplo**. Tus cambios se quedan en la pestaña del navegador y se pierden al recargar.

En un teléfono el flujo se muestra solo para leer: puedes desplazarlo, acercarlo y tocar un nodo para ver su estado. Para editar hace falta una tableta o un computador.

## Cambiar el flujo

| Para | Haz esto |
|---|---|
| Añadir un nodo | Elige **Añadir nodo**, o pulsa `A`, y escoge un nodo. El nodo nuevo queda junto al nodo con foco y conectado a él. |
| Mover nodos | Arrástralos, o pulsa `Shift` con las flechas (un paso de la cuadrícula, 20 px) o `Alt` y `Shift` con las flechas (cuatro pasos). |
| Unir dos nodos | Arrastra desde un puerto de salida hasta otro nodo, o pulsa `C`. Una conexión no puede entrar a un disparador, y un ciclo solo se permite de vuelta a un nodo Bucle. |
| Renombrar un nodo | Pulsa `F2`. Las expresiones que leen el nodo por su nombre (`$('Nombre anterior')`) se actualizan y el editor te dice cuántas. |
| Desactivar o activar un nodo | Pulsa `D`. |
| Añadir una nota adhesiva | Elige **Añadir nota adhesiva**, o pulsa `Shift` y `S`. |
| Eliminar | Pulsa `Delete`. Eliminar varios nodos pregunta antes y ofrece deshacer. |
| Buscar un nodo | Pulsa `Ctrl` y `F` (`Cmd` y `F` en Mac). |
| Ordenar | Elige **Ordenar**, o pulsa `Shift`, `Alt` y `T`. Con dos o más nodos seleccionados solo se ordenan esos; si no, se ordena todo el flujo. |

Los nodos se apoyan en una cuadrícula de 20 px. Los nombres son únicos dentro de un flujo: cuando añades un nodo cuyo nombre ya existe, el editor lo numera («Solicitud HTTP 2»).

## Deshacer y rehacer

`Ctrl` y `Z` deshacen; `Ctrl`, `Shift` y `Z` (o `Ctrl` y `Y`) rehacen. El editor guarda los últimos 100 cambios. Cada acción es un paso, incluidos un pegado y un orden.

## Copiar, cortar, pegar y duplicar

`Ctrl` y `C` copian los nodos seleccionados, las conexiones entre ellos y las notas adhesivas como texto en el formato `orcon/flow-fragment@1`. Puedes pegar ese texto en un foro o en un pull request. Incluye las credenciales solo por nombre, nunca sus valores. `Ctrl` y `X` cortan, `Ctrl` y `V` pegan en el puntero y `Ctrl` y `D` duplican. Los nodos pegados reciben ids nuevos y nombres libres.

Se rechaza pegar más de 2 MiB o más de 500 nodos de una vez. Pegar un flujo de n8n se reconoce, pero todavía no se convierte: la conversión ocurre en el servidor.

## Guardado

El editor guarda el borrador solo dos segundos después de tu último cambio, y como mucho diez segundos después del primer cambio sin guardar mientras sigues trabajando. `Ctrl` y `S` guardan al instante. La barra superior muestra **Guardado**, **Guardando…**, **Cambios sin guardar** o **Sin guardar, reintentando**. Si el borrador cambió en otro lugar, no se sobrescribe nada: el editor lo dice y ofrece **Recargar el flujo**, que descarta tus cambios sin guardar después de que lo confirmes.

## Teclado

Elige **Atajos de teclado** o pulsa `?` para ver la lista completa. Los atajos de una sola tecla funcionan solo mientras el lienzo tiene el foco, nunca mientras escribes en un campo. `Ctrl+K` abre la lista de todos los comandos. **Esquema del flujo** lista los nodos, sus conexiones y las notas adhesivas como texto, que es también como un lector de pantalla lee el flujo.

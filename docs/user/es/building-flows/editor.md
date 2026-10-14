---
title: El editor de flujos
description: Abre un flujo, cámbialo en el lienzo, copia y pega nodos, ordénalo y usa el teclado.
sources:
  - ADR-0007
  - ADR-0082
sourceHash: 530a522c55fca202454b33572fe8bcd868b8d2107a5390d1fe28d3b83bc397e3
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# El editor de flujos

> Esta página describe el editor mientras trabaja con un flujo de ejemplo guardado en la pestaña de tu navegador. Todavía no lee ni guarda flujos en el servidor y no ejecuta pruebas. Las versiones que entreguen esas funciones actualizarán esta página.

El editor muestra un flujo como nodos unidos por conexiones. Cambias el flujo añadiendo nodos, moviéndolos, uniéndolos, renombrándolos y definiendo lo que hace cada uno. Todo lo que haces con el puntero también lo puedes hacer con el teclado.

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

## Definir lo que hace un nodo

Pulsa `Enter` en un nodo enfocado, o haz doble clic en él, para abrir sus detalles. En una pantalla ancha (1024 px o más) los detalles son una ventana sobre el lienzo con tres paneles: **Entrada** a la izquierda, los **Parámetros** del nodo en el centro y **Salida** a la derecha. Un disparador no tiene panel de entrada. En una tableta los parámetros se acoplan a la derecha del lienzo y los datos de entrada y de salida se muestran en un panel debajo, uno cada vez. Pulsa `Esc` o elige **Volver al lienzo** para cerrar los detalles: el foco vuelve al nodo. En un móvil los detalles no están disponibles; al tocar un nodo se muestra su estado.

El panel central tiene tres pestañas:

- **Parámetros** tiene un campo por cada cosa que el nodo necesita. Cada tipo de nodo describe sus propios campos: una Petición HTTP tiene un método, una URL, cabeceras, parámetros de la dirección y un cuerpo, y un Si tiene una condición. Un campo de texto puede ser **Fijo** o una **Expresión**: una expresión es una fórmula, escrita entre llaves dobles, que se calcula para cada elemento. Un campo obligatorio lo dice cuando lo dejas vacío. Un campo solo aparece mientras se aplica; por ejemplo, el cuerpo de una Petición HTTP aparece para los métodos que envían uno.
- **Ajustes** tiene los reintentos (cuántos intentos, cuánto esperar y cómo crece la espera), los límites de tiempo de un intento y de todo el paso, qué pasa cuando el paso falla (detener, continuar con el error como dato o enviar los elementos fallidos a una segunda salida de **error**), cuántos elementos trata el paso a la vez y si se avisa a las personas cuando un paso que espera necesita una decisión. El texto dice cuál es el valor por defecto: un paso que solo lee se reintenta tres veces; un paso que cambia algo en el exterior no se reintenta salvo que se sepa que repetirlo es seguro.
- **Documentación** muestra qué hace el tipo de nodo, qué hace cada operación, cómo funcionan sus credenciales y qué significa cada parámetro.

Los cambios se escriben en el borrador mientras tecleas y se ven enseguida en el lienzo, por ejemplo la línea bajo el nodo y la salida de error. Todo lo que escribes en un campo mientras conserva el foco es un solo paso de deshacer.

### Credenciales

Un nodo que habla con otro servicio tiene un campo **Credencial** que lista las credenciales de un tipo compatible que puedes usar, con su tipo y cuándo cambiaron. Los valores secretos nunca se muestran. **Crear credencial nueva** abre un formulario con los campos del tipo de credencial sin salir del nodo; un campo secreto es de solo escritura, y la credencial nueva se elige para el nodo al crearla. Si el nodo nombra una credencial que el proyecto no tiene, los detalles dicen **Falta la credencial** y ofrecen **Crear** y **Elegir otra**; la publicación falla hasta que el nombre exista. Probar una credencial necesita el servidor, que la vista previa no usa.

### Llamar a otro flujo

Un nodo **Llamar a un subflujo** ejecuta otro flujo del proyecto. Elige el flujo de la lista, o indícalo con una fórmula y lista los flujos entre los que la fórmula puede elegir. Elige la versión (la publicada actual o un número fijo), si esperar el resultado o seguir adelante, y si llamar una vez por cada elemento. **Abrir subflujo** va al flujo llamado; `Ctrl` o `Cmd` y clic lo abre en una pestaña nueva. Se avisa cuando un flujo se llama a sí mismo y cuando el flujo no tiene versión publicada, porque una ejecución de prueba llama a la publicada.

### Versión nueva del conector

Cuando hay instalada una versión más nueva del conector de un nodo, los detalles dicen **Hay una versión nueva**, con la versión que usa el nodo y la más reciente. **Qué ha cambiado** lista los cambios entre ambas y marca los que pueden alterar lo que hace el paso, para que los revises después. **Actualizar este nodo** cambia solo el borrador, como un paso de deshacer; publicar sigue siendo un paso aparte.

Las notas de un paso y las marcas de valores sensibles todavía no están disponibles.

## Deshacer y rehacer

`Ctrl` y `Z` deshacen; `Ctrl`, `Shift` y `Z` (o `Ctrl` y `Y`) rehacen. El editor guarda los últimos 100 cambios. Cada acción es un paso, incluidos un pegado y un orden.

## Copiar, cortar, pegar y duplicar

`Ctrl` y `C` copian los nodos seleccionados, las conexiones entre ellos y las notas adhesivas como texto en el formato `orcon/flow-fragment@1`. Puedes pegar ese texto en un foro o en un pull request. Incluye las credenciales solo por nombre, nunca sus valores. `Ctrl` y `X` cortan, `Ctrl` y `V` pegan en el puntero y `Ctrl` y `D` duplican. Los nodos pegados reciben ids nuevos y nombres libres.

Se rechaza pegar más de 2 MiB o más de 500 nodos de una vez. Pegar un flujo de n8n se reconoce, pero todavía no se convierte: la conversión ocurre en el servidor.

## Guardado

El editor guarda el borrador solo dos segundos después de tu último cambio, y como mucho diez segundos después del primer cambio sin guardar mientras sigues trabajando. `Ctrl` y `S` guardan al instante. La barra superior muestra **Guardado**, **Guardando…**, **Cambios sin guardar** o **Sin guardar, reintentando**. Si el borrador cambió en otro lugar, no se sobrescribe nada: el editor lo dice y ofrece **Recargar el flujo**, que descarta tus cambios sin guardar después de que lo confirmes.

## Teclado

Elige **Atajos de teclado** o pulsa `?` para ver la lista completa. Los atajos de una sola tecla funcionan solo mientras el lienzo tiene el foco, nunca mientras escribes en un campo. `Ctrl+K` abre la lista de todos los comandos. **Esquema del flujo** lista los nodos, sus conexiones y las notas adhesivas como texto, que es también como un lector de pantalla lee el flujo.

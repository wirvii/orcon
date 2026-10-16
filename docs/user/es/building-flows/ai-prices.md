---
title: Precios de modelos de IA y límites de gasto
description: De dónde sale el precio de un modelo, cómo fijar el tuyo y cómo usan ese precio los límites de gasto y de tiempo de un agente de IA.
sources:
  - ADR-0062
  - ADR-0060
sourceHash: 01cd4c128cad8dca14ae5eb69bd68246fa9f513669147ae96bc5af473d41b0e5
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Precios de modelos de IA y límites de gasto

> Esta página describe un comportamiento especificado que todavía no está implementado. Las versiones que entreguen la tabla de precios, la pantalla de precios y los comandos quitarán esta nota.

Orcon necesita el precio de un modelo para mostrar cuánto cuesta un agente de IA y para detenerlo cuando llega a un límite de gasto. Esta página explica de dónde salen esos precios, cómo fijar los tuyos y qué hacen los límites.

## Los precios son estimaciones
Orcon multiplica los tokens que usó una llamada al modelo por el precio que tiene para ese modelo. El resultado es una estimación. La factura del proveedor del modelo es siempre la que vale. Los costos se muestran en dólares estadounidenses.

## De dónde sale un precio
Para cada modelo, Orcon usa el primer precio que encuentra en este orden:

1. Un precio que tu organización fijó para ese modelo.
2. Un precio que la persona que administra esta instalación de Orcon fijó para todas las organizaciones.
3. Los precios que vienen con Orcon.

Un precio que fijas tú o el operador reemplaza la entrada completa del modelo: los precios de entrada, salida y tokens en caché salen todos del mismo lugar, nunca mezclados.

Los precios que vienen con Orcon los leyó una persona en la página pública de precios del proveedor del modelo y anotó la dirección de la página y la fecha. La pantalla de precios muestra ambas para cada modelo, así que puedes abrir la página y comprobarlo. Orcon te avisa cuando el más antiguo de esos precios se confirmó hace más de 120 días. Los nombres de los modelos deben coincidir exactamente: un nombre de modelo con fecha y el mismo nombre sin fecha son entradas distintas.

Un modelo que no tiene precio en ninguno de estos lugares tiene un precio desconocido.

## Fijar tu propio precio
Necesitas el permiso para gestionar precios de IA en tu organización. Quien no lo tiene puede ver los precios, pero no cambiarlos.

1. Abre **Ajustes** y luego **Precios de IA** en tu organización.
2. Elige **Fijar precio** en el modelo, o añade el modelo si no está en la lista.
3. Escribe el precio en dólares por un millón de tokens, con hasta seis decimales, para entrada, salida, lecturas de caché y escrituras de caché. Los tokens de razonamiento y un precio para entradas muy largas son opcionales.
4. Escribe una nota que diga el motivo, por ejemplo que el precio fue negociado. La nota es obligatoria.
5. Guarda.

La pantalla dice quién puede cambiar los precios y muestra el origen de cada precio como texto: "Predeterminado, confirmado en una fecha", "Instancia" u "Organización" seguido de tu nota. Si otra persona cambió el mismo precio mientras tenías el formulario abierto, Orcon te lo dice y te pide recargar antes de guardar de nuevo. Para volver al precio predeterminado, elige **Quitar**.

Lo mismo se puede hacer desde la línea de órdenes, con el precio en dólares por millón de tokens:

```
orcon ai prices show
orcon ai prices override set <provider> <model> --input <dollars> --output <dollars> --cache-read <dollars> --cache-write <dollars> --note "<reason>"
orcon ai prices override remove <provider> <model>
```

`<provider>` es el nombre del proveedor del modelo tal como lo imprime `orcon ai prices show`, `<model>` es el nombre del modelo exactamente como lo usa el nodo y `<dollars>` es un número con hasta seis decimales. Añade `--instance` para fijar o quitar un precio de toda la instalación; eso necesita el permiso de instancia.

Un cambio puede tardar hasta 30 segundos en aplicarse en todas las partes de una instalación grande. Cada llamada al modelo registra qué versión del precio usó.

La persona que administra la instalación puede desactivar los precios de organización. Cuando lo hace, la pantalla lo dice, tus precios existentes se conservan pero no se usan, y no puedes añadir nuevos. El operador también puede dar a la instalación un archivo de precios; mientras se use un archivo, los precios de toda la instalación solo se cambian editando ese archivo.

## Límite de gasto de un agente de IA
El nodo agente de IA tiene un límite de gasto, `maxSpend`, en micro-dólares (una millonésima de dólar).

- Antes de cada llamada al modelo, Orcon suma el costo más alto que podría tener esa llamada a lo que el agente ya gastó. Si el total supera el límite, la llamada no se hace. El costo más alto supone que el modelo escribe todo lo que se le permite escribir, así que la última llamada puede rechazarse aunque en realidad habría costado menos.
- Si el agente tiene un límite de gasto y su modelo tiene un precio desconocido, Orcon no puede contar el costo. Al publicar el flujo recibes un error, con el código `MODEL_PRICE`, y el mensaje dice qué modelo es y dónde añadir un precio. Si el precio desaparece después de publicar (por ejemplo, alguien quita una sobreescritura), el agente se detiene antes de llamar al modelo, con el error `AI_PRICE_UNKNOWN`. Puedes enrutar ese error como cualquier otro.
- Un nodo de IA sin límite de gasto y con un precio desconocido se publica con un aviso. Se ejecuta, su costo se registra como desconocido y la llamada se cuenta como sin precio.

## Límite de tiempo de un agente de IA
El nodo agente de IA también tiene un límite de tiempo activo, `maxActiveTime`, escrito como una duración ISO 8601 como `PT10M`. El valor por defecto es 10 minutos y el valor debe ser mayor que cero.

- El tiempo activo es el tiempo en que el agente trabaja. El tiempo de espera, por ejemplo hasta que una persona apruebe una herramienta, no cuenta.
- Orcon revisa el límite antes de cada llamada al modelo y antes de ejecutar herramientas. Si se alcanza el límite, el agente se detiene con el error `AI_AGENT_LIMIT_EXCEEDED`.
- Si el límite se alcanza mientras el modelo responde, Orcon corta la llamada. Esa llamada se registra como posiblemente cobrada, porque el proveedor puede cobrarla aunque no haya llegado ninguna respuesta. Una herramienta que ya está en marcha no se corta, porque puede haber cambiado algo fuera de Orcon; el límite se aplica cuando termina.
- Si Orcon se detiene de forma inesperada, el tiempo desde el último paso guardado no se cuenta.

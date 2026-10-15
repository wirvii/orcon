<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Decisiones de diseño: Orcon frente a n8n

Maquetas: `index.html` (en esta carpeta). Ábrelas en un navegador; la barra de arriba cambia de pantalla (también con las teclas 1 a 0), de tema (oscuro o claro), de dispositivo (escritorio o teléfono) y muestra u oculta las notas.

Cada punto es una decisión para aprobar o rechazar por separado. **Igual que n8n** quiere decir que copiamos el modelo mental y la disposición, para que alguien que viene de n8n se sienta en casa (nunca su código, sus íconos ni su marca). **Distinto de n8n** quiere decir que cambiamos algo a propósito, y se explica por qué.

Base de la comparación: la interfaz pública de n8n 2.x (guardado automático, botón Publicar, pantalla de inicio con pestañas Flujos / Credenciales / Ejecuciones) y las quejas más repetidas de sus usuarios (depurar ejecuciones fallidas, mensajes de error poco claros, conectar credenciales, flujos grandes difíciles de manejar, curva de aprendizaje de las expresiones).

---

## A. Estructura general

**1. Barra lateral izquierda con el mismo orden que n8n.** *Igual que n8n.*
Inicio, Personal, Proyectos, Plantillas, Credenciales, Ejecuciones, y abajo Ayuda, Ajustes y el usuario. Quien viene de n8n encuentra todo donde lo espera.
Pantallas 2 y 3.

**2. Selector de organización arriba de la barra lateral, siempre visible.** *Distinto de n8n.*
n8n no permite varias organizaciones en una misma cuenta. Orcon sí (Wirvii, Quantium, Guía Migratoria), así que el nombre de la organización debe verse siempre para no trabajar en la equivocada.
Pantalla 3.

**3. Botón "Crear flujo" verde fijo en la barra lateral, además del de cada página, con la tecla N.** *Igual que n8n, con un atajo añadido.*
Pantallas 2, 3 y 4.

**4. Menú de usuario abajo con nombre real, iniciales, rol y organización.** *Igual que n8n.*
Corrige lo que hoy se ve: "Owner" en inglés, una "O" como avatar y solo "Cerrar sesión".
Pantalla 3.

**5. La barra lateral se contrae a íconos con la tecla [ y queda contraída dentro del editor.** *Igual que n8n.*
Pantallas 3 y 6.

**6. Ajustes usa la misma barra lateral que el resto.** *Distinto de lo que hay hoy.*
Hoy Ajustes cambia de estructura y no deja volver. Ahora es una página más, con un menú interno: Tu cuenta (Perfil, Contraseña y sesiones, Apariencia) y Organización (General, Personas y roles, Claves de API).
Pantalla 10.

**7. En el teléfono, una barra inferior con 5 destinos que llevan nombre.** *Distinto de lo que hay hoy (n8n no está pensado para teléfono).*
Inicio, Flujos, Ejecuciones, Credenciales y Ajustes, cada uno con texto. Arriba siempre se lee la organización y la sección. Hoy hay un menú más 3 íconos sin texto, uno de ellos la referencia técnica de la API.
Activa el modo teléfono en las pantallas 1, 2, 4, 6, 8, 9 y 10.

**8. Ningún enlace subrayado fuera de textos largos.** *Distinto de lo que hay hoy.*
Los nombres de flujo se abren con un clic sobre la fila. Los estados llevan color, ícono y palabra, nunca solo color.

## B. Inicio de sesión

**9. Pantalla partida: a la izquierda la marca y por qué Orcon, a la derecha el formulario.** *Distinto de n8n.*
n8n muestra solo un formulario centrado. Aprovechamos la primera pantalla para decir qué gana quien viene de n8n (importar sus flujos, nada se envía dos veces, código abierto). En el teléfono solo queda el formulario.
Pantalla 1.

**10. Mascota de brazos cruzados en el panel de marca, sin la caja gris.** *Distinto de lo que hay hoy.*
Hoy la mascota va dentro de un recuadro gris encima del logo y del lema en inglés. El lema pasa a español: "Construye, orquesta y ejecuta a tu manera".
Pantalla 1.

**11. "¿La olvidaste?" junto a la etiqueta de contraseña, ojo dentro del campo y botón a todo el ancho.** *Igual que n8n.*
Incluye el estado de error encima de los campos, sin empujar el botón fuera de la pantalla, y el flujo para recuperar la contraseña.
Pantalla 1, estados Error y Recuperar contraseña.

**12. Botón "Continuar con SSO de tu empresa" visible para todos.** *Distinto de n8n.*
SSO es entrar con la cuenta de la empresa (Google Workspace, Microsoft, etc.). En n8n solo está en el plan Enterprise; en Orcon sería parte del producto abierto. Por ahora es un botón de muestra.
Pantalla 1.

**13. Idioma y tema como dos controles discretos arriba a la derecha.** *Distinto de lo que hay hoy.*
Reemplaza el enlace subrayado "Idioma y apariencia" debajo del formulario.
Pantalla 1.

## C. Inicio y proyectos

**14. Página de inicio con pestañas Flujos / Credenciales / Ejecuciones de todos los proyectos.** *Igual que n8n.*
Pantalla 2.

**15. Cinco números arriba: flujos activos, ejecuciones de 24 h con gráfico por hora, fallidas, "para revisar" y tiempo ahorrado.** *Distinto de n8n (n8n muestra un resumen parecido, sin gráfico y sin "para revisar").*
Pantalla 2.

**16. Estado nuevo "Para revisar", en violeta.** *Distinto de n8n.*
Se usa cuando Orcon no sabe si algo salió de verdad (por ejemplo, la API de la transportadora no respondió). n8n lo marcaría como éxito o como error. Aparece en el inicio, en la barra lateral junto a Ejecuciones y en la lista de ejecuciones.
Pantallas 2 y 8.

**17. Flujos en filas compactas, no en tarjetas.** *Distinto de n8n.*
Cada fila muestra nombre, etiquetas, proyecto, resultado de la última ejecución, ejecuciones de 24 h, última modificación, dueño y el interruptor Activo. n8n usa tarjetas y no muestra la última ejecución. Con muchos flujos, la fila se lee más rápido.
Pantallas 2 y 4.

**18. Interruptor "Activo" en cada fila y en el editor.** *Igual que n8n 1.x.*
Ver también la decisión 24.
Pantallas 2, 4 y 6.

**19. Proyecto con pestañas Flujos | Credenciales | Ejecuciones | Ajustes y las personas del proyecto arriba.** *Igual que n8n.*
Pantalla 4.

**20. Proyecto vacío con mascota pensativa y tres salidas: crear, importar de n8n, ver plantillas.** *Distinto de n8n.*
La mascota aparece solo en estados vacíos, en una importación exitosa y en el inicio de sesión. Nunca sobre el lienzo, en errores ni en tablas.
Pantalla 4, estado Proyecto vacío.

## D. Crear un flujo

**21. "Crear flujo" abre una ventana con cuatro formas de empezar: en blanco, plantilla, importar de n8n y describirlo para que la IA lo arme.** *Distinto de n8n.*
n8n abre un lienzo vacío al instante. Aquí es un clic más, a cambio de que importar y usar plantillas no estén escondidos. Cada opción tiene su tecla (B, T, I). Reemplaza el formulario actual de un solo campo.
Pantalla 5.

**22. Al importar de n8n, se muestra un informe antes de crear nada.** *Distinto de n8n.*
Dice cuántos pasos quedan igual, cuántos se ajustaron solos, cuáles hay que revisar y qué credenciales faltan. Es la puerta de entrada de quien migra, así que debe dar confianza.
Pantalla 5, estado Importar de n8n.

**23. La IA arma un borrador que se revisa en el editor y nunca se activa solo.** *Distinto de n8n.*
Muestra los pasos propuestos y las credenciales necesarias antes de crear.
Pantalla 5, estado Describir con IA.

## E. Editor

**24. Activo y Publicar se ven juntos en la barra superior.** *Distinto de n8n 2.x.*
n8n 2.x quitó el interruptor y dejó solo Publicar / Despublicar. Proponemos las dos cosas porque responden preguntas distintas. "Activo" responde si el flujo está escuchando mensajes reales. "Publicar" responde qué versión es la que corre. El botón Publicar muestra cuántos cambios faltan por publicar (Mayús+P).
Si se rechaza, la alternativa es copiar exactamente a n8n 2.x, con Publicar solamente.
Pantalla 6.

**25. Guardado automático con el texto "Guardado · hace 4 s" y sin botón Guardar.** *Igual que n8n 2.x.*
Pantalla 6.

**26. Barra superior: proyecto / nombre del flujo, etiquetas, pestañas Editor · Ejecuciones · Versiones en el centro, y Compartir y Publicar a la derecha.** *Igual que n8n.*
Pantalla 6.

**27. Lienzo de izquierda a derecha con pasos cuadrados, el inicio del flujo con el lado izquierdo redondo y un rayo, "+" en cada salida y la cantidad de elementos sobre cada conexión.** *Igual que n8n.*
Usa los colores, los íconos y la marca de Orcon.
Pantalla 6.

**28. El agente de IA es un paso ancho con sus piezas colgando abajo: modelo, memoria y una o varias herramientas.** *Igual que n8n.*
Pantalla 6.

**29. Panel de pasos a la derecha, con búsqueda y categorías, que sugiere el siguiente paso según el anterior.** *Igual que n8n, más las sugerencias.*
Categorías: Disparadores, IA, WhatsApp, HTTP y webhooks, Transformar datos, Lógica del flujo, Personas. Si una búsqueda no encuentra nada, ofrece usar una llamada HTTP o pedirle el paso a la IA. Prueba escribir en el buscador.
Pantalla 6.

**30. "Probar flujo" grande abajo al centro, al lado de "Chat de prueba".** *Igual que n8n, con un añadido.*
El chat de prueba simula una conversación de WhatsApp contra el flujo.
Pantalla 6.

**31. Registros abajo que se despliegan: pasos con su duración, los pasos internos del agente (memoria, herramientas, llamadas al modelo), los tokens consumidos y el chat.** *Igual que n8n, con más detalle.*
Pantalla 6, estado Registros y chat.

**32. Etiqueta "Nunca envía dos veces" sobre los pasos que envían algo hacia afuera.** *Distinto de n8n.*
Hace visible la garantía de Orcon: si un paso se reintenta, primero confirma con el servicio externo si el envío ya salió.
Pantallas 6 y 7.

**33. En el teléfono, el editor es solo de lectura.** *Distinto de n8n.*
Se ve el flujo en pequeño, con un aviso y un resumen de la última ejecución. Para editar hace falta computador o tablet.
Pantalla 6 en modo teléfono.

## F. Detalle del paso

**34. Tres paneles: Entrada | Parámetros | Salida, con vistas Esquema, Tabla y JSON, y bordes que se pueden arrastrar.** *Igual que n8n.*
Pantalla 7.

**35. Arrastrar un campo de la entrada hasta un parámetro lo inserta.** *Igual que n8n.*
Prueba arrastrar "nombre" hasta el campo Mensaje.
Pantalla 7.

**36. Los campos insertados se ven como fichas legibles (cliente.telefono) con el resultado real debajo.** *Distinto de n8n.*
n8n muestra `{{ $json.cliente.telefono }}`. La sintaxis sigue existiendo, pero lo que se ve primero es la ficha. Las expresiones son de lo más difícil de aprender en n8n.
Pantalla 7.

**37. Botón "Pedir a la IA" junto a cada campo de texto.** *Distinto de n8n.*
Propone el contenido o la expresión, y la explica antes de usarla.
Pantalla 7.

**38. La credencial elegida muestra la cuenta real, si está conectada y cuándo se probó.** *Distinto de n8n.*
Por ejemplo "Wirvii Ventas · +57 310 555 0142 · Conectada · probada hace 2 h". n8n muestra solo el nombre.
Pantalla 7.

**39. Cuadro "Protección contra mensajes duplicados" dentro del paso, con la clave que identifica cada envío y una pestaña propia "Reintentos y seguridad".** *Distinto de n8n.*
Pantalla 7.

## G. Ejecuciones

**40. Lista a la izquierda y detalle a la derecha, con filtros por estado, fecha y flujo.** *Igual que n8n.*
Pantalla 8.

**41. Buscar ejecuciones por teléfono o por el texto del mensaje.** *Distinto de n8n.*
En n8n buscar dentro de los datos requiere marcar datos personalizados y un plan pago.
Pantalla 8.

**42. Línea de tiempo por paso, con barras de duración, en el detalle.** *Distinto de n8n.*
Muestra de un vistazo dónde se fue el tiempo y qué pasos no corrieron.
Pantalla 8.

**43. Cada error se explica en palabras simples, junto al error original y una sugerencia de arreglo que se aplica en un clic.** *Distinto de n8n.*
Ejemplo: la ventana de 24 horas de WhatsApp está cerrada, así que hay que usar una plantilla aprobada. Es la queja más repetida de los usuarios de n8n.
Pantalla 8.

**44. "Reintentar desde el fallo" repite solo los elementos que fallaron y nunca reenvía los que sí salieron.** *Igual que n8n, con una garantía añadida.*
También está "Depurar en el editor", que abre el flujo con los datos de esa ejecución, igual que n8n.
Pantalla 8.

## H. Credenciales

**45. Lista con tipo, proyecto, en cuántos flujos se usa, último uso y estado de salud (conectada, vence pronto, falló la prueba, sin probar).** *Distinto de n8n.*
n8n no avisa cuándo vence una credencial ni cuántos flujos se romperán.
Pantalla 9.

**46. Aviso arriba cuando una credencial va a vencer o falló, con el número de flujos afectados y un botón Reconectar.** *Distinto de n8n.*
Pantalla 9.

**47. Ventana de creación en tres columnas: servicio a la izquierda (con buscador), formulario al centro y una guía paso a paso a la derecha.** *Igual que n8n en la búsqueda; distinto en la guía.*
n8n envía a su documentación en otra pestaña. Aquí los pasos para conseguir cada dato están al lado, más un aviso para quien viene de n8n. Hay tres ejemplos: WhatsApp Cloud API, OpenAI y Encabezado HTTP.
Pantalla 9, estado Crear credencial.

**48. Para claves enviadas en un encabezado HTTP, se indica a qué dominios se pueden enviar.** *Distinto de n8n.*
Orcon nunca manda la clave a otro sitio, aunque un paso lo pida.
Pantalla 9, credencial Encabezado HTTP.

## I. Ajustes

**49. Las preferencias (tema, densidad, idioma) se aplican al instante, sin botón Guardar.** *Distinto de lo que hay hoy.*
Los datos de perfil y de organización sí llevan Guardar, porque afectan a otras personas.
Pantalla 10, Apariencia.

**50. Cuatro roles explicados en una línea cada uno: Propietario, Administrador, Editor y Solo lectura.** *Igual que n8n.*
En n8n los roles por proyecto son de pago.
Pantalla 10, Personas y roles.

**51. Las claves de API se muestran una sola vez al crearlas, con permisos y último uso.** *Igual que n8n.*
Las mismas claves sirven para la línea de comandos y para asistentes de IA.
Pantalla 10, Claves de API.

## J. Sistema visual

**52. Paleta oficial de Orcon, tema oscuro por defecto, verde para acciones y verde-lima a cian solo en la marca.** *Distinto de n8n (su naranja y su marca no se usan).*
Fondo azul marino; verde (#4ADE80 en oscuro, #15803D en claro) para la acción principal; el degradado #84EB72 → #22C55E → #06B6D4 solo en el logo y en el indicador de la sección activa. Tema claro completo.
Todas las pantallas.

**53. Estados con color, ícono y palabra siempre juntos.** *Igual que n8n.*
Verde = correcta, rojo = fallida, violeta = para revisar, cian = en curso, ámbar = aviso. Contraste AA y anillo de foco visible al navegar con teclado.
Todas las pantallas.

**54. Atajos de teclado visibles junto a las acciones (N, /, Esc, R, Ctrl+Enter) y una lista completa con la tecla ?.** *Igual que n8n, más visible.*
En el teléfono se ocultan.
Todas las pantallas.

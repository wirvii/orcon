// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { compareVersions } from './connector-versions';
import type { OperationSafety } from './effect-safety';
import type { FlowNode } from './flow-document';
import { START_TYPE, UNSUPPORTED_TYPE } from './node-types';

/**
 * The parameter schema of the node types (ADR-0030 §3), as the editor reads it to build the
 * configuration form of a node. It holds what the node-type view of the API (SPEC-005 A10, cut
 * ED4a) will serve: parameters with a type, `required`, `default`, `options`, `displayWhen`;
 * credential slots; the documentation of the Docs tab; and the labels of both, in the language
 * catalogs a connector package ships (`i18n/{en,es}.json`, SPEC-014 R-ED-CFG-01). Until then this
 * table is the source and the renderer (`forms/node-config`) reads nothing else, so replacing it
 * with the API response changes no screen.
 */

export type ParamType =
  | 'STRING'
  | 'NUMBER'
  | 'BOOLEAN'
  | 'OPTIONS'
  | 'MULTI_OPTIONS'
  | 'COLLECTION'
  | 'FIXED_COLLECTION'
  | 'JSON'
  | 'DATE_TIME'
  | 'RESOURCE_LOCATOR'
  | 'BINARY_PROPERTY';

type Scalar = string | number | boolean;

/** The parameter is shown only while the other parameter holds one of these values (all conditions must hold). */
export interface DisplayCondition {
  readonly param: string;
  readonly oneOf: readonly Scalar[];
}

/** The registered custom widgets this cut has (SPEC-014 R-ED-CFG-03); the others arrive with their cuts. */
export type Widget = 'flow-target';

export interface ParamDef {
  readonly name: string;
  readonly type: ParamType;
  readonly required?: boolean;
  readonly default?: unknown;
  /** The values of OPTIONS and MULTI_OPTIONS; their labels are `param.<name>.option.<value>` in the catalog. */
  readonly options?: readonly string[];
  readonly displayWhen?: readonly DisplayCondition[];
  readonly sensitive?: boolean;
  readonly multiline?: boolean;
  /** A text-like parameter starts in expression mode (the value is a formula by nature). */
  readonly expression?: 'default';
  /** The connector version that introduced the parameter; a node pinned to an older one does not show it. */
  readonly since?: string;
  /** The fields of one entry of a COLLECTION or FIXED_COLLECTION; labels are `param.<name>.field.<field>`. */
  readonly fields?: readonly ParamDef[];
  readonly widget?: Widget;
}

export interface CredentialSlot {
  readonly slot: string;
  /** The credential types that fit the slot (ADR-0057 §4 binds by name; the type must match). */
  readonly types: readonly string[];
  readonly required?: boolean;
}

export interface Catalog {
  readonly en: Readonly<Record<string, string>>;
  readonly es: Readonly<Record<string, string>>;
}

/** How a node type writes to the outside: it decides the retry default the Settings tab explains (ADR-0011 §2). */
export type EffectClass = 'pure' | 'read' | 'write' | 'by-method';

export interface NodeManifest {
  readonly key: string;
  readonly type: string;
  readonly parameters: readonly ParamDef[];
  readonly credentials: readonly CredentialSlot[];
  readonly effect: EffectClass;
  /**
   * What the operation declares about repeating it (`purity`, `safetyLevel`, the unique key). Absent for the built-in
   * types: they send nothing to a service that could receive it twice.
   */
  readonly safety?: OperationSafety;
  /** Catalog keys of the operations the Docs tab lists. */
  readonly operations: readonly string[];
  readonly catalog: Catalog;
}

export type Locale = keyof Catalog;

export const localeOf = (tag: string): Locale => (tag.toLowerCase().startsWith('es') ? 'es' : 'en');

/** A text of the manifest's catalog in the person's language; the key itself when the catalog lacks it. */
export function catalogText(manifest: Pick<NodeManifest, 'catalog'>, locale: Locale, key: string): string {
  return manifest.catalog[locale][key] ?? manifest.catalog.en[key] ?? key;
}

const pairs = (name: string): ParamDef => ({ name, type: 'FIXED_COLLECTION', fields: [{ name: 'name', type: 'STRING', required: true }, { name: 'value', type: 'STRING' }] });
const verbs = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD'] as const;
const verbOptions = (name: string) => Object.fromEntries(verbs.map((verb) => [`param.${name}.option.${verb}`, verb]));

const WEBHOOK: NodeManifest = {
  key: 'webhook',
  type: START_TYPE,
  parameters: [
    { name: 'method', type: 'OPTIONS', required: true, default: 'POST', options: [...verbs] },
    { name: 'path', type: 'STRING', required: true, default: '/webhook' },
  ],
  credentials: [],
  effect: 'pure',
  operations: ['op.receive'],
  catalog: {
    en: {
      name: 'Webhook',
      description: 'Starts the flow when another system sends a request to the address of this node.',
      'param.method.label': 'HTTP method',
      'param.method.description': 'The method the request must use.',
      ...verbOptions('method'),
      'param.path.label': 'Path',
      'param.path.description': 'The part of the address after the flow address. It starts with a slash.',
      'op.receive': 'Receive a request: the body, the query string and the headers reach the next step as the first item.',
    },
    es: {
      name: 'Webhook',
      description: 'Inicia el flujo cuando otro sistema envía una petición a la dirección de este nodo.',
      'param.method.label': 'Método HTTP',
      'param.method.description': 'El método que debe usar la petición.',
      ...verbOptions('method'),
      'param.path.label': 'Ruta',
      'param.path.description': 'La parte de la dirección que sigue a la dirección del flujo. Empieza con una barra.',
      'op.receive': 'Recibir una petición: el cuerpo, los parámetros de la dirección y las cabeceras llegan al siguiente paso como primer elemento.',
    },
  },
};

const MANUAL: NodeManifest = {
  key: 'manual',
  type: START_TYPE,
  parameters: [],
  credentials: [],
  effect: 'pure',
  operations: ['op.start'],
  catalog: {
    en: { name: 'Manual trigger', description: 'Starts the flow when a person runs it from the editor.', 'op.start': 'Start a test run by hand.' },
    es: { name: 'Disparador manual', description: 'Inicia el flujo cuando una persona lo ejecuta desde el editor.', 'op.start': 'Iniciar una ejecución de prueba a mano.' },
  },
};

const IF: NodeManifest = {
  key: 'if',
  type: 'orcon.if@1',
  parameters: [{ name: 'condition', type: 'STRING', required: true, expression: 'default' }],
  credentials: [],
  effect: 'pure',
  operations: ['op.route'],
  catalog: {
    en: {
      name: 'If',
      description: 'Sends each item down the "true" or the "false" path depending on a condition.',
      'param.condition.label': 'Condition',
      'param.condition.description': 'A formula that is true or false for each item, for example the order total is more than zero.',
      'op.route': 'Evaluate the condition for every item and send it to the "true" or the "false" output.',
    },
    es: {
      name: 'Si',
      description: 'Envía cada elemento por el camino "verdadero" o "falso" según una condición.',
      'param.condition.label': 'Condición',
      'param.condition.description': 'Una fórmula que es verdadera o falsa para cada elemento, por ejemplo que el total del pedido sea mayor que cero.',
      'op.route': 'Evaluar la condición para cada elemento y enviarlo a la salida "verdadero" o "falso".',
    },
  },
};

const HTTP: NodeManifest = {
  key: 'http',
  type: 'orcon.http@1',
  parameters: [
    { name: 'method', type: 'OPTIONS', required: true, default: 'GET', options: [...verbs] },
    { name: 'url', type: 'STRING', required: true },
    pairs('headers'),
    pairs('query'),
    { name: 'body', type: 'STRING', multiline: true, displayWhen: [{ param: 'method', oneOf: ['POST', 'PUT', 'PATCH', 'DELETE'] }] },
    { name: 'responseFormat', type: 'OPTIONS', default: 'auto', options: ['auto', 'json', 'text', 'file'], since: '1.1.0' },
    { name: 'followRedirects', type: 'BOOLEAN', default: false, since: '1.2.0' },
  ],
  credentials: [{ slot: 'auth', types: ['httpHeaderAuth'] }],
  effect: 'by-method',
  operations: ['op.request'],
  catalog: {
    en: {
      name: 'HTTP Request',
      description: 'Calls any web address and passes the response to the next step. Use it for services that have no node of their own, such as a Slack incoming webhook.',
      'param.method.label': 'Method',
      'param.method.description': 'Reading methods (GET, HEAD) can be repeated safely. The others change something on the other side.',
      ...verbOptions('method'),
      'param.url.label': 'URL',
      'param.url.description': 'The full address, starting with https://.',
      'param.headers.label': 'Headers',
      'param.headers.description': 'Extra headers to send. Put secrets in a credential, not here.',
      'param.headers.field.name': 'Header name',
      'param.headers.field.value': 'Header value',
      'param.query.label': 'Query parameters',
      'param.query.description': 'Pairs added to the address after a question mark.',
      'param.query.field.name': 'Parameter name',
      'param.query.field.value': 'Parameter value',
      'param.body.label': 'Body',
      'param.body.description': 'What to send. A formula that builds an object is sent as JSON.',
      'param.responseFormat.label': 'Response format',
      'param.responseFormat.description': 'How to read the response.',
      'param.responseFormat.option.auto': 'Detect from the response',
      'param.responseFormat.option.json': 'JSON',
      'param.responseFormat.option.text': 'Text',
      'param.responseFormat.option.file': 'File',
      'param.followRedirects.label': 'Follow redirects to other hosts',
      'param.followRedirects.description': 'Off by default: a redirect to another host is returned as the response.',
      'credential.auth.label': 'Authentication',
      'credential.auth.note': 'Optional. A header credential adds a header, for example Authorization, to every request of this node. Its value is never shown.',
      'op.request': 'Send one request per item and return the response status, headers and body.',
    },
    es: {
      name: 'Petición HTTP',
      description: 'Llama a cualquier dirección web y pasa la respuesta al siguiente paso. Úsalo para servicios que no tienen un nodo propio, como un webhook entrante de Slack.',
      'param.method.label': 'Método',
      'param.method.description': 'Los métodos de lectura (GET, HEAD) se pueden repetir sin riesgo. Los demás cambian algo al otro lado.',
      ...verbOptions('method'),
      'param.url.label': 'URL',
      'param.url.description': 'La dirección completa, empezando por https://.',
      'param.headers.label': 'Cabeceras',
      'param.headers.description': 'Cabeceras adicionales que enviar. Los secretos van en una credencial, no aquí.',
      'param.headers.field.name': 'Nombre de la cabecera',
      'param.headers.field.value': 'Valor de la cabecera',
      'param.query.label': 'Parámetros de la dirección',
      'param.query.description': 'Pares que se añaden a la dirección tras un signo de interrogación.',
      'param.query.field.name': 'Nombre del parámetro',
      'param.query.field.value': 'Valor del parámetro',
      'param.body.label': 'Cuerpo',
      'param.body.description': 'Lo que se envía. Una fórmula que construye un objeto se envía como JSON.',
      'param.responseFormat.label': 'Formato de respuesta',
      'param.responseFormat.description': 'Cómo leer la respuesta.',
      'param.responseFormat.option.auto': 'Detectarlo en la respuesta',
      'param.responseFormat.option.json': 'JSON',
      'param.responseFormat.option.text': 'Texto',
      'param.responseFormat.option.file': 'Archivo',
      'param.followRedirects.label': 'Seguir redirecciones a otros servidores',
      'param.followRedirects.description': 'Desactivado por defecto: una redirección a otro servidor se devuelve como respuesta.',
      'credential.auth.label': 'Autenticación',
      'credential.auth.note': 'Opcional. Una credencial de cabecera añade una cabecera, por ejemplo Authorization, a cada petición de este nodo. Su valor nunca se muestra.',
      'op.request': 'Enviar una petición por elemento y devolver el estado, las cabeceras y el cuerpo de la respuesta.',
    },
  },
};

const SET: NodeManifest = {
  key: 'set',
  type: 'orcon.set@1',
  parameters: [{ name: 'assignments', type: 'FIXED_COLLECTION', fields: [{ name: 'name', type: 'STRING', required: true }, { name: 'value', type: 'STRING' }] }],
  credentials: [],
  effect: 'pure',
  operations: ['op.set'],
  catalog: {
    en: {
      name: 'Set',
      description: 'Adds or changes fields of each item.',
      'param.assignments.label': 'Fields to set',
      'param.assignments.description': 'Each pair becomes a field of the item that leaves this step.',
      'param.assignments.field.name': 'Field name',
      'param.assignments.field.value': 'Value',
      'op.set': 'Write the fields into every item and pass the items on.',
    },
    es: {
      name: 'Establecer',
      description: 'Añade o cambia campos de cada elemento.',
      'param.assignments.label': 'Campos que establecer',
      'param.assignments.description': 'Cada par se convierte en un campo del elemento que sale de este paso.',
      'param.assignments.field.name': 'Nombre del campo',
      'param.assignments.field.value': 'Valor',
      'op.set': 'Escribir los campos en cada elemento y pasar los elementos.',
    },
  },
};

const plain = (key: string, type: string, en: { name: string; description: string; op: string }, es: { name: string; description: string; op: string }): NodeManifest => ({
  key,
  type,
  parameters: [],
  credentials: [],
  effect: 'pure',
  operations: ['op.main'],
  catalog: {
    en: { name: en.name, description: en.description, 'op.main': en.op },
    es: { name: es.name, description: es.description, 'op.main': es.op },
  },
});

const NOOP = plain(
  'noop',
  'orcon.noop@1',
  { name: 'No operation', description: 'Does nothing and passes the items on. Use it as a placeholder.', op: 'Pass every item on unchanged.' },
  { name: 'Sin operación', description: 'No hace nada y pasa los elementos. Úsalo como marcador.', op: 'Pasar cada elemento sin cambios.' },
);
const END = plain(
  'end',
  'orcon.end@1',
  { name: 'Done', description: 'Ends this path of the flow.', op: 'End the path. The flow finishes when no path is still running.' },
  { name: 'Fin', description: 'Termina este camino del flujo.', op: 'Terminar el camino. El flujo termina cuando ningún camino sigue en marcha.' },
);
const MERGE = plain(
  'merge',
  'orcon.merge@1',
  { name: 'Merge', description: 'Waits for the paths that lead to it and joins their items.', op: 'Join the items of the incoming paths into one list.' },
  { name: 'Unir', description: 'Espera a los caminos que llegan a él y une sus elementos.', op: 'Unir los elementos de los caminos de entrada en una lista.' },
);
const LOOP = plain(
  'loop',
  'orcon.loop@1',
  { name: 'Loop', description: 'Repeats the steps inside it until a condition is met or the limit is reached.', op: 'Run the loop body again for each pass.' },
  { name: 'Bucle', description: 'Repite los pasos que contiene hasta que se cumple una condición o se llega al límite.', op: 'Ejecutar de nuevo el cuerpo del bucle en cada vuelta.' },
);

const CALL: NodeManifest = {
  key: 'call',
  type: 'orcon.callFlow@1',
  parameters: [
    { name: 'call', type: 'RESOURCE_LOCATOR', required: true, widget: 'flow-target' },
    { name: 'input', type: 'STRING', multiline: true, expression: 'default' },
  ],
  credentials: [],
  effect: 'write',
  operations: ['op.call'],
  catalog: {
    en: {
      name: 'Call sub-flow',
      description: 'Runs another flow of the project and, in wait mode, continues with its result.',
      'param.call.label': 'Flow to call',
      'param.input.label': 'Data to send',
      'param.input.description': 'A formula for the data the called flow receives. Leave it empty to send the current item.',
      'op.call': 'Start the called flow with the data above, once per item or once for the whole list.',
    },
    es: {
      name: 'Llamar a un subflujo',
      description: 'Ejecuta otro flujo del proyecto y, en modo de espera, continúa con su resultado.',
      'param.call.label': 'Flujo que llamar',
      'param.input.label': 'Datos que enviar',
      'param.input.description': 'Una fórmula con los datos que recibe el flujo llamado. Déjala vacía para enviar el elemento actual.',
      'op.call': 'Iniciar el flujo llamado con los datos de arriba, una vez por elemento o una vez para toda la lista.',
    },
  },
};

const BY_TYPE = new Map<string, NodeManifest>([IF, HTTP, SET, NOOP, END, MERGE, LOOP, CALL].map((manifest) => [manifest.type, manifest]));

export const isUnsupported = (node: Pick<FlowNode, 'type'>): boolean => node.type === UNSUPPORTED_TYPE;

/** The manifest of a node, or undefined for a type the editor has no schema for. */
export function manifestOf(node: Pick<FlowNode, 'type' | 'element' | 'parameters'>): NodeManifest | undefined {
  if (node.type === START_TYPE) return typeof node.parameters['path'] === 'string' ? WEBHOOK : MANUAL;
  if (node.element === 'callActivity') return CALL;
  return BY_TYPE.get(node.type);
}

/** The parameters the node's pinned connector version has: a newer parameter is hidden from an older pin. */
export function parametersFor(manifest: NodeManifest, pinnedVersion: string | undefined): readonly ParamDef[] {
  return manifest.parameters.filter((param) => param.since === undefined || pinnedVersion === undefined || compareVersions(pinnedVersion, param.since) >= 0);
}

/** The retry default the Settings tab explains for a node, from its manifest and its parameters (ADR-0011 §2). */
export function effectOf(manifest: NodeManifest, parameters: Readonly<Record<string, unknown>>): 'pure' | 'read' | 'write' {
  if (manifest.effect !== 'by-method') return manifest.effect;
  const method = parameters['method'];
  if (method === undefined) return 'read';
  // A method given by a formula cannot be known here: it counts as a write, the careful reading.
  if (typeof method !== 'string' || method.startsWith('=')) return 'write';
  return method.toUpperCase() === 'GET' || method.toUpperCase() === 'HEAD' ? 'read' : 'write';
}

/**
 * What the node's operation declares about repeating it, or undefined for a step that changes nothing outside.
 * A type that declares nothing but sends by method (the HTTP request) is a send that cannot be repeated safely
 * as soon as the method writes: Orcon cannot know whether the other side already did it.
 */
export function safetyOf(node: Pick<FlowNode, 'type' | 'element' | 'parameters'>): OperationSafety | undefined {
  const manifest = manifestOf(node);
  if (!manifest) return undefined;
  if (manifest.safety) return manifest.safety;
  if (manifest.effect === 'by-method' && effectOf(manifest, node.parameters) === 'write') return { purity: 'EFFECT', safetyLevel: 'UNSAFE' };
  return undefined;
}

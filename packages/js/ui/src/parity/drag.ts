// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Point } from './env';

const center = (element: Element, offset?: Point): Point => {
  const box = element.getBoundingClientRect();
  return offset ? { x: box.left + offset.x, y: box.top + offset.y } : { x: box.left + box.width / 2, y: box.top + box.height / 2 };
};

const tick = () => new Promise<void>((resolve) => setTimeout(resolve, 8));

/**
 * Presses on `from`, moves in small steps to `to` and releases, sending the pointer and mouse events
 * a real mouse sends. A drag needs several moves for the canvas library to see it as a drag, which a
 * one-step drag from the test tool does not give. Positions are relative to the element's top left.
 */
export async function dragBetween(from: Element, to: Element, options?: { sourcePosition?: Point; targetPosition?: Point }): Promise<void> {
  const start = center(from, options?.sourcePosition);
  const end = center(to, options?.targetPosition);
  const view = from.ownerDocument.defaultView as Window;
  const fire = (target: EventTarget, type: string, point: Point, buttons: number) => {
    const init = { bubbles: true, cancelable: true, composed: true, view, clientX: point.x, clientY: point.y, screenX: point.x, screenY: point.y, button: 0, buttons, pointerId: 1, pointerType: 'mouse', isPrimary: true };
    target.dispatchEvent(type.startsWith('pointer') ? new PointerEvent(type, init) : new MouseEvent(type, init));
  };
  fire(from, 'pointerdown', start, 1);
  fire(from, 'mousedown', start, 1);
  await tick();
  const steps = 12;
  for (let index = 1; index <= steps; index += 1) {
    const point = { x: start.x + ((end.x - start.x) * index) / steps, y: start.y + ((end.y - start.y) * index) / steps };
    const under = from.ownerDocument.elementFromPoint(point.x, point.y) ?? from.ownerDocument.body;
    fire(under, 'pointermove', point, 1);
    fire(under, 'mousemove', point, 1);
    await tick();
  }
  const under = from.ownerDocument.elementFromPoint(end.x, end.y) ?? from.ownerDocument.body;
  fire(under, 'pointerup', end, 0);
  fire(under, 'mouseup', end, 0);
  await tick();
}

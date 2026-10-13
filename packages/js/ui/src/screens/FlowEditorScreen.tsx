// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Toaster } from '../components/toast/Toast';

import { EditorFrame, type EditorFrameProps } from './EditorFrame';
import { ProductShell } from './ProductShell';

export type FlowEditorScreenProps = EditorFrameProps;

/**
 * S3: the flow editor in style A. Editing is for tablet and desktop: the canvas, its tools, the
 * node palette, the node details view with Input, Parameters and Output (docked and with a bottom
 * data panel on a tablet), and the Execute workflow button. On a phone the same canvas is view only:
 * pan, zoom and tap a node for its state and counts. The editor fills the space of the page.
 */
export function FlowEditorScreen(props: FlowEditorScreenProps) {
  return (
    <ProductShell current="flows" fill>
      <div className="min-h-0 flex-1">
        <EditorFrame {...props} />
      </div>
      <Toaster />
    </ProductShell>
  );
}

// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';

import { renderInApp } from '../../../../test/screen-harness';
import { exampleFlowDocument } from '../model/example-flow';
import { EditorStoreProvider } from '../store/editor-context';
import { createEditorStore } from '../store/editor-store';

import { RunBar } from './RunBar';

function renderBar(change?: (document: ReturnType<typeof exampleFlowDocument>) => void, handlers: { onTest?: () => void; onChat?: () => void } = {}) {
  const document = exampleFlowDocument();
  change?.(document);
  return renderInApp(
    <EditorStoreProvider store={createEditorStore({ document, revision: '1' })}>
      <RunBar onTest={handlers.onTest ?? vi.fn()} onChat={handlers.onChat ?? vi.fn()} />
    </EditorStoreProvider>,
  );
}

test('RunBar shows test chat only for chat or messaging triggers', async () => {
  // The sample flow starts from a webhook: there is nobody to chat with.
  const first = renderBar();
  expect(screen.getByRole('button', { name: /^Test flow/ })).toBeVisible();
  expect(screen.queryByRole('button', { name: 'Test chat' })).toBeNull();
  first.unmount();

  // A manual start is not a chat either.
  const manual = renderBar((document) => {
    const start = document.nodes.find((node) => node.id === 'webhook');
    if (start) start.parameters = {};
  });
  expect(screen.queryByRole('button', { name: 'Test chat' })).toBeNull();
  manual.unmount();

  // A messaging connector's trigger is.
  const onChat = vi.fn();
  renderBar(
    (document) => {
      const start = document.nodes.find((node) => node.id === 'webhook');
      if (start) start.type = 'orcon.whatsapp.whatsapp@1';
    },
    { onChat },
  );
  await userEvent.click(screen.getByRole('button', { name: 'Test chat' }));
  expect(onChat).toHaveBeenCalledTimes(1);
});

test('RunBar runs the test when Test flow is pressed', async () => {
  const onTest = vi.fn();
  renderBar(undefined, { onTest });
  await userEvent.click(screen.getByRole('button', { name: /^Test flow/ }));
  expect(onTest).toHaveBeenCalledTimes(1);
});

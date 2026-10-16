// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, KbdCombination } from '@orcon/ui';
import { MessageSquare, Play } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { startsFromChat } from '../model/next-step-suggestions';
import { useEditorStore } from '../store/editor-context';

const messages = defineMessages({
  label: { id: 'editor.run.label', defaultMessage: 'Test the flow', description: 'Accessible name of the pair of buttons at the bottom of the canvas that test the flow.' },
  test: { id: 'editor.run.test', defaultMessage: 'Test flow', description: 'Big button at the bottom of the canvas that runs the flow once to see what it does.' },
  chat: { id: 'editor.run.chat', defaultMessage: 'Test chat', description: 'Button next to Test flow that opens a conversation to talk to the flow the way a customer would.' },
});

export interface RunBarProps {
  /** Runs the flow once (Ctrl+Enter). */
  onTest: () => void;
  /** Opens the test chat. Only used when the flow starts from a chat or messaging trigger. */
  onChat: () => void;
  chatOpen?: boolean | undefined;
}

/**
 * "Test flow" large at the bottom center of the canvas and, beside it, "Test chat" (SPEC-019 R-UX-EDT-05). The
 * chat shows only when the flow starts from a chat or a messaging trigger: for any other flow there is nobody
 * to talk to, so the button is not offered.
 */
export function RunBar({ onTest, onChat, chatOpen = false }: RunBarProps) {
  const intl = useIntl();
  const chat = useEditorStore((state) => startsFromChat(state.document));
  return (
    <div role="group" aria-label={intl.formatMessage(messages.label)} className="flex items-center gap-3" data-run-bar>
      <Button onClick={onTest} className="min-h-12 px-5 shadow-lg">
        <Play aria-hidden className="size-icon-16" />
        {intl.formatMessage(messages.test)}
        <KbdCombination keys={['Ctrl', 'Enter']} className="ms-1" />
      </Button>
      {chat ? (
        <Button variant="secondary" aria-pressed={chatOpen} onClick={onChat} className="min-h-12 px-5 shadow-lg">
          <MessageSquare aria-hidden className="size-icon-16" />
          {intl.formatMessage(messages.chat)}
        </Button>
      ) : null}
    </div>
  );
}

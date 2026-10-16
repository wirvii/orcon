// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, IconButton, Input } from '@orcon/ui';
import { Send } from 'lucide-react';
import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  title: { id: 'editor.chat.title', defaultMessage: 'Test chat', description: 'Title of the panel where the flow is tested by chatting with it, like a customer would.' },
  hint: {
    id: 'editor.chat.hint',
    defaultMessage: 'Write as if you were the customer. Nothing is sent to a real person.',
    description: 'Explains the test chat: the messages are a simulation, nobody real receives them.',
  },
  empty: { id: 'editor.chat.empty', defaultMessage: 'No messages yet.', description: 'Shown in the test chat before anything was written.' },
  field: { id: 'editor.chat.field', defaultMessage: 'Write as if you were the customer', description: 'Label and placeholder of the field where the test message is typed.' },
  send: { id: 'editor.chat.send', defaultMessage: 'Send the test message', description: 'Accessible name of the button that sends the test message.' },
  unavailable: {
    id: 'editor.chat.unavailable',
    defaultMessage: 'The test conversation is not available yet: it needs the test sessions of the server.',
    description: 'Says why the test chat cannot send messages yet.',
  },
  soon: { id: 'editor.chat.soon', defaultMessage: 'Coming soon', description: 'Label on a feature that is not available yet.' },
  you: { id: 'editor.chat.you', defaultMessage: 'Customer', description: 'Name of the sender of a message the person typed in the test chat.' },
  flow: { id: 'editor.chat.flow', defaultMessage: 'Flow', description: 'Name of the sender of a message the flow answered in the test chat.' },
});

export interface ChatMessage {
  id: string;
  from: 'customer' | 'flow';
  text: string;
  /** Local time of the message, already formatted. */
  time?: string | undefined;
}

export interface TestChatProps {
  messages: readonly ChatMessage[];
  /** Sends what the person wrote. Without it the test chat says it is not available yet and the field is off. */
  onSend?: ((text: string) => void) | undefined;
  /** A line under the title, for example which number the chat simulates. */
  context?: string | undefined;
}

/**
 * A chat that looks like the messaging app the flow talks to (SPEC-019 R-UX-EDT-05): what the customer writes on the
 * right, what the flow answers on the left. The messages are a simulation; the host connects it to the test sessions
 * of the server and, until it does, the field is off and says why.
 */
export function TestChat({ messages: conversation, onSend, context }: TestChatProps) {
  const intl = useIntl();
  const [draft, setDraft] = useState('');
  const available = onSend !== undefined;
  const submit = () => {
    const text = draft.trim();
    if (text === '' || !onSend) return;
    onSend(text);
    setDraft('');
  };
  return (
    <section aria-label={intl.formatMessage(messages.title)} data-test-chat className="flex min-h-0 min-w-0 flex-1 flex-col">
      <header className="border-border flex flex-wrap items-baseline gap-x-2 border-b px-4 py-2 text-sm">
        <h3 className="font-semibold">{intl.formatMessage(messages.title)}</h3>
        {context ? <span className="text-fg-muted">{context}</span> : null}
        {available ? null : <Badge tone="neutral">{intl.formatMessage(messages.soon)}</Badge>}
      </header>
      <ol className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-4 py-3" aria-live="polite">
        {conversation.length === 0 ? <li className="text-fg-muted text-sm">{intl.formatMessage(messages.empty)}</li> : null}
        {conversation.map((message) => (
          <li
            key={message.id}
            data-from={message.from}
            className={`max-w-3/4 rounded-lg px-3 py-2 text-sm ${message.from === 'customer' ? 'bg-action-primary text-action-primary-fg self-end' : 'bg-surface-sunken text-fg self-start'}`}
          >
            <span className="sr-only">{intl.formatMessage(message.from === 'customer' ? messages.you : messages.flow)}: </span>
            {message.text}
            {message.time ? <span className="ms-2 text-xs opacity-80">{message.time}</span> : null}
          </li>
        ))}
      </ol>
      <form
        className="border-border flex items-center gap-2 border-t p-3"
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
      >
        <Input
          value={draft}
          disabled={!available}
          aria-label={intl.formatMessage(messages.field)}
          placeholder={intl.formatMessage(messages.field)}
          aria-describedby="editor-chat-note"
          onChange={(event) => {
            setDraft(event.target.value);
          }}
        />
        <IconButton type="submit" variant="primary" icon={Send} label={intl.formatMessage(messages.send)} disabled={!available || draft.trim() === ''} />
      </form>
      <p id="editor-chat-note" className="text-fg-muted px-4 pb-3 text-xs">
        {intl.formatMessage(available ? messages.hint : messages.unavailable)}
      </p>
    </section>
  );
}

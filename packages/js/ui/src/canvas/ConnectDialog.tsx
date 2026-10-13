// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react';
import { useIntl } from 'react-intl';

import { Button } from '../components/button/Button';
import { Dialog, DialogClose } from '../components/dialog/Dialog';
import { FormField } from '../components/form-field/FormField';
import { Select } from '../components/select/Select';
import { commonMessages } from '../lib/messages';

import { canvasMessages } from './messages';
import { connectError, type ConnectError, type ConnectRequest, type Graph } from './model';

export interface ConnectDialogProps {
  graph: Graph;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The node the form starts with as the source, usually the one that has focus. */
  initialSource?: string;
  onConnect: (request: ConnectRequest) => void;
}

const errorMessage = {
  'same-node': canvasMessages.connectErrorSame,
  duplicate: canvasMessages.connectErrorDuplicate,
  'trigger-input': canvasMessages.connectErrorTrigger,
  'unknown-node': canvasMessages.connectErrorIncomplete,
  'unknown-port': canvasMessages.connectErrorIncomplete,
} satisfies Record<ConnectError, unknown>;

function ConnectForm({ graph, initialSource, onConnect, onDone }: Pick<ConnectDialogProps, 'graph' | 'initialSource' | 'onConnect'> & { onDone: () => void }) {
  const intl = useIntl();
  const sources = graph.nodes.filter((node) => node.outputs.length > 0);
  const targets = graph.nodes.filter((node) => node.inputs.length > 0);
  const start = sources.find((node) => node.id === initialSource) ?? sources[0];
  const [source, setSource] = useState(start?.id ?? '');
  const [sourceOutput, setSourceOutput] = useState(start && start.outputs.length === 1 ? (start.outputs[0]?.id ?? '') : '');
  const [target, setTarget] = useState('');
  const [targetInput, setTargetInput] = useState('');
  const [error, setError] = useState<string | undefined>();

  const sourceNode = graph.nodes.find((node) => node.id === source);
  const targetNode = graph.nodes.find((node) => node.id === target);
  const choose = intl.formatMessage(canvasMessages.connectChoose);

  const pickSource = (id: string) => {
    setSource(id);
    const node = graph.nodes.find((candidate) => candidate.id === id);
    setSourceOutput(node && node.outputs.length === 1 ? (node.outputs[0]?.id ?? '') : '');
    setError(undefined);
  };
  const pickTarget = (id: string) => {
    setTarget(id);
    const node = graph.nodes.find((candidate) => candidate.id === id);
    setTargetInput(node && node.inputs.length === 1 ? (node.inputs[0]?.id ?? '') : '');
    setError(undefined);
  };

  const submit = () => {
    if (!source || !sourceOutput || !target || !targetInput) {
      setError(intl.formatMessage(canvasMessages.connectErrorIncomplete));
      return;
    }
    const request = { source, sourceOutput, target, targetInput };
    const problem = connectError(graph, request);
    if (problem) {
      setError(intl.formatMessage(errorMessage[problem]));
      return;
    }
    onConnect(request);
    onDone();
  };

  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <FormField label={intl.formatMessage(canvasMessages.connectSource)}>
        <Select value={source} onValueChange={pickSource} placeholder={choose} options={sources.map((node) => ({ value: node.id, label: node.name }))} />
      </FormField>
      <FormField label={intl.formatMessage(canvasMessages.connectOutput)}>
        <Select
          value={sourceOutput}
          onValueChange={(value) => {
            setSourceOutput(value);
            setError(undefined);
          }}
          placeholder={choose}
          options={(sourceNode?.outputs ?? []).map((port) => ({ value: port.id, label: port.label ?? port.id }))}
        />
      </FormField>
      <FormField label={intl.formatMessage(canvasMessages.connectTarget)}>
        <Select value={target} onValueChange={pickTarget} placeholder={choose} options={targets.map((node) => ({ value: node.id, label: node.name }))} />
      </FormField>
      <FormField label={intl.formatMessage(canvasMessages.connectInput)} error={error}>
        <Select
          value={targetInput}
          onValueChange={(value) => {
            setTargetInput(value);
            setError(undefined);
          }}
          placeholder={choose}
          options={(targetNode?.inputs ?? []).map((port) => ({ value: port.id, label: port.label ?? port.id }))}
        />
      </FormField>
      <div className="flex flex-wrap justify-end gap-2 pt-1">
        <DialogClose asChild>
          <Button variant="secondary">{intl.formatMessage(commonMessages.cancel)}</Button>
        </DialogClose>
        <Button type="submit">{intl.formatMessage(canvasMessages.connectSubmit)}</Button>
      </div>
    </form>
  );
}

/** Connects an output of one node to an input of another from lists, with no dragging. */
export function ConnectDialog({ graph, open, onOpenChange, initialSource, onConnect }: ConnectDialogProps) {
  const intl = useIntl();
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(canvasMessages.connectTitle)}
      description={intl.formatMessage(canvasMessages.connectDescription)}
    >
      <ConnectForm graph={graph} initialSource={initialSource} onConnect={onConnect} onDone={() => onOpenChange(false)} />
    </Dialog>
  );
}

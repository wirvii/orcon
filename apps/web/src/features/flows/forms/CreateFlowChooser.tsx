// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Badge, Button, ComingSoon, cn, Dialog, FormField, Input, Kbd, ProblemAlert, Select, type ProblemDetails } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { FileUp, LayoutTemplate, Plus, Sparkles, type LucideIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { defineMessages, useIntl } from 'react-intl';
import { z } from 'zod';

import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { formatIssue } from '../../../app/forms/zod-messages';
import { shellProjectsQueryOptions } from '../../../app/shell/shell-data';
import { useCreateFlow } from '../api/mutations';
import type { CreateMode } from '../model/params';

const messages = defineMessages({
  title: { id: 'flows.create.title', defaultMessage: 'Create flow', description: 'Title of the dialog where a person chooses how to start a flow.' },
  description: {
    id: 'flows.create.description',
    defaultMessage: 'Choose how to start. A new flow is inactive: nothing runs until you publish it.',
    description: 'Explains the four ways to start a flow and that the new flow starts inactive.',
  },
  options: { id: 'flows.create.options', defaultMessage: 'How to start', description: 'Accessible name of the group of the four ways to start a flow.' },
  blank: { id: 'flows.create.blank.title', defaultMessage: 'Blank', description: 'Option that starts a flow with an empty canvas.' },
  blankText: { id: 'flows.create.blank.text', defaultMessage: 'Opens the editor with an empty canvas.', description: 'Explains the blank option.' },
  template: { id: 'flows.create.template.title', defaultMessage: 'From a template', description: 'Option that starts a flow from a ready-made template.' },
  templateText: { id: 'flows.create.template.text', defaultMessage: 'Start from a flow the team already built.', description: 'Explains the template option.' },
  import: { id: 'flows.create.import.title', defaultMessage: 'Import from n8n', description: 'Option that starts a flow from an n8n workflow.' },
  importText: { id: 'flows.create.import.text', defaultMessage: 'Bring a workflow you already have in n8n.', description: 'Explains the import option.' },
  ai: { id: 'flows.create.ai.title', defaultMessage: 'Describe it and AI builds it', description: 'Option that starts a flow from a description written in plain words.' },
  aiText: { id: 'flows.create.ai.text', defaultMessage: 'Write what you need in your own words.', description: 'Explains the AI option.' },
  soon: { id: 'flows.create.soon', defaultMessage: 'Soon', description: 'Small label on an option that is not available yet.' },
  name: { id: 'flows.create.name', defaultMessage: 'Flow name', description: 'Label of the name field of a new flow.' },
  nameHint: { id: 'flows.create.nameHint', defaultMessage: 'You can rename it later inside the editor.', description: 'Hint under the name field of a new flow.' },
  defaultName: { id: 'flows.create.defaultName', defaultMessage: 'Untitled flow', description: 'Name suggested for a new flow.' },
  templateSoonTitle: { id: 'flows.create.template.soon.title', defaultMessage: 'Templates', description: 'Title of the card that says templates are not available yet.' },
  templateSoonText: {
    id: 'flows.create.template.soon.text',
    defaultMessage: 'Ready-made flows are not available yet. For now, start from a blank canvas.',
    description: 'One sentence that says templates are missing and what to do meanwhile.',
  },
  importSoonTitle: { id: 'flows.create.import.soon.title', defaultMessage: 'Import from n8n', description: 'Title of the card that says importing from n8n is not available yet.' },
  importSoonText: {
    id: 'flows.create.import.soon.text',
    defaultMessage: 'Converting an n8n workflow and showing what changes is not available yet. For now, start from a blank canvas.',
    description: 'One sentence that says importing from n8n is missing and what to do meanwhile.',
  },
  aiSoonTitle: { id: 'flows.create.ai.soon.title', defaultMessage: 'Build with AI', description: 'Title of the card that says building a flow from a description is not available yet.' },
  aiSoonText: {
    id: 'flows.create.ai.soon.text',
    defaultMessage: 'Describing a flow and having it built for you is not available yet. For now, start from a blank canvas.',
    description: 'One sentence that says the AI builder is missing and what to do meanwhile.',
  },
  project: { id: 'flows.create.project', defaultMessage: 'Project', description: 'Label of the choice of the project where the flow is created.' },
  cancel: { id: 'flows.create.cancel', defaultMessage: 'Cancel', description: 'Button that closes the dialog without creating anything.' },
  submit: { id: 'flows.create.submit', defaultMessage: 'Create and open', description: 'Button that creates the flow and opens it in the editor.' },
});

const createFlowSchema = z.object({
  name: z.string().trim().min(1).max(120),
});

interface Choice {
  mode: CreateMode;
  key: string;
  icon: LucideIcon;
  title: (typeof messages)[keyof typeof messages];
  text: (typeof messages)[keyof typeof messages];
  /** The option has no server part yet: it is marked and its pane says so. */
  soon: boolean;
}

/** The four ways to start. Template, import and AI have no server part yet, so they are marked and say so. */
const choices: readonly Choice[] = [
  { mode: 'blank', key: 'B', icon: Plus, title: messages.blank, text: messages.blankText, soon: false },
  { mode: 'template', key: 'T', icon: LayoutTemplate, title: messages.template, text: messages.templateText, soon: true },
  { mode: 'import', key: 'I', icon: FileUp, title: messages.import, text: messages.importText, soon: true },
  { mode: 'ai', key: 'A', icon: Sparkles, title: messages.ai, text: messages.aiText, soon: true },
];

/** True when a key press is going into something a person types in. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

export interface CreateFlowChooserProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The option shown first. Defaults to Blank. */
  initialMode?: CreateMode | undefined;
  orgId: string;
  orgSlug: string;
  /** The project the flow is created in unless the person picks another one. */
  projectHandle: string;
  /**
   * The address asked for the chooser (`?create`): the editor then takes that history entry's place, so going back
   * from the editor does not open the chooser again.
   */
  replaceHistory?: boolean | undefined;
}

/**
 * The choice of how to start a flow: blank, from a template, imported from n8n, or described to AI. Only Blank
 * has a server part today; the other three show the coming-soon state and never pretend to work (SPEC-019 UX4).
 * The keys B, T, I and A choose an option. The project the flow is created in is chosen in the footer.
 */
export function CreateFlowChooser({ open, onOpenChange, initialMode = 'blank', orgId, orgSlug, projectHandle, replaceHistory = false }: CreateFlowChooserProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const projects = useQuery(shellProjectsQueryOptions(orgId));
  const [mode, setMode] = useState<CreateMode>(initialMode);
  const [chosenHandle, setChosenHandle] = useState<string | undefined>(undefined);
  const [failure, setFailure] = useState<ProblemDetails | undefined>(undefined);

  const items = projects.data ?? [];
  const target = items.find((project) => project.handle === (chosenHandle ?? projectHandle)) ?? items.find((project) => project.handle === projectHandle);
  const create = useCreateFlow(orgId, target?.id ?? '');

  // The chooser opens on the option the address asked for, every time it opens.
  useEffect(() => {
    if (open) {
      setMode(initialMode);
      setFailure(undefined);
    }
  }, [open, initialMode]);

  useEffect(() => {
    if (!open) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return;
      const choice = choices.find((candidate) => candidate.key.toLowerCase() === event.key.toLowerCase());
      if (choice === undefined) return;
      event.preventDefault();
      setMode(choice.mode);
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const form = useForm<z.infer<typeof createFlowSchema>>({
    resolver: zodResolver(createFlowSchema, { error: (issue) => formatIssue(intl, issue) }),
    defaultValues: { name: intl.formatMessage(messages.defaultName) },
    mode: 'onTouched',
  });
  const { errors } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    if (target === undefined) return;
    setFailure(undefined);
    try {
      const flow = await create.mutateAsync(values.name);
      await navigate({
        to: '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId',
        params: { orgSlug, projectHandle: target.handle, flowId: flow.id },
        replace: replaceHistory,
      });
    } catch (error) {
      setFailure(toProblemDetails(error));
    }
  });

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(messages.title)}
      description={intl.formatMessage(messages.description)}
      className="max-w-screen-md"
      footer={
        <div className="flex w-full flex-wrap items-center gap-2">
          <div className="flex min-w-0 items-center gap-2 max-md:w-full">
            <span aria-hidden className="text-fg-muted text-sm">
              {intl.formatMessage(messages.project)}
            </span>
            <Select
              aria-label={intl.formatMessage(messages.project)}
              value={target?.handle ?? ''}
              onValueChange={setChosenHandle}
              options={items.map((project) => ({ value: project.handle, label: project.name }))}
              className="min-w-panel-width max-md:w-full"
            />
          </div>
          <span className="flex-1 max-md:hidden" />
          <Button
            variant="secondary"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            {intl.formatMessage(messages.cancel)}
          </Button>
          {mode === 'blank' ? (
            <Button type="submit" form="create-flow-form" loading={create.isPending} disabled={target === undefined}>
              {intl.formatMessage(messages.submit)}
            </Button>
          ) : null}
        </div>
      }
    >
      <div role="group" aria-label={intl.formatMessage(messages.options)} className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {choices.map((choice) => {
          const Icon = choice.icon;
          const selected = choice.mode === mode;
          return (
            <button
              key={choice.mode}
              type="button"
              aria-pressed={selected}
              aria-keyshortcuts={choice.key}
              onClick={() => {
                setMode(choice.mode);
              }}
              className={cn(
                'border-strong bg-surface relative flex min-h-16 flex-col items-start gap-2 rounded-xl border p-4 text-left transition-colors',
                'hover:border-control',
                selected && 'border-canvas-selection bg-surface-hover ring-canvas-selection ring-1',
              )}
            >
              <span className="bg-surface-sunken border-border inline-flex size-10 items-center justify-center rounded-lg border">
                <Icon aria-hidden className="size-icon-20" />
              </span>
              <span className="text-base font-semibold">{intl.formatMessage(choice.title)}</span>
              <span className="text-fg-muted text-xs">{intl.formatMessage(choice.text)}</span>
              <span className="absolute top-3 right-3 flex items-center gap-1">
                {choice.soon ? <Badge tone="neutral">{intl.formatMessage(messages.soon)}</Badge> : null}
                <Kbd aria-hidden>{choice.key}</Kbd>
              </span>
            </button>
          );
        })}
      </div>
      {mode === 'blank' ? (
        <form id="create-flow-form" noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-3 pt-2">
          {failure ? <ProblemAlert problem={failure} /> : null}
          <FormField label={intl.formatMessage(messages.name)} name="/name" required error={errors.name?.message} description={intl.formatMessage(messages.nameHint)} className="max-w-screen-sm">
            <Input autoComplete="off" {...form.register('name')} />
          </FormField>
        </form>
      ) : mode === 'template' ? (
        <ComingSoon icon={LayoutTemplate} headingLevel={3} title={intl.formatMessage(messages.templateSoonTitle)} description={intl.formatMessage(messages.templateSoonText)} />
      ) : mode === 'import' ? (
        <ComingSoon icon={FileUp} headingLevel={3} title={intl.formatMessage(messages.importSoonTitle)} description={intl.formatMessage(messages.importSoonText)} />
      ) : (
        <ComingSoon icon={Sparkles} headingLevel={3} title={intl.formatMessage(messages.aiSoonTitle)} description={intl.formatMessage(messages.aiSoonText)} />
      )}
    </Dialog>
  );
}

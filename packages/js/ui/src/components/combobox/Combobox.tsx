// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'cmdk';
import { Check, ChevronDown } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useState, type AriaAttributes } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { commandItem, floatingPanel, textControl } from '../../lib/control';
import { CommandResults } from '../../lib/command-list';
import { useFieldControlProps } from '../form-field/field-context';

const messages = defineMessages({
  empty: { id: 'combobox.empty', defaultMessage: 'No matches', description: 'Shown in a searchable list when nothing matches what was typed.' },
});

export interface ComboboxOption {
  value: string;
  label: string;
  disabled?: boolean;
  /** Extra words that also match the search. */
  keywords?: readonly string[];
}

export interface ComboboxProps {
  options: readonly ComboboxOption[];
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  /** Shown on the button when nothing is chosen. Pass a translated string. */
  placeholder: string;
  /** Accessible name and placeholder of the search box. Pass a translated string. */
  searchLabel: string;
  disabled?: boolean;
  defaultOpen?: boolean;
  id?: string;
  'aria-label'?: string;
  'aria-describedby'?: string;
  'aria-invalid'?: AriaAttributes['aria-invalid'];
  className?: string;
}

/**
 * A single choice from a long list, with a search box. The button is a combobox;
 * the list opens under it, filters as you type, and Enter chooses. Use Select for
 * short lists.
 */
export function Combobox({
  options,
  value: controlledValue,
  defaultValue,
  onValueChange,
  placeholder,
  searchLabel,
  disabled,
  defaultOpen = false,
  id,
  'aria-label': ariaLabel,
  'aria-describedby': describedBy,
  'aria-invalid': invalid,
  className,
}: ComboboxProps) {
  const intl = useIntl();
  const [open, setOpen] = useState(defaultOpen);
  const [innerValue, setInnerValue] = useState(defaultValue);
  const value = controlledValue ?? innerValue;
  const control = useFieldControlProps({ id, 'aria-describedby': describedBy, 'aria-invalid': invalid });
  const selected = options.find((option) => option.value === value);

  const choose = (next: string) => {
    if (controlledValue === undefined) setInnerValue(next);
    onValueChange?.(next);
    setOpen(false);
  };

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          role="combobox"
          aria-expanded={open}
          aria-haspopup="listbox"
          aria-label={ariaLabel}
          disabled={disabled}
          {...control}
          className={cn(textControl, 'flex h-control items-center justify-between gap-2 text-left', !selected && 'text-fg-subtle', className)}
        >
          <span className="min-w-0 truncate">{selected ? selected.label : placeholder}</span>
          <ChevronDown aria-hidden className="text-fg-muted size-icon-16 shrink-0" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content sideOffset={4} align="start" collisionPadding={8} className={cn(floatingPanel, 'min-w-trigger p-1')}>
          <Command label={searchLabel}>
            <Command.Input
              placeholder={searchLabel}
              aria-label={searchLabel}
              className="border-border text-fg placeholder:text-fg-subtle mb-1 h-control min-h-target w-full border-b bg-transparent px-control-x text-base outline-hidden"
            />
            <CommandResults
              emptyMessage={intl.formatMessage(messages.empty)}
              emptyClassName="text-fg-muted px-control-x py-2 text-base"
              className="max-h-available overflow-y-auto"
            >
              {options.map((option) => (
                <Command.Item
                  key={option.value}
                  value={option.label}
                  keywords={option.keywords ? [...option.keywords] : undefined}
                  disabled={option.disabled}
                  onSelect={() => choose(option.value)}
                  className={commandItem}
                >
                  <span className="min-w-0 flex-1 truncate">{option.label}</span>
                  {option.value === value ? <Check aria-hidden className="size-icon-16 shrink-0" /> : null}
                </Command.Item>
              ))}
            </CommandResults>
          </Command>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

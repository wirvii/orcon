// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Checkbox } from '../../src/components/checkbox/Checkbox';
import { ErrorSummary } from '../../src/components/error-summary/ErrorSummary';
import { FormField } from '../../src/components/form-field/FormField';
import { Input } from '../../src/components/input/Input';
import { RadioGroup, RadioGroupItem } from '../../src/components/radio-group/RadioGroup';
import { Switch } from '../../src/components/switch/Switch';
import { Textarea } from '../../src/components/textarea/Textarea';

import { renderWithProviders } from './helpers';

describe('FormField', () => {
  it('names the control with the label and links the description', () => {
    renderWithProviders(
      <FormField label="Flow name" description="Shown in the list.">
        <Input />
      </FormField>,
    );
    const input = screen.getByRole('textbox', { name: 'Flow name' });
    expect(input).toHaveAccessibleDescription('Shown in the list.');
    expect(input).not.toBeInvalid();
  });

  it('marks the control invalid and links the error text', () => {
    renderWithProviders(
      <FormField label="Flow name" error="Enter a name.">
        <Input />
      </FormField>,
    );
    const input = screen.getByRole('textbox');
    expect(input).toBeInvalid();
    expect(input).toHaveAccessibleDescription(/Enter a name\./);
  });

  it('marks required in text for screen readers, not only with a symbol', () => {
    renderWithProviders(
      <FormField label="Flow name" required>
        <Input />
      </FormField>,
    );
    expect(screen.getByRole('textbox', { name: 'Flow name (required)' })).toBeRequired();
  });

  it('translates the required and error prefixes', () => {
    renderWithProviders(
      <FormField label="Nombre" required error="Escribe un nombre.">
        <Input />
      </FormField>,
      { locale: 'es' },
    );
    expect(screen.getByRole('textbox', { name: 'Nombre (obligatorio)' })).toBeInTheDocument();
    expect(screen.getByText(/Error:/)).toBeInTheDocument();
  });

  it('uses the given id so a summary can link to it', () => {
    renderWithProviders(
      <FormField id="flow-name" label="Flow name">
        <Input />
      </FormField>,
    );
    expect(screen.getByRole('textbox')).toHaveAttribute('id', 'flow-name');
  });

  it('works for textareas too', () => {
    renderWithProviders(
      <FormField label="Notes" error="Too long.">
        <Textarea />
      </FormField>,
    );
    expect(screen.getByRole('textbox', { name: 'Notes' })).toBeInvalid();
  });

  it('labels a group with a legend', () => {
    renderWithProviders(
      <FormField group label="Run mode" description="Applies to new runs.">
        <RadioGroup defaultValue="a">
          <RadioGroupItem value="a" label="A" />
          <RadioGroupItem value="b" label="B" />
        </RadioGroup>
      </FormField>,
    );
    expect(screen.getByRole('radiogroup', { name: 'Run mode' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Run mode' })).toHaveAccessibleDescription('Applies to new runs.');
  });

  it('keeps what a person typed when an error appears', async () => {
    const { rerender } = renderWithProviders(
      <FormField label="Flow name">
        <Input />
      </FormField>,
    );
    await userEvent.type(screen.getByRole('textbox'), 'Nightly');
    rerender(
      <FormField label="Flow name" error="Name already used.">
        <Input />
      </FormField>,
    );
    expect(screen.getByRole('textbox')).toHaveValue('Nightly');
  });
});

describe('choice controls', () => {
  it('toggles a checkbox when its label text is clicked', async () => {
    const onChange = vi.fn();
    renderWithProviders(<Checkbox label="Retry" onCheckedChange={onChange} />);
    await userEvent.click(screen.getByText('Retry'));
    expect(onChange).toHaveBeenCalledWith(true);
    expect(screen.getByRole('checkbox', { name: 'Retry' })).toBeChecked();
  });

  it('exposes the indeterminate state', () => {
    renderWithProviders(<Checkbox label="All" checked="indeterminate" />);
    expect(screen.getByRole('checkbox')).toBePartiallyChecked();
  });

  it('switches on and off by role, not by color', async () => {
    renderWithProviders(<Switch label="Enabled" />);
    const toggle = screen.getByRole('switch', { name: 'Enabled' });
    expect(toggle).not.toBeChecked();
    await userEvent.click(toggle);
    expect(toggle).toBeChecked();
  });

  it('selects one radio at a time', async () => {
    renderWithProviders(
      <RadioGroup aria-label="Mode">
        <RadioGroupItem value="a" label="A" />
        <RadioGroupItem value="b" label="B" />
      </RadioGroup>,
    );
    await userEvent.click(screen.getByRole('radio', { name: 'A' }));
    await userEvent.click(screen.getByRole('radio', { name: 'B' }));
    expect(screen.getByRole('radio', { name: 'A' })).not.toBeChecked();
    expect(screen.getByRole('radio', { name: 'B' })).toBeChecked();
  });
});

describe('ErrorSummary', () => {
  const errors = [
    { fieldId: 'a', message: 'Enter a.' },
    { fieldId: 'b', message: 'Enter b.' },
  ];

  it('uses the plural form for the count', () => {
    renderWithProviders(<ErrorSummary errors={errors} />);
    expect(screen.getByRole('heading', { name: 'There are 2 problems to fix' })).toBeInTheDocument();
  });

  it('uses the singular form for one error and the Spanish plural rules', () => {
    renderWithProviders(<ErrorSummary errors={[errors[0]!]} />, { locale: 'es' });
    expect(screen.getByRole('heading', { name: 'Hay 1 problema por corregir' })).toBeInTheDocument();
  });

  it('renders nothing without errors', () => {
    const { container } = renderWithProviders(<ErrorSummary errors={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('moves focus to the field when an entry is activated', async () => {
    renderWithProviders(
      <>
        <ErrorSummary errors={errors} />
        <FormField id="b" label="Field b" error="Enter b.">
          <Input />
        </FormField>
      </>,
    );
    await userEvent.click(screen.getByRole('link', { name: 'Enter b.' }));
    expect(screen.getByRole('textbox', { name: 'Field b' })).toHaveFocus();
  });

  it('takes focus on mount when asked, so a failed submit is announced', () => {
    renderWithProviders(<ErrorSummary errors={errors} focusOnMount />);
    expect(screen.getByRole('alert')).toHaveFocus();
  });
});

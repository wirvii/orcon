// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { Banner } from '../../src/components/banner/Banner';
import { ConnectorIcon } from '../../src/components/connector-icon/ConnectorIcon';
import { EmptyState } from '../../src/components/empty-state/EmptyState';
import { ErrorBoundary } from '../../src/components/error-boundary/ErrorBoundary';
import { ErrorSummary } from '../../src/components/error-summary/ErrorSummary';
import { FormField } from '../../src/components/form-field/FormField';
import { ProblemErrorsProvider } from '../../src/components/form-field/problem-errors';
import { Input } from '../../src/components/input/Input';
import { ProblemAlert } from '../../src/components/problem-alert/ProblemAlert';
import { fieldIdForName, problemFieldErrors, type ProblemDetails } from '../../src/components/problem-alert/problem';
import { ProtectedValue, RedactedValue } from '../../src/components/redacted-value/RedactedValue';

import { renderWithProviders } from './helpers';

afterEach(() => vi.restoreAllMocks());

/** Replaces the clipboard of the test browser with a spy. */
function stubClipboard(writeText: (text: string) => unknown) {
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
}

describe('Banner', () => {
  it('announces errors at once and other tones politely, with a text prefix', () => {
    renderWithProviders(<Banner tone="danger" title="Save failed" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Error: Save failed');
  });

  it('uses role status for the non-error tones', () => {
    renderWithProviders(<Banner tone="attention" title="Needs a decision" />);
    expect(screen.getByRole('status')).toHaveTextContent('Needs attention: Needs a decision');
  });
});

describe('ProblemAlert', () => {
  const problem: ProblemDetails = {
    title: 'Unprocessable',
    status: 422,
    code: 'validation-failed',
    type: 'https://orcon.example/problems/validation-failed',
    detail: 'The field "name" must not be empty.',
    requestId: 'req-123',
    errors: [
      { pointer: '/name', detail: 'Enter a name.' },
      { pointer: '/owner', detail: 'Choose an owner.' },
    ],
  };

  it('falls back to the title when the code has no translation', () => {
    renderWithProviders(<ProblemAlert problem={problem} />);
    expect(screen.getByRole('alert')).toHaveTextContent('Unprocessable');
  });

  it('translates by code in both languages, ignoring the title', () => {
    const resolved = { code: 'attention-already-resolved', title: 'Conflict', status: 409 };
    const { unmount } = renderWithProviders(<ProblemAlert problem={resolved} />);
    expect(screen.getByText('Someone already resolved this request.')).toBeInTheDocument();
    unmount();
    renderWithProviders(<ProblemAlert problem={resolved} />, { locale: 'es' });
    expect(screen.getByText('Alguien ya resolvió esta solicitud.')).toBeInTheDocument();
  });

  it('uses a generic message when there is neither a translation nor a title', () => {
    renderWithProviders(<ProblemAlert problem={{ status: 500 }} />);
    expect(screen.getByText('Something went wrong.')).toBeInTheDocument();
  });

  it('keeps the raw detail in a collapsed "Technical details" section', async () => {
    renderWithProviders(<ProblemAlert problem={problem} />);
    const summary = screen.getByText('Technical details');
    const details = summary.closest('details');
    expect(details).not.toHaveAttribute('open');
    await userEvent.click(summary);
    expect(details).toHaveAttribute('open');
    expect(screen.getByText('The field "name" must not be empty.')).toBeInTheDocument();
    expect(screen.getByText('422')).toBeInTheDocument();
  });

  it('shows the request id and copies it', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    stubClipboard(writeText);
    renderWithProviders(<ProblemAlert problem={problem} />);
    expect(screen.getByText('req-123')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Copy request ID' }));
    expect(writeText).toHaveBeenCalledWith('req-123');
    expect(await screen.findByText('Copied')).toBeInTheDocument();
  });

  it('shows the current result for 409 attention-already-resolved instead of a generic error', () => {
    renderWithProviders(
      <ProblemAlert
        problem={{ code: 'attention-already-resolved', status: 409 }}
        currentResult="Approved by Ada Lovelace."
      />,
    );
    expect(screen.getByText('Approved by Ada Lovelace.')).toBeInTheDocument();
    // Not announced as an error.
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('assigns errors[].pointer to the FormField whose name equals the pointer', () => {
    renderWithProviders(
      <ProblemErrorsProvider problem={problem}>
        <FormField name="/name" label="Name">
          <Input />
        </FormField>
        <FormField name="/owner" label="Owner">
          <Input />
        </FormField>
        <FormField name="/notes" label="Notes">
          <Input />
        </FormField>
      </ProblemErrorsProvider>,
    );
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveAccessibleDescription(/Enter a name\./);
    expect(screen.getByRole('textbox', { name: 'Owner' })).toHaveAccessibleDescription(/Choose an owner\./);
    expect(screen.getByRole('textbox', { name: 'Notes' })).not.toBeInvalid();
  });

  it('builds the error summary entries and links them to the same ids', async () => {
    const errors = problemFieldErrors(problem);
    expect(errors).toEqual([
      { fieldId: fieldIdForName('/name'), message: 'Enter a name.' },
      { fieldId: fieldIdForName('/owner'), message: 'Choose an owner.' },
    ]);
    renderWithProviders(
      <ProblemErrorsProvider problem={problem}>
        <ErrorSummary errors={errors} />
        <FormField name="/owner" label="Owner">
          <Input />
        </FormField>
      </ProblemErrorsProvider>,
    );
    await userEvent.click(screen.getByRole('link', { name: 'Choose an owner.' }));
    expect(screen.getByRole('textbox', { name: 'Owner' })).toHaveFocus();
  });

  it('never clears what a person typed when the error arrives', async () => {
    const { rerender } = renderWithProviders(
      <ProblemErrorsProvider problem={undefined}>
        <FormField name="/name" label="Name">
          <Input />
        </FormField>
      </ProblemErrorsProvider>,
    );
    await userEvent.type(screen.getByRole('textbox'), 'Nightly');
    rerender(
      <ProblemErrorsProvider problem={problem}>
        <FormField name="/name" label="Name">
          <Input />
        </FormField>
      </ProblemErrorsProvider>,
    );
    expect(screen.getByRole('textbox')).toHaveValue('Nightly');
    expect(screen.getByRole('textbox')).toBeInvalid();
  });
});

describe('RedactedValue', () => {
  it('says that the value is hidden and why, never empty', () => {
    renderWithProviders(<RedactedValue />);
    expect(screen.getByText('Hidden — you need permission to view execution data')).toBeInTheDocument();
  });

  it('translates the notice', () => {
    renderWithProviders(<RedactedValue />, { locale: 'es' });
    expect(screen.getByText(/Oculto: necesitas permiso/)).toBeInTheDocument();
  });

  it('keeps a hidden value out of the page and the clipboard', () => {
    const writeText = vi.fn();
    stubClipboard(writeText);
    const { container } = renderWithProviders(<ProtectedValue value="sk-live-secret" hidden />);
    expect(container.textContent).not.toContain('sk-live-secret');
    expect(container.innerHTML).not.toContain('sk-live-secret');
    expect(screen.queryByRole('button')).toBeNull();
    expect(writeText).not.toHaveBeenCalled();
  });

  it('shows and copies a visible value', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    stubClipboard(writeText);
    renderWithProviders(<ProtectedValue value="order-48213" hidden={false} />);
    expect(screen.getByText('order-48213')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Copy value' }));
    expect(writeText).toHaveBeenCalledWith('order-48213');
  });
});

describe('EmptyState, ConnectorIcon', () => {
  it('EmptyState uses the requested heading level and renders its action', () => {
    renderWithProviders(<EmptyState title="No flows yet" headingLevel={3} action={<button type="button">Create</button>} />);
    expect(screen.getByRole('heading', { level: 3, name: 'No flows yet' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
  });

  it('ConnectorIcon renders an <img> named after the connector, never inline SVG', () => {
    const { container } = renderWithProviders(<ConnectorIcon src="data:image/png;base64,AA==" name="Example" />);
    expect(screen.getByRole('img', { name: 'Example' }).tagName).toBe('IMG');
    expect(container.querySelector('svg')).toBeNull();
  });
});

describe('ErrorBoundary', () => {
  function Broken(): never {
    throw new Error('boom');
  }

  it('replaces a crashing child with a recoverable fallback and reports the error', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const onError = vi.fn();
    renderWithProviders(
      <ErrorBoundary onError={onError}>
        <Broken />
      </ErrorBoundary>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong on this screen.');
    expect(onError).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('renders the children again after Try again when the cause is gone', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    let broken = true;
    function Maybe() {
      if (broken) throw new Error('boom');
      return <p>Recovered</p>;
    }
    renderWithProviders(
      <ErrorBoundary>
        <Maybe />
      </ErrorBoundary>,
    );
    broken = false;
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(screen.getByText('Recovered')).toBeInTheDocument();
  });
});

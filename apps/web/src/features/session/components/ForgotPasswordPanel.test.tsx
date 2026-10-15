// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderInApp } from '../../../../test/screen-harness';

import { ForgotPasswordPanel } from './ForgotPasswordPanel';

describe('ForgotPasswordPanel', () => {
  it('explains the administrator reset when the reset operation does not exist', async () => {
    const onBack = vi.fn();
    renderInApp(<ForgotPasswordPanel onBack={onBack} />);
    expect(screen.getByRole('heading', { level: 1, name: 'Recover your password' })).toHaveFocus();
    // It asks for nothing it cannot use: no field, no send button.
    expect(screen.queryByLabelText(/^Email/)).toBeNull();
    expect(screen.queryByRole('button', { name: 'Send the link' })).toBeNull();
    expect(screen.getByText('Ask whoever administers your organization')).toBeVisible();
    expect(screen.getByText('orcon accounts set-password')).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(onBack).toHaveBeenCalled();
  });

  it('shows the same confirmation for any email when the reset operation exists', async () => {
    // One address the server knows and one it does not (or fails for): the person sees the same thing.
    const requestReset = vi.fn((email: string) => (email.startsWith('known') ? Promise.resolve() : Promise.reject(new Error('unknown account'))));
    for (const email of ['known@example.com', 'nobody@example.com']) {
      const view = renderInApp(<ForgotPasswordPanel onBack={() => undefined} requestReset={requestReset} />);
      await userEvent.type(screen.getByLabelText(/^Email/), email);
      await userEvent.click(screen.getByRole('button', { name: 'Send the link' }));
      expect(await screen.findByText('Check your email.')).toBeVisible();
      expect(screen.getByText('If an account exists with that email, you will get the link in a minute. Check your spam folder too.')).toBeVisible();
      expect(requestReset).toHaveBeenLastCalledWith(email);
      view.unmount();
    }
  });

  it('starts with the email typed on the sign-in form', () => {
    renderInApp(<ForgotPasswordPanel onBack={() => undefined} requestReset={() => Promise.resolve()} initialEmail="ana@example.com" />);
    expect(screen.getByLabelText(/^Email/)).toHaveValue('ana@example.com');
  });

  it('sends nothing for an empty email', async () => {
    const requestReset = vi.fn(() => Promise.resolve());
    renderInApp(<ForgotPasswordPanel onBack={() => undefined} requestReset={requestReset} />);
    await userEvent.click(screen.getByRole('button', { name: 'Send the link' }));
    expect(requestReset).not.toHaveBeenCalled();
    expect(screen.queryByText('Check your email.')).toBeNull();
  });
});

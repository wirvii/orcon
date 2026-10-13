// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react';
import { useIntl } from 'react-intl';
import { describe, expect, it, vi } from 'vitest';

import { Providers, type ProvidersProps } from '../../src/providers/Providers';

const appMessages = {
  en: { 'app.hello': 'Hello from the app', 'bell.label': 'Overridden by the app' },
  es: { 'app.hello': 'Hola desde la app', 'bell.label': 'Reemplazado por la app' },
};

function Probe({ id }: { id: string }) {
  const intl = useIntl();
  return <p>{intl.formatMessage({ id, defaultMessage: 'default text', description: 'Probe for the app messages test.' })}</p>;
}

function renderProbe(id: string, props: Omit<ProvidersProps, 'children'>) {
  return render(
    <Providers {...props}>
      <Probe id={id} />
    </Providers>,
  );
}

describe('Providers with application messages', () => {
  it('shows the application message of the active locale', () => {
    renderProbe('app.hello', { locale: 'en', messages: appMessages });
    expect(screen.getByText('Hello from the app')).toBeInTheDocument();
  });

  it('switches with the locale', () => {
    renderProbe('app.hello', { locale: 'es', messages: appMessages });
    expect(screen.getByText('Hola desde la app')).toBeInTheDocument();
  });

  it('keeps the design system messages the application does not define', () => {
    renderProbe('common.field.required', { locale: 'en', messages: appMessages });
    expect(screen.getByText('required')).toBeInTheDocument();
  });

  it('lets the application replace a design system message', () => {
    renderProbe('bell.label', { locale: 'en', messages: appMessages });
    expect(screen.getByText('Overridden by the app')).toBeInTheDocument();
  });

  it('gives the pseudo-locale the English application messages', () => {
    // The runtime has no number data for the pseudo-locale and says so; that is not what this test is about.
    const quiet = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    renderProbe('app.hello', { locale: 'xx-LS', messages: appMessages });
    expect(screen.getByText('Hello from the app')).toBeInTheDocument();
    quiet.mockRestore();
  });

  it('works without application messages', () => {
    renderProbe('common.field.required', { locale: 'es' });
    expect(screen.getByText('obligatorio')).toBeInTheDocument();
  });
});

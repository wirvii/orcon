// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { useTheme, type ThemePreference } from '../../providers/ThemeProvider';
import { RadioGroup, RadioGroupItem } from '../radio-group/RadioGroup';

const messages = defineMessages({
  label: { id: 'theme.label', defaultMessage: 'Theme', description: 'Name of the group of options that choose the color theme.' },
  system: { id: 'theme.system', defaultMessage: 'System', description: 'Theme option that follows the operating system setting.' },
  light: { id: 'theme.light', defaultMessage: 'Light', description: 'Theme option for the light color theme.' },
  dark: { id: 'theme.dark', defaultMessage: 'Dark', description: 'Theme option for the dark color theme.' },
});

export interface ThemeSwitcherProps {
  /** Controlled value. Without it the switcher reads and sets the ThemeProvider preference. */
  value?: ThemePreference;
  onValueChange?: (value: ThemePreference) => void;
  className?: string;
}

/** Chooses system, light or dark. The choice is stored and applied by ThemeProvider. */
export function ThemeSwitcher({ value, onValueChange, className }: ThemeSwitcherProps) {
  const intl = useIntl();
  const theme = useTheme();
  const current = value ?? theme.preference;
  return (
    <RadioGroup
      aria-label={intl.formatMessage(messages.label)}
      value={current}
      onValueChange={(next) => {
        const preference = next as ThemePreference;
        if (onValueChange) onValueChange(preference);
        else theme.setPreference(preference);
      }}
      className={cn('flex-col sm:flex-row', className)}
    >
      <RadioGroupItem value="system" label={intl.formatMessage(messages.system)} />
      <RadioGroupItem value="light" label={intl.formatMessage(messages.light)} />
      <RadioGroupItem value="dark" label={intl.formatMessage(messages.dark)} />
    </RadioGroup>
  );
}

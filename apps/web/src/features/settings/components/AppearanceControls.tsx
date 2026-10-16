// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn, Switch, useDensity, useLocale, useTheme, type Density, type ThemePreference } from '@orcon/ui';
import { Check } from 'lucide-react';
import { useId, useState, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  theme: { id: 'settings.appearance.theme', defaultMessage: 'Theme', description: 'Label of the choice between dark, light and the setting of the device.' },
  dark: { id: 'settings.appearance.theme.dark', defaultMessage: 'Dark', description: 'Theme option for the dark color theme.' },
  light: { id: 'settings.appearance.theme.light', defaultMessage: 'Light', description: 'Theme option for the light color theme.' },
  system: { id: 'settings.appearance.theme.system', defaultMessage: 'Like the system', description: 'Theme option that follows the setting of the device.' },
  density: { id: 'settings.appearance.density', defaultMessage: 'Density', description: 'Label of the choice of how much fits on a screen.' },
  comfortable: { id: 'settings.appearance.density.comfortable', defaultMessage: 'Comfortable', description: 'Density option: easier to read and to tap.' },
  compact: { id: 'settings.appearance.density.compact', defaultMessage: 'Compact', description: 'Density option: fits more rows on screen.' },
  language: { id: 'settings.appearance.language', defaultMessage: 'Interface language', description: 'Label of the choice of the language of the interface.' },
  english: { id: 'settings.appearance.language.en', defaultMessage: 'English', description: 'Language option. It is the name of the language in that language, so it is the same in every catalog.' },
  spanish: { id: 'settings.appearance.language.es', defaultMessage: 'Español', description: 'Language option. It is the name of the language in that language, so it is the same in every catalog.' },
  motion: { id: 'settings.appearance.motion', defaultMessage: 'Reduce animations', description: 'Switch that turns off the movement of the interface: transitions and looping animations.' },
  motionHint: {
    id: 'settings.appearance.motion.hint',
    defaultMessage: 'Panels and menus appear without moving. Your device setting for less motion is always respected.',
    description: 'One line under the reduce animations switch that says what it changes.',
  },
  saved: { id: 'settings.appearance.saved', defaultMessage: 'Saved automatically', description: 'Confirmation, read aloud politely, after a choice was applied and kept in this browser.' },
});

/** A page drawn with the tokens of one theme, whatever theme is shown now: `data-theme` re-scopes the tokens below it. */
function PreviewPage({ variant, className }: { variant: 'dark' | 'light'; className?: string | undefined }) {
  return (
    <span data-theme={variant} className={cn('bg-app border-border flex h-12 min-w-0 flex-1 overflow-hidden rounded-md border', className)}>
      <span className="bg-surface border-border w-1/4 border-r" />
      <span className="flex flex-1 flex-col gap-1 p-2">
        <span className="bg-strong h-1 w-3/4 rounded-sm" />
        <span className="bg-action-primary h-1 w-2/5 rounded-sm" />
        <span className="bg-border h-1 w-full rounded-sm" />
      </span>
    </span>
  );
}

/** The three previews of a theme. The system one is half light and half dark. */
function ThemePreview({ theme }: { theme: 'dark' | 'light' | 'system' }) {
  return (
    <span aria-hidden className="flex">
      {theme === 'system' ? (
        <>
          <PreviewPage variant="light" className="rounded-r-none" />
          <PreviewPage variant="dark" className="rounded-l-none" />
        </>
      ) : (
        <PreviewPage variant={theme} />
      )}
    </span>
  );
}

const choiceRing = 'peer-focus-visible:outline-solid peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-focus';

function useAutosaveNotice() {
  const [saved, setSaved] = useState(false);
  return { saved, markSaved: () => setSaved(true) };
}

/**
 * The appearance choices of Settings. Every choice applies the moment it is made and is kept in this browser, so there
 * is no Save button; a polite live region says "Saved automatically" after each one (SPEC-019 R-UX-SET-03).
 */
export interface AppearanceControlsProps {
  reducedMotion: boolean;
  onReducedMotionChange: (reduced: boolean) => void;
}

export function AppearanceControls({ reducedMotion, onReducedMotionChange }: AppearanceControlsProps) {
  const intl = useIntl();
  const theme = useTheme();
  const density = useDensity();
  const language = useLocale();
  const { saved, markSaved } = useAutosaveNotice();
  const name = useId();

  const themeOptions: { value: ThemePreference; label: string }[] = [
    { value: 'dark', label: intl.formatMessage(messages.dark) },
    { value: 'light', label: intl.formatMessage(messages.light) },
    { value: 'system', label: intl.formatMessage(messages.system) },
  ];
  const densityOptions: { value: Density; label: string }[] = [
    { value: 'comfortable', label: intl.formatMessage(messages.comfortable) },
    { value: 'compact', label: intl.formatMessage(messages.compact) },
  ];
  const languageOptions: { value: 'en' | 'es'; label: string }[] = [
    { value: 'es', label: intl.formatMessage(messages.spanish) },
    { value: 'en', label: intl.formatMessage(messages.english) },
  ];
  const currentLanguage = language.locale === 'es' ? 'es' : 'en';

  return (
    <div className="flex flex-col gap-5">
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">{intl.formatMessage(messages.theme)}</legend>
        <div className="grid grid-cols-3 gap-3 max-md:gap-2">
          {themeOptions.map((option) => (
            <label key={option.value} className="relative flex">
              <input
                type="radio"
                name={`${name}-theme`}
                value={option.value}
                checked={theme.preference === option.value}
                onChange={() => {
                  theme.setPreference(option.value);
                  markSaved();
                }}
                className="peer sr-only"
              />
              <span
                className={cn(
                  'border-strong hover:border-control bg-surface flex w-full flex-col gap-2 rounded-lg border p-2 text-left text-sm font-semibold',
                  'peer-checked:border-canvas-selection peer-checked:ring-1 peer-checked:ring-canvas-selection',
                  choiceRing,
                )}
              >
                <ThemePreview theme={option.value} />
                {option.label}
              </span>
            </label>
          ))}
        </div>
      </fieldset>

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
        <Segmented
          legend={intl.formatMessage(messages.density)}
          name={`${name}-density`}
          value={density.density}
          options={densityOptions}
          onChange={(value) => {
            density.setDensity(value);
            markSaved();
          }}
        />
        <Segmented
          legend={intl.formatMessage(messages.language)}
          name={`${name}-language`}
          value={currentLanguage}
          options={languageOptions}
          onChange={(value) => {
            language.setLocale(value);
            markSaved();
          }}
        />
      </div>

      <div className="flex flex-col gap-1">
        <Switch
          label={intl.formatMessage(messages.motion)}
          checked={reducedMotion}
          onCheckedChange={(checked) => {
            onReducedMotionChange(checked);
            markSaved();
          }}
        />
        <p className="text-fg-muted text-xs">{intl.formatMessage(messages.motionHint)}</p>
      </div>

      {/* Polite: a screen reader says it after the choice, without taking the focus. */}
      <p role="status" aria-live="polite" className="text-status-success flex min-h-icon-16 items-center gap-1 text-xs">
        {saved ? (
          <>
            <Check aria-hidden className="size-icon-16" />
            {intl.formatMessage(messages.saved)}
          </>
        ) : null}
      </p>
    </div>
  );
}

interface SegmentedProps<T extends string> {
  legend: ReactNode;
  name: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}

/** A row of radio buttons drawn as a segmented control: native radios, so the arrow keys and the group are the browser's. */
function Segmented<T extends string>({ legend, name, value, options, onChange }: SegmentedProps<T>) {
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="text-sm font-medium">{legend}</legend>
      <div className="bg-surface-sunken border-border flex gap-1 rounded-md border p-1">
        {options.map((option) => (
          <label key={option.value} className="relative flex flex-1">
            <input type="radio" name={name} value={option.value} checked={value === option.value} onChange={() => { onChange(option.value); }} className="peer sr-only" />
            <span
              className={cn(
                'text-fg-muted hover:text-fg flex min-h-target w-full items-center justify-center rounded-base px-3 text-xs font-medium transition-colors',
                'peer-checked:bg-fg peer-checked:text-app peer-checked:font-semibold',
                choiceRing,
              )}
            >
              {option.label}
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

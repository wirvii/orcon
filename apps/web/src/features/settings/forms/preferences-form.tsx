// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import {
  Banner,
  Button,
  DensitySwitcher,
  ErrorSummary,
  FormField,
  LocaleSwitcher,
  ThemeSwitcher,
  cn,
  fieldIdForName,
  useDensity,
  useLocale,
  useTheme,
} from '@orcon/ui';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { defineMessages, useIntl } from 'react-intl';

import { formatIssue } from '../../../app/forms/zod-messages';
import { changedPreferences, preferenceOfSection, preferencesSchema, type Preferences, type Section } from '../model/preferences';

const messages = defineMessages({
  themeTitle: { id: 'settings.appearance.theme.title', defaultMessage: 'Theme', description: 'Heading of the section that chooses light, dark or the system theme.' },
  themeText: {
    id: 'settings.appearance.theme.text',
    defaultMessage: 'System follows the setting of your device.',
    description: 'Explains the theme options: system follows the device.',
  },
  themeLabel: { id: 'settings.appearance.theme.label', defaultMessage: 'Color theme', description: 'Label of the control that chooses the color theme.' },
  densityTitle: { id: 'settings.appearance.density.title', defaultMessage: 'Density', description: 'Heading of the section that chooses how much fits on a screen.' },
  densityText: {
    id: 'settings.appearance.density.text',
    defaultMessage: 'Compact fits more rows on screen. Comfortable is easier to read and to tap.',
    description: 'Explains the two densities: compact and comfortable.',
  },
  densityLabel: { id: 'settings.appearance.density.label', defaultMessage: 'Row density', description: 'Label of the control that chooses the density.' },
  languageTitle: { id: 'settings.appearance.language.title', defaultMessage: 'Language', description: 'Heading of the section that chooses the language of the interface.' },
  languageText: {
    id: 'settings.appearance.language.text',
    defaultMessage: 'Dates, times and numbers follow the language you choose.',
    description: 'Explains that formats follow the chosen language.',
  },
  languageLabel: { id: 'settings.appearance.language.label', defaultMessage: 'Interface language', description: 'Label of the control that chooses the language.' },
  save: { id: 'settings.appearance.save', defaultMessage: 'Save preferences', description: 'Button that applies and saves the appearance choices.' },
  saved: {
    id: 'settings.appearance.saved',
    defaultMessage: 'Preferences saved',
    description: 'Confirmation shown after the appearance choices were applied and saved in this browser.',
  },
  savedText: {
    id: 'settings.appearance.savedText',
    defaultMessage: 'They are kept in this browser.',
    description: 'Explains where the saved appearance choices live.',
  },
});

interface AppearanceSectionProps {
  id: string;
  title: string;
  text: string;
  /** The URL points at this section. */
  selected: boolean;
  headingRef: (element: HTMLHeadingElement | null) => void;
  children: ReactNode;
}

function AppearanceSection({ id, title, text, selected, headingRef, children }: AppearanceSectionProps) {
  return (
    <section
      id={id}
      aria-labelledby={`${id}-title`}
      className={cn('bg-surface flex flex-col gap-2 rounded-lg border p-4', selected ? 'border-action-primary' : 'border-border')}
    >
      <h2 id={`${id}-title`} tabIndex={-1} ref={headingRef} className="text-lg font-semibold wrap-anywhere">
        {title}
      </h2>
      <p className="text-fg-muted wrap-anywhere">{text}</p>
      {children}
    </section>
  );
}

export interface PreferencesFormProps {
  /** The section named by the URL: its heading takes focus, so a link lands where it points. */
  section: Section | undefined;
}

/**
 * The appearance preferences as one form: React Hook Form with the Zod schema of the model, the
 * fields named by JSON pointer, messages from the catalog. Saving applies the changed choices
 * through the design system providers, which keep them in this browser.
 */
export function PreferencesForm({ section }: PreferencesFormProps) {
  const intl = useIntl();
  const theme = useTheme();
  const density = useDensity();
  const language = useLocale();
  const [saved, setSaved] = useState(false);
  const headings = useRef<Partial<Record<keyof Preferences, HTMLHeadingElement | null>>>({});

  const current = useMemo<Preferences>(
    () => ({ theme: theme.preference, density: density.density, locale: language.locale === 'xx-LS' ? 'en' : language.locale }),
    [theme.preference, density.density, language.locale],
  );

  const form = useForm<Preferences>({
    resolver: zodResolver(preferencesSchema, { error: (issue) => formatIssue(intl, issue) }),
    mode: 'onTouched',
    reValidateMode: 'onChange',
    // The providers are the source of the values; edits in progress survive a change of language.
    values: current,
    resetOptions: { keepDirtyValues: true },
  });
  const { isDirty, errors, submitCount } = form.formState;
  const selected = section === undefined ? undefined : preferenceOfSection(section);

  useEffect(() => {
    if (selected) headings.current[selected]?.focus();
  }, [selected]);

  useEffect(() => {
    if (isDirty) setSaved(false);
  }, [isDirty]);

  const submit = form.handleSubmit((values) => {
    const changed = changedPreferences(current, values);
    if (changed.theme) theme.setPreference(changed.theme);
    if (changed.density) density.setDensity(changed.density);
    if (changed.locale) language.setLocale(changed.locale);
    setSaved(true);
  });

  const summary = Object.entries(errors).flatMap(([name, error]) =>
    error.message ? [{ fieldId: fieldIdForName(`/${name}`), message: error.message }] : [],
  );

  return (
    <form noValidate onSubmit={(event) => void submit(event)} className="flex max-w-screen-md flex-col gap-4">
      {submitCount > 0 ? <ErrorSummary errors={summary} focusOnMount /> : null}
      {saved ? (
        <Banner tone="success" title={intl.formatMessage(messages.saved)}>
          {intl.formatMessage(messages.savedText)}
        </Banner>
      ) : null}

      <AppearanceSection
        id="theme"
        title={intl.formatMessage(messages.themeTitle)}
        text={intl.formatMessage(messages.themeText)}
        selected={selected === 'theme'}
        headingRef={(element) => {
          headings.current.theme = element;
        }}
      >
        <FormField group name="/theme" label={intl.formatMessage(messages.themeLabel)} error={errors.theme?.message}>
          <Controller control={form.control} name="theme" render={({ field }) => <ThemeSwitcher value={field.value} onValueChange={field.onChange} />} />
        </FormField>
      </AppearanceSection>

      <AppearanceSection
        id="density"
        title={intl.formatMessage(messages.densityTitle)}
        text={intl.formatMessage(messages.densityText)}
        selected={selected === 'density'}
        headingRef={(element) => {
          headings.current.density = element;
        }}
      >
        <FormField group name="/density" label={intl.formatMessage(messages.densityLabel)} error={errors.density?.message}>
          <Controller control={form.control} name="density" render={({ field }) => <DensitySwitcher value={field.value} onValueChange={field.onChange} />} />
        </FormField>
      </AppearanceSection>

      <AppearanceSection
        id="language"
        title={intl.formatMessage(messages.languageTitle)}
        text={intl.formatMessage(messages.languageText)}
        selected={selected === 'locale'}
        headingRef={(element) => {
          headings.current.locale = element;
        }}
      >
        <FormField name="/locale" label={intl.formatMessage(messages.languageLabel)} error={errors.locale?.message}>
          <Controller control={form.control} name="locale" render={({ field }) => <LocaleSwitcher value={field.value} onValueChange={field.onChange} />} />
        </FormField>
      </AppearanceSection>

      <div>
        <Button type="submit">{intl.formatMessage(messages.save)}</Button>
      </div>
    </form>
  );
}

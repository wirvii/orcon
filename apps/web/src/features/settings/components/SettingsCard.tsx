// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useId, type ReactNode } from 'react';

export interface SettingsCardProps {
  title: string;
  description?: string | undefined;
  children: ReactNode;
}

/** One block of a settings page: a heading, one line that says what it is, and its content. */
export function SettingsCard({ title, description, children }: SettingsCardProps) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="bg-surface border-border flex flex-col gap-4 rounded-lg border p-5">
      <div className="flex flex-col gap-1">
        <h2 id={headingId} className="text-md font-semibold wrap-anywhere">
          {title}
        </h2>
        {description ? <p className="text-fg-muted wrap-anywhere">{description}</p> : null}
      </div>
      {children}
    </section>
  );
}

// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

import { Breadcrumb, type BreadcrumbItem } from '../components/breadcrumb/Breadcrumb';

export interface PageHeaderProps {
  title: string;
  description?: ReactNode;
  breadcrumb?: readonly BreadcrumbItem[];
  /** Buttons at the end of the header. They wrap under the title on a phone. */
  actions?: ReactNode;
}

/** The top of a page: where you are, the h1, a line of context and the page actions. */
export function PageHeader({ title, description, breadcrumb, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-col gap-2 pb-4">
      {breadcrumb ? <Breadcrumb items={breadcrumb} /> : null}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-semibold break-words">{title}</h1>
          {description ? <p className="text-fg-muted break-words">{description}</p> : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
    </header>
  );
}

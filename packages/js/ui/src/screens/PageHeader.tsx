// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

import { Breadcrumb, type BreadcrumbItem } from '../components/breadcrumb/Breadcrumb';

export interface PageHeaderProps {
  title: string;
  description?: ReactNode | undefined;
  breadcrumb?: readonly BreadcrumbItem[] | undefined;
  /** Buttons at the end of the header. They wrap under the title on a phone. */
  actions?: ReactNode | undefined;
}

/** The top of a page: where you are, the h1, a line of context and the page actions. */
export function PageHeader({ title, description, breadcrumb, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-col gap-2 pb-4">
      {/* On a phone the breadcrumb gives way: the top bar already says where you are. */}
      {breadcrumb ? <Breadcrumb items={breadcrumb} className="max-md:hidden" /> : null}
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-semibold wrap-anywhere">{title}</h1>
          {description ? <p className="text-fg-muted wrap-anywhere">{description}</p> : null}
        </div>
        {/* The actions take the whole row on a phone, where they are easy to reach. */}
        {actions ? <div className="flex flex-wrap items-center gap-2 max-md:*:flex-1">{actions}</div> : null}
      </div>
    </header>
  );
}

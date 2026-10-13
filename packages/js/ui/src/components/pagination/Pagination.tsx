// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ChevronLeft, ChevronRight } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Button } from '../button/Button';

const messages = defineMessages({
  label: { id: 'pagination.label', defaultMessage: 'Pagination', description: 'Accessible name of the navigation landmark that moves between pages of results.' },
  previous: { id: 'pagination.previous', defaultMessage: 'Previous', description: 'Button that shows the previous page of results.' },
  next: { id: 'pagination.next', defaultMessage: 'Next', description: 'Button that shows the next page of results.' },
  position: {
    id: 'pagination.position',
    defaultMessage: 'Page {page} of {count}',
    description: 'Current position in a paged list. {page} is the current page and {count} the total number of pages.',
  },
});

export interface PaginationProps {
  /** Current page, starting at 1. */
  page: number;
  pageCount: number;
  onPageChange: (page: number) => void;
  className?: string;
}

/** Previous and next with the current position in words. The position is announced politely when it changes. */
export function Pagination({ page, pageCount, onPageChange, className }: PaginationProps) {
  const intl = useIntl();
  return (
    <nav aria-label={intl.formatMessage(messages.label)} className={cn('flex flex-wrap items-center justify-between gap-2', className)}>
      <Button variant="secondary" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
        <ChevronLeft aria-hidden className="size-icon-16" />
        {intl.formatMessage(messages.previous)}
      </Button>
      <span aria-live="polite" className="text-fg-muted text-sm">
        {intl.formatMessage(messages.position, { page, count: pageCount })}
      </span>
      <Button variant="secondary" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)}>
        {intl.formatMessage(messages.next)}
        <ChevronRight aria-hidden className="size-icon-16" />
      </Button>
    </nav>
  );
}

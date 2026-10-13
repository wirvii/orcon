// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react';

import { Button, type ButtonProps } from '../button/Button';

export interface IconButtonProps extends Omit<ButtonProps, 'children' | 'size' | 'aria-label'> {
  /** Accessible name. Required: an icon alone does not say what the button does. */
  label: string;
  icon: LucideIcon;
}

/** A square button with an icon and no visible text. The label is its accessible name. */
export function IconButton({ label, icon: Icon, loading, ...props }: IconButtonProps) {
  return (
    <Button size="icon" aria-label={label} loading={loading} {...props}>
      {loading ? null : <Icon aria-hidden className="size-icon-16" />}
    </Button>
  );
}

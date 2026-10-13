// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { useDensity, type Density } from '../../providers/DensityProvider';
import { RadioGroup, RadioGroupItem } from '../radio-group/RadioGroup';

const messages = defineMessages({
  label: { id: 'density.label', defaultMessage: 'Density', description: 'Name of the group of options that choose how compact the interface is.' },
  comfortable: {
    id: 'density.comfortable',
    defaultMessage: 'Comfortable',
    description: 'Density option with more space around controls and table rows.',
  },
  compact: { id: 'density.compact', defaultMessage: 'Compact', description: 'Density option that fits more rows and controls on screen.' },
});

export interface DensitySwitcherProps {
  /** Controlled value. Without it the switcher reads and sets the DensityProvider density. */
  value?: Density | undefined;
  onValueChange?: ((value: Density) => void) | undefined;
  className?: string | undefined;
}

/** Chooses comfortable or compact. Row and control heights follow the density tokens. */
export function DensitySwitcher({ value, onValueChange, className }: DensitySwitcherProps) {
  const intl = useIntl();
  const state = useDensity();
  const current = value ?? state.density;
  return (
    <RadioGroup
      aria-label={intl.formatMessage(messages.label)}
      value={current}
      onValueChange={(next) => {
        const density = next as Density;
        if (onValueChange) onValueChange(density);
        else state.setDensity(density);
      }}
      className={cn('flex-col sm:flex-row', className)}
    >
      <RadioGroupItem value="comfortable" label={intl.formatMessage(messages.comfortable)} />
      <RadioGroupItem value="compact" label={intl.formatMessage(messages.compact)} />
    </RadioGroup>
  );
}

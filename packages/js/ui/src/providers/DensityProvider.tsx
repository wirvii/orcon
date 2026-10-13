// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { readPreference, writePreference } from './storage';

export type Density = 'comfortable' | 'compact';
export const densities: readonly Density[] = ['comfortable', 'compact'];

export const DENSITY_STORAGE_KEY = 'orcon.density';

interface DensityContextValue {
  density: Density;
  setDensity: (density: Density) => void;
}

const DensityContext = createContext<DensityContextValue | null>(null);

function isDensity(value: unknown): value is Density {
  return value === 'comfortable' || value === 'compact';
}

/** Applies the density to <html>. Comfortable is the default and carries no attribute. */
export function applyDensity(density: Density, root: HTMLElement = document.documentElement): void {
  if (density === 'compact') {
    root.setAttribute('data-density', 'compact');
  } else {
    root.removeAttribute('data-density');
  }
}

export interface DensityProviderProps {
  children: ReactNode;
  defaultDensity?: Density | undefined;
  /** Controlled density (Storybook globals, tests). Disables persistence. */
  density?: Density | undefined;
  onDensityChange?: ((density: Density) => void) | undefined;
}

export function DensityProvider({
  children,
  defaultDensity = 'comfortable',
  density: controlled,
  onDensityChange,
}: DensityProviderProps) {
  const [stored, setStored] = useState<Density>(() => {
    const saved = readPreference(DENSITY_STORAGE_KEY);
    return isDensity(saved) ? saved : defaultDensity;
  });

  const density = controlled ?? stored;

  useEffect(() => {
    applyDensity(density);
  }, [density]);

  const setDensity = useCallback(
    (next: Density) => {
      if (controlled === undefined) {
        setStored(next);
        writePreference(DENSITY_STORAGE_KEY, next);
      }
      onDensityChange?.(next);
    },
    [controlled, onDensityChange],
  );

  const value = useMemo(() => ({ density, setDensity }), [density, setDensity]);

  return <DensityContext.Provider value={value}>{children}</DensityContext.Provider>;
}

export function useDensity(): DensityContextValue {
  const value = useContext(DensityContext);
  if (!value) throw new Error('useDensity must be used inside a DensityProvider');
  return value;
}

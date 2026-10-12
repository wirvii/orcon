// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';

// Newer Node versions define their own global localStorage, which shadows jsdom's and is
// not usable without a backing file. Tests get a small in-memory Storage instead.
class MemoryStorage implements Storage {
  private data = new Map<string, string>();
  get length() {
    return this.data.size;
  }
  clear() {
    this.data.clear();
  }
  getItem(key: string) {
    return this.data.get(key) ?? null;
  }
  key(index: number) {
    return [...this.data.keys()][index] ?? null;
  }
  removeItem(key: string) {
    this.data.delete(key);
  }
  setItem(key: string, value: string) {
    this.data.set(key, String(value));
  }
}

const storage = new MemoryStorage();
Object.defineProperty(window, 'localStorage', { value: storage, configurable: true });
Object.defineProperty(globalThis, 'localStorage', { value: storage, configurable: true });

// jsdom does not implement ResizeObserver, which Radix scroll areas use.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver = ResizeObserverStub;

// jsdom does not implement scrolling.
Element.prototype.scrollIntoView = () => undefined;

// jsdom has no matchMedia. Tests that need a dark system theme override this stub.
beforeEach(() => {
  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
  document.documentElement.removeAttribute('lang');
});

// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import '@testing-library/jest-dom/vitest';

import { cleanup } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, beforeEach, vi } from 'vitest';

import { server } from './server';

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
    this.data.set(key, value);
  }
}

const storage = new MemoryStorage();
Object.defineProperty(window, 'localStorage', { value: storage, configurable: true });
Object.defineProperty(globalThis, 'localStorage', { value: storage, configurable: true });

// jsdom does not implement ResizeObserver, which Radix components use.
class ResizeObserverStub {
  observe() {
    // jsdom has no layout, so there is nothing to observe.
  }
  unobserve() {
    // See observe.
  }
  disconnect() {
    // See observe.
  }
}
globalThis.ResizeObserver = ResizeObserverStub;

// jsdom does not implement scrolling or pointer capture, which the router and Radix selects use.
Element.prototype.scrollIntoView = () => undefined;
window.scrollTo = () => undefined;
Element.prototype.hasPointerCapture = () => false;
Element.prototype.releasePointerCapture = () => undefined;

// Every request a test makes must have a handler: an unhandled request fails the test (orcon-web TE-3).
beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' });
});

// jsdom has no matchMedia.
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
  server.resetHandlers();
  vi.unstubAllGlobals();
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
  document.documentElement.removeAttribute('lang');
});

afterAll(() => {
  server.close();
});

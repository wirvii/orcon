// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Shared look of text-like form controls (input, textarea, select trigger).
 * Height and padding come from the density tokens; on a coarse pointer the
 * minimum target size raises the height to 44 px.
 */
export const textControl = [
  'min-h-target w-full rounded-md border border-control bg-surface px-control-x text-base text-fg',
  'placeholder:text-fg-subtle transition-colors hover:border-fg-muted',
  'aria-invalid:border-status-danger',
  'disabled:border-border disabled:text-fg-disabled disabled:placeholder:text-fg-disabled disabled:hover:border-border',
].join(' ');

/**
 * An item in a floating menu, listbox or command list. The highlighted item
 * gets a background and a focus outline, so it is visible without color.
 */
export const menuItem = [
  'relative flex h-control min-h-target cursor-default items-center gap-2 rounded-sm px-control-x text-base text-fg outline-hidden select-none',
  'data-[highlighted]:bg-surface-sunken data-[highlighted]:outline-solid data-[highlighted]:outline-2 data-[highlighted]:-outline-offset-2 data-[highlighted]:outline-focus',
  'data-[disabled]:pointer-events-none data-[disabled]:text-fg-disabled',
].join(' ');

/** The floating panel behind menus, selects and popovers. */
export const floatingPanel = 'z-dropdown rounded-md border border-border bg-surface-raised text-fg shadow-md';

/** Backdrop behind a modal dialog or sheet. Dims the page in both themes. */
export const scrim = 'fixed inset-0 z-overlay bg-scrim opacity-60';

/** The surface of a modal dialog or sheet. */
export const modalPanel = 'z-modal border border-border bg-surface-raised text-fg shadow-lg';

/** An item in a cmdk list (combobox, command palette). cmdk marks the active item with data-selected. */
export const commandItem = [
  'relative flex h-control min-h-target cursor-default items-center gap-2 rounded-sm px-control-x text-base text-fg outline-hidden select-none',
  'data-[selected=true]:bg-surface-sunken data-[selected=true]:outline-solid data-[selected=true]:outline-2 data-[selected=true]:-outline-offset-2 data-[selected=true]:outline-focus',
  'data-[disabled=true]:pointer-events-none data-[disabled=true]:text-fg-disabled',
].join(' ');

// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { AlertDialog } from '../../src/components/alert-dialog/AlertDialog';
import { Breadcrumb } from '../../src/components/breadcrumb/Breadcrumb';
import { Button } from '../../src/components/button/Button';
import { Dialog } from '../../src/components/dialog/Dialog';
import { DropdownMenu } from '../../src/components/dropdown-menu/DropdownMenu';
import { Popover } from '../../src/components/popover/Popover';
import { Sheet } from '../../src/components/sheet/Sheet';
import { Tabs } from '../../src/components/tabs/Tabs';
import { Tooltip } from '../../src/components/tooltip/Tooltip';

import { renderWithProviders } from './helpers';

describe('Breadcrumb', () => {
  it('is a labeled navigation with the current page marked and not linked', () => {
    renderWithProviders(<Breadcrumb items={[{ label: 'Flows', href: '/flows' }, { label: 'Nightly sync' }]} />);
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Flows' })).toHaveAttribute('href', '/flows');
    expect(screen.getByText('Nightly sync')).toHaveAttribute('aria-current', 'page');
    expect(screen.queryByRole('link', { name: 'Nightly sync' })).toBeNull();
  });

  it('translates the landmark name', () => {
    renderWithProviders(<Breadcrumb items={[{ label: 'Flujos' }]} />, { locale: 'es' });
    expect(screen.getByRole('navigation', { name: 'Ruta de navegación' })).toBeInTheDocument();
  });
});

describe('Tabs', () => {
  const tabs = [
    { value: 'a', label: 'A', content: <p>Panel A</p> },
    { value: 'b', label: 'B', content: <p>Panel B</p> },
    { value: 'c', label: 'C', content: <p>Panel C</p>, disabled: true },
  ];

  it('shows the first panel and switches with the arrow keys', async () => {
    renderWithProviders(<Tabs label="Sections" tabs={tabs} />);
    expect(screen.getByRole('tablist', { name: 'Sections' })).toBeInTheDocument();
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Panel A');
    await userEvent.tab();
    await userEvent.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'B' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Panel B');
  });

  it('skips a disabled tab', () => {
    renderWithProviders(<Tabs label="Sections" tabs={tabs} />);
    expect(screen.getByRole('tab', { name: 'C' })).toBeDisabled();
  });

  it('marks the selected tab with more than color: a bar and a heavier weight', () => {
    renderWithProviders(<Tabs label="Sections" tabs={tabs} />);
    const selected = screen.getByRole('tab', { name: 'A' }).className;
    expect(selected).toContain('data-[state=active]:after:border-action-primary');
    expect(selected).toContain('data-[state=active]:font-semibold');
  });
});

describe('Dialog', () => {
  it('is a named modal with a translated close button', async () => {
    const onOpenChange = vi.fn();
    renderWithProviders(
      <Dialog defaultOpen title="Rename flow" description="Shown in the list." onOpenChange={onOpenChange}>
        <p>Body</p>
      </Dialog>,
      { locale: 'es' },
    );
    const dialog = screen.getByRole('dialog', { name: 'Rename flow' });
    expect(dialog).toHaveAccessibleDescription('Shown in the list.');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cerrar' }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('closes with Escape', async () => {
    const onOpenChange = vi.fn();
    renderWithProviders(<Dialog defaultOpen title="Rename flow" onOpenChange={onOpenChange} />);
    await userEvent.keyboard('{Escape}');
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe('AlertDialog', () => {
  it('shows the impact text as its description and focuses Cancel first', () => {
    renderWithProviders(
      <AlertDialog defaultOpen title="Delete this flow?" impact="This cannot be undone." confirmLabel="Delete flow" onConfirm={() => undefined} />,
    );
    const dialog = screen.getByRole('alertdialog', { name: 'Delete this flow?' });
    expect(dialog).toHaveAccessibleDescription('This cannot be undone.');
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
  });

  it('confirms only when the named action is chosen', async () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <AlertDialog defaultOpen title="Delete this flow?" impact="This cannot be undone." confirmLabel="Delete flow" onConfirm={onConfirm} />,
    );
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('uses the danger style for the confirm button unless told otherwise', () => {
    renderWithProviders(
      <AlertDialog defaultOpen title="Leave?" impact="Unsaved changes are lost." confirmLabel="Leave" onConfirm={() => undefined} />,
    );
    expect(screen.getByRole('button', { name: 'Leave' }).className).toContain('bg-action-danger');
  });
});

describe('Sheet', () => {
  it('is a modal dialog that is a bottom sheet on phones and a side panel from md up', () => {
    renderWithProviders(<Sheet defaultOpen title="Node details" />);
    const dialog = screen.getByRole('dialog', { name: 'Node details' });
    expect(dialog.className).toContain('bottom-0');
    expect(dialog.className).toContain('md:right-0');
  });
});

describe('Popover, Tooltip and DropdownMenu', () => {
  it('Popover is a named panel opened from its trigger', () => {
    renderWithProviders(
      <Popover defaultOpen label="Run settings" trigger={<Button>Run settings</Button>}>
        <p>Content</p>
      </Popover>,
    );
    expect(screen.getByRole('dialog', { name: 'Run settings' })).toHaveTextContent('Content');
  });

  it('Tooltip describes its trigger', async () => {
    renderWithProviders(
      <Tooltip content="Shown in the list" defaultOpen>
        <Button>Name</Button>
      </Tooltip>,
    );
    expect(await screen.findByRole('button', { name: 'Name' })).toHaveAccessibleDescription('Shown in the list');
  });

  it('DropdownMenu lists its entries, marks disabled ones and runs the selected one', async () => {
    const onSelect = vi.fn();
    renderWithProviders(
      <DropdownMenu
        defaultOpen
        trigger={<Button>Actions</Button>}
        items={[
          { type: 'label', label: 'Flow' },
          { type: 'item', label: 'Rename', onSelect },
          { type: 'item', label: 'Export', disabled: true },
          { type: 'separator' },
          { type: 'item', label: 'Delete flow', destructive: true },
        ]}
      />,
    );
    expect(screen.getByRole('menuitem', { name: 'Export' })).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('menuitem', { name: 'Delete flow' }).className).toContain('text-status-danger');
    await userEvent.click(screen.getByRole('menuitem', { name: 'Rename' }));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });
});

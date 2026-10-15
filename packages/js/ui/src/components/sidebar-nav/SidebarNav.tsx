// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ExternalLink, PanelLeft, type LucideIcon } from 'lucide-react';
import { createContext, useContext, useEffect, useId, type ButtonHTMLAttributes, type MouseEvent, type ReactNode, type Ref } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { BrandMark } from '../brand-mark/BrandMark';
import { Tooltip } from '../tooltip/Tooltip';

const messages = defineMessages({
  collapse: {
    id: 'sidebar.collapse',
    defaultMessage: 'Collapse the sidebar',
    description: 'Accessible name of the button that shrinks the sidebar to icons. The key [ does the same.',
  },
  expand: {
    id: 'sidebar.expand',
    defaultMessage: 'Expand the sidebar',
    description: 'Accessible name of the button that widens the collapsed sidebar to show names. The key [ does the same.',
  },
  opensInNewTab: {
    id: 'common.link.opensInNewTab',
    defaultMessage: '(opens in a new tab)',
    description: 'Screen reader text appended to a link that opens in a new browser tab.',
  },
});

const externalProps = { target: '_blank', rel: 'noopener noreferrer' };

interface SidebarState {
  collapsed: boolean;
}

const SidebarContext = createContext<SidebarState>({ collapsed: false });

/** Whether the sidebar is collapsed to icons. A control inside the sidebar uses it to hide its text. */
export function useSidebarCollapsed(): boolean {
  return useContext(SidebarContext).collapsed;
}

export interface SidebarItem {
  id: string;
  /** The destination name. Always present: when the sidebar is collapsed it is hidden text plus a tooltip. */
  label: string;
  href: string;
  icon?: LucideIcon | undefined;
  /** Something custom before the label instead of an icon, such as the letter tile of a project. */
  leading?: ReactNode | undefined;
  /** How many things are in or waiting in that destination. Left out when it is not known. */
  count?: number | undefined;
  /** The count is something a person has to look at: it is drawn in the attention color and read after the label. */
  attention?: boolean | undefined;
  /** The link leaves the single-page app (the server answers it with its own page). */
  external?: boolean | undefined;
}

export interface SidebarSection {
  id: string;
  /** The small heading of the group. Hidden, but still read, when the sidebar is collapsed. */
  label?: string | undefined;
  items: readonly SidebarItem[];
  /** A control at the end of the heading, such as a button that adds an item to the group. */
  action?: ReactNode | undefined;
}

const itemClass = [
  'text-fg-muted hover:bg-surface-hover hover:text-fg relative flex h-8 min-h-target w-full items-center gap-2 rounded-md px-2 text-left text-sm font-medium transition-colors',
].join(' ');

/** The marker of the current section: a bar with the brand gradient on the left edge, plus a heavier weight. */
const currentClass =
  'bg-surface-hover text-fg font-semibold before:bg-brand-gradient before:absolute before:top-2 before:bottom-2 before:-left-2 before:w-1 before:rounded-r-sm';

interface ItemContentProps {
  label: string;
  icon?: LucideIcon | undefined;
  leading?: ReactNode | undefined;
  count?: number | undefined;
  attention?: boolean | undefined;
  external?: boolean | undefined;
  collapsed: boolean;
}

function ItemContent({ label, icon: Icon, leading, count, attention, external, collapsed }: ItemContentProps) {
  const intl = useIntl();
  return (
    <>
      {leading ?? (Icon ? <Icon aria-hidden className="size-icon-16 shrink-0" /> : null)}
      <span data-truncate className={cn('min-w-0 flex-1 truncate', collapsed && 'sr-only')}>
        {label}
      </span>
      {count === undefined ? null : (
        <span className={cn('text-2xs shrink-0', attention ? 'text-status-attention font-bold' : 'text-fg-muted font-medium', collapsed && 'sr-only')}>{count}</span>
      )}
      {external ? (
        <>
          <ExternalLink aria-hidden className={cn('text-fg-subtle size-icon-16 shrink-0', collapsed && 'sr-only')} />
          <span className="sr-only">{intl.formatMessage(messages.opensInNewTab)}</span>
        </>
      ) : null}
    </>
  );
}

export interface SidebarLinkProps {
  item: SidebarItem;
  current?: boolean | undefined;
  /** Called instead of following the link on a plain click, so a router can change the address without reloading. */
  onNavigate?: ((item: SidebarItem) => void) | undefined;
}

/** One destination of the sidebar: an icon, a name and, when known, a count. The current one is marked. */
export function SidebarLink({ item, current = false, onNavigate }: SidebarLinkProps) {
  const collapsed = useSidebarCollapsed();
  const follow = (event: MouseEvent<HTMLAnchorElement>) => {
    if (!onNavigate || item.external || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    onNavigate(item);
  };
  const link = (
    <a
      href={item.href}
      aria-current={current ? 'page' : undefined}
      onClick={follow}
      {...(item.external ? externalProps : {})}
      className={cn(itemClass, collapsed && 'justify-center px-0', current && currentClass)}
    >
      <ItemContent {...item} collapsed={collapsed} />
    </a>
  );
  return collapsed ? (
    <Tooltip content={item.label} side="right">
      {link}
    </Tooltip>
  ) : (
    link
  );
}

export interface SidebarButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  ref?: Ref<HTMLButtonElement> | undefined;
  label: string;
  icon: LucideIcon;
}

/** A button that looks like a destination of the sidebar, such as the one that opens the Help menu. */
export function SidebarButton({ label, icon: Icon, className, ref, type = 'button', ...props }: SidebarButtonProps) {
  const collapsed = useSidebarCollapsed();
  const button = (
    <button ref={ref} type={type} className={cn(itemClass, collapsed && 'justify-center px-0', className)} {...props}>
      <Icon aria-hidden className="size-icon-16 shrink-0" />
      <span data-truncate className={cn('min-w-0 flex-1 truncate text-left', collapsed && 'sr-only')}>
        {label}
      </span>
    </button>
  );
  return collapsed ? (
    <Tooltip content={label} side="right">
      {button}
    </Tooltip>
  ) : (
    button
  );
}

export interface SidebarNavProps {
  /** Names the navigation landmark. */
  label: string;
  /** The groups of destinations, in order. */
  sections: readonly SidebarSection[];
  /** The `id` of the destination being shown: only that one is marked. */
  currentId?: string | undefined;
  /** At the top, under the logo: the organization switcher. */
  header?: ReactNode | undefined;
  /** Under the header: the main action of the app. */
  primary?: ReactNode | undefined;
  /** At the bottom: help, settings and the account. */
  footer?: ReactNode | undefined;
  /** Shrunk to icons. */
  collapsed: boolean;
  onCollapsedChange: (collapsed: boolean) => void;
  onNavigate?: ((item: SidebarItem) => void) | undefined;
  className?: string | undefined;
}

/** True when a key press is going into something a person types in, so a shortcut must not take it. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

/**
 * The sidebar of the desktop app (240 px wide, 64 px when collapsed): the logo, the organization, the main
 * action, the groups of destinations and, at the bottom, help, settings and the account. The key `[` and the
 * button at the top collapse it to icons; every destination keeps its name for assistive technology and gets a
 * tooltip. Only the destination that owns the current address is marked.
 */
export function SidebarNav({ label, sections, currentId, header, primary, footer, collapsed, onCollapsedChange, onNavigate, className }: SidebarNavProps) {
  const intl = useIntl();
  const regionId = useId();

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== '[' || event.metaKey || event.ctrlKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return;
      event.preventDefault();
      onCollapsedChange(!collapsed);
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [collapsed, onCollapsedChange]);

  const toggleLabel = intl.formatMessage(collapsed ? messages.expand : messages.collapse);
  return (
    <SidebarContext.Provider value={{ collapsed }}>
      <nav
        id={regionId}
        aria-label={label}
        data-collapsed={collapsed || undefined}
        className={cn(
          'bg-surface border-border flex h-full min-h-0 shrink-0 flex-col gap-1 border-r px-2 py-3 transition-all',
          collapsed ? 'w-16' : 'w-panel-width',
          className,
        )}
      >
        <div className={cn('flex items-center justify-between gap-2 px-1', collapsed && 'flex-col')}>
          <BrandMark variant={collapsed ? 'symbol' : 'lockup'} />
          <Tooltip content={toggleLabel} side="right">
            <button
              type="button"
              aria-label={toggleLabel}
              aria-expanded={!collapsed}
              aria-controls={regionId}
              onClick={() => {
                onCollapsedChange(!collapsed);
              }}
              className="text-fg-muted hover:bg-surface-hover hover:text-fg inline-flex size-8 min-h-target min-w-target items-center justify-center rounded-md transition-colors"
            >
              <PanelLeft aria-hidden className="size-icon-16" />
            </button>
          </Tooltip>
        </div>
        {header ? <div className="mt-2">{header}</div> : null}
        {primary ? <div className="mt-1 mb-2">{primary}</div> : null}
        <div className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto">
          {sections.map((section) => (
            <div key={section.id} className="flex flex-col gap-1">
              {section.label ? (
                <div className={cn('flex items-center justify-between px-2 pt-3 pb-1', collapsed && 'hidden')}>
                  {/* The group is named by the list below; the visible heading is not read twice. */}
                  <p aria-hidden className="text-fg-subtle text-2xs font-semibold uppercase">
                    {section.label}
                  </p>
                  {section.action}
                </div>
              ) : null}
              <ul aria-label={section.label} className="flex flex-col gap-1">
                {section.items.map((item) => (
                  <li key={item.id}>
                    <SidebarLink item={item} current={item.id === currentId} onNavigate={onNavigate} />
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        {footer ? <div className="flex flex-col gap-1">{footer}</div> : null}
      </nav>
    </SidebarContext.Provider>
  );
}

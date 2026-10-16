// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AppShell,
  BottomTabBar,
  BrandMark,
  Button,
  DropdownMenu,
  IconButton,
  Kbd,
  PhoneTopBar,
  SidebarButton,
  SidebarLink,
  SidebarNav,
  useSidebarCollapsed,
  type BottomTab,
  type MenuEntry,
  type SidebarItem,
  type SidebarSection,
} from '@orcon/ui';
import { linkOptions, useMatches, useNavigate, useParams, useRouter, useRouterState } from '@tanstack/react-router';
import { Activity, BookOpen, Braces, CircleHelp, FolderKanban, House, KeyRound, Keyboard, LayoutTemplate, LifeBuoy, Plus, Settings, Upload } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ConnectionBanners } from '../../../app/shell/ConnectionBanners';
import type { ShellProject } from '../../../app/shell/shell-data';
import { frameSectionOf, projectEntryId, sidebarIdOf, tabIdOf, type FrameSection } from '../model/frame-section';
import { FrameOrgSwitcher, type FrameOrganization } from './OrgSwitcher';
import { ShortcutsDialog } from './ShortcutsDialog';
import { FrameUserMenu, type FrameUser } from './UserMenu';

const messages = defineMessages({
  main: { id: 'session.frame.main', defaultMessage: 'Main navigation', description: 'Accessible name of the navigation landmark with the sections of the app.' },
  home: { id: 'session.frame.home', defaultMessage: 'Home', description: 'Navigation entry for the home page of the organization.' },
  projects: { id: 'session.frame.projects', defaultMessage: 'Projects', description: 'Heading of the group of projects in the sidebar.' },
  resources: { id: 'session.frame.resources', defaultMessage: 'Resources', description: 'Heading of the group of organization-wide pages (templates, credentials, executions) in the sidebar.' },
  templates: { id: 'session.frame.templates', defaultMessage: 'Templates', description: 'Navigation entry for the ready-made flows.' },
  credentials: { id: 'session.frame.credentials', defaultMessage: 'Credentials', description: 'Navigation entry for the credentials of the organization.' },
  executions: { id: 'session.frame.executions', defaultMessage: 'Executions', description: 'Navigation entry for the runs of flows.' },
  settings: { id: 'session.frame.settings', defaultMessage: 'Settings', description: 'Navigation entry for the settings of the account and the organization.' },
  help: { id: 'session.frame.help', defaultMessage: 'Help', description: 'Navigation entry that opens the help menu.' },
  createFlow: { id: 'session.frame.createFlow', defaultMessage: 'Create flow', description: 'Button at the top of the sidebar that starts a new flow.' },
  createProject: { id: 'session.frame.createProject', defaultMessage: 'Create a project', description: 'Accessible name of the button next to the Projects heading that opens the new project dialog.' },
  flows: { id: 'session.frame.flows', defaultMessage: 'Flows', description: 'Tab of the phone bar that opens the projects and their flows.' },
  documentation: { id: 'session.frame.help.documentation', defaultMessage: 'Documentation', description: 'Help menu entry that opens the documentation.' },
  apiReference: {
    id: 'session.frame.help.apiReference',
    defaultMessage: 'API reference',
    description: 'Help menu entry that opens the API reference, a page the server serves outside the app.',
  },
  community: { id: 'session.frame.help.community', defaultMessage: 'Community and support', description: 'Help menu entry that opens the place to ask questions and report problems.' },
  fromN8n: { id: 'session.frame.help.fromN8n', defaultMessage: 'Guide for coming from n8n', description: 'Help menu entry that opens the guide for people who used n8n before.' },
  shortcuts: { id: 'session.frame.help.shortcuts', defaultMessage: 'Keyboard shortcuts', description: 'Help menu entry that opens the list of keyboard shortcuts.' },
});

/** Where the Help menu leads. The documentation lives in the open repository. */
const DOCS = 'https://github.com/wirvii/orcon/tree/main/docs/user/en';
const SUPPORT = 'https://github.com/wirvii/orcon/tree/main/docs/user/en/support';
const FROM_N8N = 'https://github.com/wirvii/orcon/tree/main/docs/user/en/migrating-from-n8n';

/** The tiles of the projects take the calm colors in turn; red, amber and violet are for status only. */
const tileTones = ['bg-status-success text-fg-inverse', 'bg-status-active text-fg-inverse', 'bg-fg-muted text-fg-inverse'] as const;

function ProjectTile({ name, index }: { name: string; index: number }) {
  return (
    <span aria-hidden className={`${tileTones[index % tileTones.length] ?? ''} inline-flex size-icon-20 shrink-0 items-center justify-center rounded-base text-2xs font-bold`}>
      {(name.trim().charAt(0) || '?').toUpperCase()}
    </span>
  );
}

/** True when a key press is going into something a person types in. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

export interface AppFrameProps {
  children: ReactNode;
  /** The organization of the address, when the page is inside one. */
  orgSlug?: string | undefined;
  /** The organizations the person belongs to. */
  organizations: readonly FrameOrganization[];
  /** The projects the person can read in the organization; `undefined` while they are being read. */
  projects: readonly ShellProject[] | undefined;
  /** The signed-in person; `undefined` while unknown. */
  user: FrameUser | undefined;
  /** Executions that need review, shown next to Executions. `undefined` when the server cannot say. */
  needsReviewCount?: number | undefined;
  /** What the person chose for the sidebar: collapsed, open, or `undefined` if they never chose. */
  collapsedChoice: boolean | undefined;
  onCollapsedChange: (collapsed: boolean) => void;
  onSignOut: () => void;
  /** Another organization was chosen in the switcher. */
  onSwitchOrganization: (slug: string) => void;
  /** The list of organizations was asked for from a page outside an organization. */
  onOpenOrganizations: () => void;
}

/**
 * The frame of the signed-in pages (SPEC-019 R-UX-SHL): inside an organization, the sidebar with the organization,
 * the Create flow button, Home, the projects the person can read, the organization-wide pages, Help, Settings and
 * the account; on a phone the top bar and the five-destination bar instead. Pages outside an organization (the
 * picker, the explanation for an account without one) get a plain bar with the account.
 */
export function AppFrame(props: AppFrameProps) {
  const { orgSlug } = props;
  return orgSlug === undefined ? <PlainFrame {...props} /> : <OrganizationFrame {...props} orgSlug={orgSlug} />;
}

function PlainFrame({ children, user, organizations, onSignOut, onOpenOrganizations }: AppFrameProps) {
  return (
    <AppShell
      brand={<BrandMark />}
      actions={
        user === undefined ? null : (
          <FrameUserMenu user={user} compact side="bottom" onSignOut={onSignOut} onSwitchOrganization={organizations.length > 1 ? onOpenOrganizations : undefined} />
        )
      }
    >
      <div className="flex flex-col gap-3">
        <ConnectionBanners />
        {children}
      </div>
    </AppShell>
  );
}

function OrganizationFrame({
  orgSlug,
  children,
  organizations,
  projects,
  user,
  needsReviewCount,
  collapsedChoice,
  onCollapsedChange,
  onSignOut,
  onSwitchOrganization,
}: AppFrameProps & { orgSlug: string }) {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const params = useParams({ strict: false });
  const organization = organizations.find((candidate) => candidate.slug === orgSlug);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  // A page that fills the space between the bars (the flow editor) declares it in its route's staticData.
  const fill = useMatches({ select: (matches) => matches.some((match) => (match.staticData as { fill?: boolean }).fill === true) });

  // Collapsed by default inside the editor, open everywhere else, until the person chooses.
  const collapsed = collapsedChoice ?? fill;

  const section: FrameSection = frameSectionOf(pathname, orgSlug);
  // `linkOptions` checks each destination against the route tree where it is written; this only builds its address.
  const hrefOf = (options: ReturnType<typeof linkOptions>) => router.buildLocation(options as never).href;
  const items: readonly ShellProject[] = projects ?? [];
  const currentProject = params.projectHandle === undefined ? undefined : items.find((project) => project.handle === params.projectHandle);

  /** Creating a flow starts in the project being looked at, else the first one; without any it starts a project. */
  const createFlow = () => {
    // The projects are still being read: the key is ignored rather than sending the person to start a project.
    if (projects === undefined) return;
    const target = currentProject ?? items[0];
    if (target === undefined) {
      void navigate({ to: '/orgs/$orgSlug/projects', params: { orgSlug }, search: { create: true } });
      return;
    }
    void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle: target.handle }, search: { create: true } });
  };

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return;
      // A dialog or a menu that is open owns the keyboard.
      if (document.querySelector('[role="dialog"], [role="menu"]') !== null) return;
      if (event.key === 'n' || event.key === 'N') {
        event.preventDefault();
        createFlow();
      } else if (event.key === '?') {
        event.preventDefault();
        setShortcutsOpen(true);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
    };
  });

  const sections: SidebarSection[] = [
    { id: 'start', items: [{ id: 'home', label: intl.formatMessage(messages.home), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/overview', params: { orgSlug } })), icon: House }] },
    {
      id: 'projects',
      label: intl.formatMessage(messages.projects),
      action: (
        <IconButton
          variant="ghost"
          icon={Plus}
          label={intl.formatMessage(messages.createProject)}
          onClick={() => void navigate({ to: '/orgs/$orgSlug/projects', params: { orgSlug }, search: { create: true } })}
          className="size-6"
        />
      ),
      items: items.map(
        (project, index): SidebarItem => ({
          id: projectEntryId(project.handle),
          label: project.name,
          href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle: project.handle } })),
          leading: <ProjectTile name={project.name} index={index} />,
          // The number of flows is shown when the server says it (SPEC-019 UXB1); until then nothing stands in for it.
          count: project.flowCount,
        }),
      ),
    },
    {
      id: 'resources',
      label: intl.formatMessage(messages.resources),
      items: [
        { id: 'templates', label: intl.formatMessage(messages.templates), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/templates', params: { orgSlug } })), icon: LayoutTemplate },
        { id: 'credentials', label: intl.formatMessage(messages.credentials), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/credentials', params: { orgSlug } })), icon: KeyRound },
        {
          id: 'executions',
          label: intl.formatMessage(messages.executions),
          href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/executions', params: { orgSlug } })),
          icon: Activity,
          // Shown only when there is something to review: a zero or an unknown number draws nothing.
          count: needsReviewCount !== undefined && needsReviewCount > 0 ? needsReviewCount : undefined,
          attention: true,
        },
      ],
    },
  ];

  const settingsItem: SidebarItem = {
    id: 'settings',
    label: intl.formatMessage(messages.settings),
    href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/settings/profile', params: { orgSlug } })),
    icon: Settings,
  };

  const goTo = (item: { href: string }) => void navigate({ href: item.href });

  const tabs: BottomTab[] = [
    { id: 'home', label: intl.formatMessage(messages.home), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/overview', params: { orgSlug } })), icon: House },
    { id: 'flows', label: intl.formatMessage(messages.flows), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/projects', params: { orgSlug } })), icon: FolderKanban },
    { id: 'executions', label: intl.formatMessage(messages.executions), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/executions', params: { orgSlug } })), icon: Activity },
    { id: 'credentials', label: intl.formatMessage(messages.credentials), href: hrefOf(linkOptions({ to: '/orgs/$orgSlug/credentials', params: { orgSlug } })), icon: KeyRound },
    { id: 'settings', label: intl.formatMessage(messages.settings), href: settingsItem.href, icon: Settings },
  ];

  const sectionTitle = (() => {
    switch (section.kind) {
      case 'home':
        return intl.formatMessage(messages.home);
      case 'projects':
        return intl.formatMessage(messages.flows);
      case 'project':
        return items.find((project) => project.handle === section.handle)?.name ?? intl.formatMessage(messages.flows);
      case 'templates':
        return intl.formatMessage(messages.templates);
      case 'credentials':
        return intl.formatMessage(messages.credentials);
      case 'executions':
        return intl.formatMessage(messages.executions);
      case 'settings':
        return intl.formatMessage(messages.settings);
      default:
        return organization?.name ?? '';
    }
  })();

  const frameOrganization = organization === undefined ? undefined : { slug: organization.slug, name: organization.name };
  const account = (compact: boolean, side: 'top' | 'bottom') =>
    user === undefined ? null : (
      <FrameUserMenu user={user} organization={frameOrganization} compact={compact} side={side} onSignOut={onSignOut} onOpenShortcuts={() => setShortcutsOpen(true)} />
    );

  return (
    <>
      <AppShell
        fill={fill}
        sidebar={
          <SidebarNav
            label={intl.formatMessage(messages.main)}
            sections={sections}
            currentId={sidebarIdOf(section)}
            header={<FrameOrgSwitcher organizations={organizations} activeSlug={orgSlug} onSelect={onSwitchOrganization} />}
            primary={<CreateFlowButton onClick={createFlow} />}
            footer={
              <>
                <HelpMenu onOpenShortcuts={() => setShortcutsOpen(true)} />
                <SidebarLink item={settingsItem} current={section.kind === 'settings'} onNavigate={goTo} />
                <div className="border-border mt-1 border-t pt-2">
                  {account(collapsed, 'top')}
                </div>
              </>
            }
            collapsed={collapsed}
            onCollapsedChange={onCollapsedChange}
            onNavigate={goTo}
          />
        }
        phoneBar={
          <PhoneTopBar
            context={organization?.name ?? ''}
            title={sectionTitle}
            actions={account(true, 'bottom')}
          />
        }
        tabBar={<BottomTabBar label={intl.formatMessage(messages.main)} tabs={tabs} currentId={tabIdOf(section)} onNavigate={goTo} />}
      >
        <div className={fill ? 'flex min-h-0 flex-1 flex-col gap-3' : 'flex flex-col gap-6'}>
          <ConnectionBanners />
          {children}
        </div>
      </AppShell>
      <ShortcutsDialog open={shortcutsOpen} onOpenChange={setShortcutsOpen} />
    </>
  );
}

/** The green button at the top of the sidebar. With the sidebar collapsed it is only the plus, still named. */
function CreateFlowButton({ onClick }: { onClick: () => void }) {
  const intl = useIntl();
  const collapsed = useSidebarCollapsed();
  const label = intl.formatMessage(messages.createFlow);
  return (
    <Button onClick={onClick} className="w-full justify-start" aria-label={collapsed ? label : undefined}>
      <Plus aria-hidden className="size-icon-16 shrink-0" />
      {collapsed ? null : (
        <>
          <span className="flex-1 text-left">{label}</span>
          <Kbd aria-hidden className="bg-transparent text-action-primary-fg border-action-primary-fg">
            N
          </Kbd>
        </>
      )}
    </Button>
  );
}

/** The help menu: the documentation, where to ask, the guide for people who come from n8n and the shortcuts. */
function HelpMenu({ onOpenShortcuts }: { onOpenShortcuts: () => void }) {
  const intl = useIntl();
  const entries: MenuEntry[] = [
    { type: 'link', label: intl.formatMessage(messages.documentation), href: DOCS, external: true, icon: BookOpen },
    // The API reference is a page the server answers, not a route of the app: a plain link that loads it in this tab.
    { type: 'link', label: intl.formatMessage(messages.apiReference), href: '/api/reference/', icon: Braces },
    { type: 'link', label: intl.formatMessage(messages.community), href: SUPPORT, external: true, icon: LifeBuoy },
    { type: 'link', label: intl.formatMessage(messages.fromN8n), href: FROM_N8N, external: true, icon: Upload },
    { type: 'item', label: intl.formatMessage(messages.shortcuts), icon: Keyboard, shortcut: '?', onSelect: onOpenShortcuts },
  ];
  return <DropdownMenu align="start" side="top" width="panel" items={entries} trigger={<SidebarButton label={intl.formatMessage(messages.help)} icon={CircleHelp} />} />;
}

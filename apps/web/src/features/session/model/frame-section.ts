// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** The part of the product an address belongs to, as the frame (sidebar and tab bar) shows it. */
export type FrameSection =
  | { kind: 'home' }
  /** The list of all projects. It has no entry of its own in the sidebar. */
  | { kind: 'projects' }
  | { kind: 'project'; handle: string }
  | { kind: 'templates' }
  | { kind: 'credentials' }
  | { kind: 'executions' }
  | { kind: 'settings' }
  | { kind: 'other' };

/** The five destinations of the tab bar of a phone. */
export type TabId = 'home' | 'flows' | 'executions' | 'credentials' | 'settings';

/**
 * The section of the organization's address space that owns a path. Only the entry that owns the path is marked
 * (SPEC-019 R-UX-SHL-06): a project's own pages (flows, credentials, executions, the editor) belong to that project.
 */
export function frameSectionOf(pathname: string, orgSlug: string): FrameSection {
  const base = `/orgs/${orgSlug}`;
  if (pathname !== base && !pathname.startsWith(`${base}/`)) return { kind: 'other' };
  const [first, second] = pathname.slice(base.length).split('/').filter(Boolean);
  switch (first) {
    case 'overview':
      return { kind: 'home' };
    case 'projects':
      return second === undefined ? { kind: 'projects' } : { kind: 'project', handle: decodeURIComponent(second) };
    case 'templates':
      return { kind: 'templates' };
    case 'credentials':
      return { kind: 'credentials' };
    case 'executions':
      return { kind: 'executions' };
    case 'settings':
      return { kind: 'settings' };
    default:
      return { kind: 'other' };
  }
}

/** The `id` of the sidebar entry a section marks, if it has one. */
export function sidebarIdOf(section: FrameSection): string | undefined {
  switch (section.kind) {
    case 'home':
    case 'templates':
    case 'credentials':
    case 'executions':
    case 'settings':
      return section.kind;
    case 'project':
      return projectEntryId(section.handle);
    default:
      return undefined;
  }
}

/** The tab of the phone bar a section marks, if it has one. */
export function tabIdOf(section: FrameSection): TabId | undefined {
  switch (section.kind) {
    case 'home':
      return 'home';
    case 'projects':
    case 'project':
      return 'flows';
    case 'executions':
    case 'credentials':
    case 'settings':
      return section.kind;
    default:
      return undefined;
  }
}

/** The id of the sidebar entry of a project. */
export function projectEntryId(handle: string): string {
  return `project:${handle}`;
}

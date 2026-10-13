// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The parity table of the canvas reference (`canvas.md#parity`): every interaction of the editor,
 * with the most steps it may take. A step is a commit action (a click, a drag, a key chord); moving
 * the pointer or the focus onto the target, and typing the content of a field, are not steps.
 * A unit test keeps this list equal to the table in the skill.
 */
export interface ParityRow {
  id: string;
  title: string;
  maxSteps: number;
  /** The specification that owns the row. */
  owner: 'SPEC-007' | 'SPEC-014' | 'SPEC-007 visuals; SPEC-014 behavior' | 'SPEC-007 visuals; SPEC-014 storage' | 'SPEC-007 visuals; SPEC-014 store' | 'SPEC-007 visuals; SPEC-014 reference rewrite' | 'SPEC-007 panel; SPEC-014 index' | 'SPEC-007 (rules SPEC-014)';
}

export const parityRows: readonly ParityRow[] = [
  { id: 'P-01', title: 'First step on an empty canvas', maxSteps: 2, owner: 'SPEC-007' },
  { id: 'P-02', title: 'Add a node after a node', maxSteps: 2, owner: 'SPEC-007' },
  { id: 'P-03', title: 'Add after a dangling output', maxSteps: 2, owner: 'SPEC-007' },
  { id: 'P-04', title: 'Insert a node on a connection', maxSteps: 2, owner: 'SPEC-007' },
  { id: 'P-05', title: 'Delete a connection', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-06', title: 'Connect two nodes', maxSteps: 1, owner: 'SPEC-007 (rules SPEC-014)' },
  { id: 'P-07', title: 'Move and multi-select', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-08', title: 'Node quick actions', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-09', title: 'Select and open a node', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-10', title: 'Node details view', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-11', title: 'Switch data view', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-12', title: 'Map an input field into a parameter', maxSteps: 1, owner: 'SPEC-014' },
  { id: 'P-13', title: 'Execute one step', maxSteps: 1, owner: 'SPEC-007 visuals; SPEC-014 behavior' },
  { id: 'P-14', title: 'Execute the workflow', maxSteps: 1, owner: 'SPEC-007 visuals; SPEC-014 behavior' },
  { id: 'P-15', title: 'Pin output data', maxSteps: 1, owner: 'SPEC-007 visuals; SPEC-014 storage' },
  { id: 'P-16', title: 'Disable a node', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-17', title: 'Read a failed node', maxSteps: 0, owner: 'SPEC-007' },
  { id: 'P-18', title: 'Read an uncertain node (Orcon improvement)', maxSteps: 0, owner: 'SPEC-007' },
  { id: 'P-19', title: 'Sticky notes', maxSteps: 1, owner: 'SPEC-007 visuals; SPEC-014 store' },
  { id: 'P-20', title: 'Zoom', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-21', title: 'Minimap', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-22', title: 'Pan', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-23', title: 'Copy, paste, duplicate', maxSteps: 1, owner: 'SPEC-014' },
  { id: 'P-24', title: 'Undo and redo', maxSteps: 1, owner: 'SPEC-014' },
  { id: 'P-25', title: 'Rename a node', maxSteps: 2, owner: 'SPEC-007 visuals; SPEC-014 reference rewrite' },
  { id: 'P-26', title: 'Tidy up', maxSteps: 1, owner: 'SPEC-014' },
  { id: 'P-27', title: 'Find a node type', maxSteps: 2, owner: 'SPEC-007 panel; SPEC-014 index' },
  { id: 'P-28', title: 'Multi-output node', maxSteps: 0, owner: 'SPEC-007' },
  { id: 'P-29', title: 'Trigger node', maxSteps: 1, owner: 'SPEC-007' },
  { id: 'P-30', title: 'Same experience in dark theme and by keyboard', maxSteps: 1, owner: 'SPEC-007' },
];

/** The rows this repository delivers: the ones whose owner starts with SPEC-007. */
export const ownedRows = parityRows.filter((row) => row.owner.startsWith('SPEC-007'));

/** `P-07` -> `P07`, the name of the story export. */
export const storyExport = (id: string) => id.replace('-', '');

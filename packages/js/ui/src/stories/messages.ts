// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

/**
 * Sample content for the overlay stories (dialog, alert dialog, menu, palette,
 * toast). It lives in the catalogs so the pseudo-locale lengthens it and the
 * overlays are checked with long text. Story-only: not exported from the package.
 */
export const storyMessages = defineMessages({
  renameTitle: { id: 'story.rename.title', defaultMessage: 'Rename flow', description: 'Sample dialog title and its trigger button in the dialog stories.' },
  renameDescription: {
    id: 'story.rename.description',
    defaultMessage: 'The new name appears in the flow list and in notifications.',
    description: 'Sample dialog description in the dialog stories.',
  },
  renameField: { id: 'story.rename.field', defaultMessage: 'Flow name', description: 'Sample field label in the dialog stories.' },
  renameValue: { id: 'story.rename.value', defaultMessage: 'Nightly sync', description: 'Sample flow name typed into the field in the dialog stories.' },
  cancel: { id: 'story.action.cancel', defaultMessage: 'Cancel', description: 'Sample secondary button in the dialog footer story.' },
  save: { id: 'story.action.save', defaultMessage: 'Save', description: 'Sample primary button in the dialog footer story.' },

  deleteTitle: { id: 'story.delete.title', defaultMessage: 'Delete this flow?', description: 'Sample alert dialog title in the alert dialog stories.' },
  deleteImpact: {
    id: 'story.delete.impact',
    defaultMessage: 'The flow and its schedule are removed. Past executions stay in the history. This cannot be undone.',
    description: 'Sample alert dialog text that says what deleting the flow does.',
  },
  deleteShort: { id: 'story.delete.short', defaultMessage: 'This cannot be undone.', description: 'Sample short alert dialog text in the alert dialog matrix.' },
  deleteFlow: { id: 'story.delete.action', defaultMessage: 'Delete flow', description: 'Sample destructive button and its trigger in the alert dialog and menu stories.' },

  menuLabel: { id: 'story.menu.label', defaultMessage: 'Flow', description: 'Sample group label at the top of the menu in the menu stories.' },
  menuRename: { id: 'story.menu.rename', defaultMessage: 'Rename', description: 'Sample menu item in the menu stories.' },
  menuDuplicate: { id: 'story.menu.duplicate', defaultMessage: 'Duplicate', description: 'Sample menu item in the menu stories.' },
  menuExport: { id: 'story.menu.export', defaultMessage: 'Export', description: 'Sample disabled menu item in the menu stories.' },
  menuTrigger: { id: 'story.menu.trigger', defaultMessage: 'Flow actions', description: 'Accessible name of the sample menu button in the menu stories.' },
  menuClosed: { id: 'story.menu.closed', defaultMessage: 'Actions, closed', description: 'Accessible name of the sample menu button in the closed menu matrix cell.' },
  menuOpen: { id: 'story.menu.open', defaultMessage: 'Actions, open', description: 'Accessible name of the sample menu button in the open menu matrix cell.' },

  paletteHint: {
    id: 'story.palette.hint',
    defaultMessage: 'Press {first} + {second} to open the palette.',
    description: 'Hint above the sample command palette. The placeholders are the two keys of the shortcut.',
  },
  paletteCreate: { id: 'story.palette.create', defaultMessage: 'Create a flow', description: 'Sample palette command in the command palette stories.' },
  paletteOpenFlow: { id: 'story.palette.openFlow', defaultMessage: 'Open a flow', description: 'Sample palette command in the command palette stories.' },
  paletteExecutions: { id: 'story.palette.executions', defaultMessage: 'Go to executions', description: 'Sample palette command in the command palette stories.' },
  paletteInbox: { id: 'story.palette.inbox', defaultMessage: 'Go to the attention inbox', description: 'Sample palette command in the command palette stories.' },
  paletteTheme: { id: 'story.palette.theme', defaultMessage: 'Change theme', description: 'Sample palette command without a group in the command palette stories.' },
  paletteGroupFlows: { id: 'story.palette.groupFlows', defaultMessage: 'Flows', description: 'Sample palette group name in the command palette stories.' },
  paletteGroupNavigate: { id: 'story.palette.groupNavigate', defaultMessage: 'Navigate', description: 'Sample palette group name in the command palette stories.' },

  toastNote: {
    id: 'story.toast.note',
    defaultMessage: 'Toasts appear in the bottom corner and never take focus.',
    description: 'Note under the toast stories.',
  },
  toastMatrix: {
    id: 'story.toast.matrix',
    defaultMessage: 'Toast states (all five shown together in the corner)',
    description: 'Caption of the toast matrix story.',
  },
  toastSaved: { id: 'story.toast.saved', defaultMessage: 'Flow saved', description: 'Sample success toast.' },
  toastWarning: { id: 'story.toast.warning', defaultMessage: 'Saved with a warning', description: 'Sample warning toast title.' },
  toastWarningText: { id: 'story.toast.warningText', defaultMessage: 'One node has no output.', description: 'Sample warning toast description.' },
  toastDanger: { id: 'story.toast.danger', defaultMessage: 'Could not copy the link', description: 'Sample error toast.' },
  toastAction: { id: 'story.toast.action', defaultMessage: 'Flow deleted', description: 'Sample toast that offers to undo.' },
  toastUndo: { id: 'story.toast.undo', defaultMessage: 'Undo', description: 'Sample toast action button.' },
  toastDefault: { id: 'story.toast.default', defaultMessage: 'Link copied', description: 'Sample neutral toast.' },
});

// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export { cn } from './lib/cn';
export { ThemeProvider, useTheme, useResolvedTheme, applyTheme, themePreferences, type ThemePreference } from './providers/ThemeProvider';
export { DensityProvider, useDensity, applyDensity, densities, type Density } from './providers/DensityProvider';
export { I18nProvider, useLocale } from './providers/I18nProvider';
export { Providers } from './providers/Providers';
export { locales, productLocales, matchProductLocale, type Locale, type ProductLocale } from './i18n/locales';

// Components
export { ActiveToggle } from './components/active-toggle/ActiveToggle';
export type { ActiveToggleProps } from './components/active-toggle/ActiveToggle';
export { AlertDialog } from './components/alert-dialog/AlertDialog';
export type { AlertDialogProps } from './components/alert-dialog/AlertDialog';
export { AppShell } from './components/app-shell/AppShell';
export type { NavEntry, AppShellProps } from './components/app-shell/AppShell';
export { PhoneTopBar } from './components/app-shell/PhoneTopBar';
export type { PhoneTopBarProps } from './components/app-shell/PhoneTopBar';
export { AttentionInboxItem } from './components/attention-inbox-item/AttentionInboxItem';
export type { AttentionInboxItemProps } from './components/attention-inbox-item/AttentionInboxItem';
export { initials, Avatar } from './components/avatar/Avatar';
export type { AvatarProps } from './components/avatar/Avatar';
export { badgeVariants, Badge } from './components/badge/Badge';
export type { BadgeProps } from './components/badge/Badge';
export { Banner } from './components/banner/Banner';
export type { BannerTone, BannerProps } from './components/banner/Banner';
export { BrandMark } from './components/brand-mark/BrandMark';
export type { BrandMarkProps } from './components/brand-mark/BrandMark';
export { BrandHero } from './components/brand-hero/BrandHero';
export type { BrandHeroProps } from './components/brand-hero/BrandHero';
export { BottomTabBar } from './components/bottom-tab-bar/BottomTabBar';
export type { BottomTab, BottomTabBarProps } from './components/bottom-tab-bar/BottomTabBar';
export { Breadcrumb } from './components/breadcrumb/Breadcrumb';
export type { BreadcrumbItem, BreadcrumbProps } from './components/breadcrumb/Breadcrumb';
export { buttonVariants, Button } from './components/button/Button';
export type { ButtonProps } from './components/button/Button';
export { Checkbox } from './components/checkbox/Checkbox';
export type { CheckboxProps } from './components/checkbox/Checkbox';
export { CodeBlock } from './components/code-block/CodeBlock';
export type { CodeBlockProps } from './components/code-block/CodeBlock';
export { Combobox } from './components/combobox/Combobox';
export type { ComboboxOption, ComboboxProps } from './components/combobox/Combobox';
export { CommandPalette } from './components/command-palette/CommandPalette';
export type { PaletteCommand, CommandPaletteProps } from './components/command-palette/CommandPalette';
export { ComingSoon } from './components/coming-soon/ComingSoon';
export type { ComingSoonProps } from './components/coming-soon/ComingSoon';
export { ConnectorIcon } from './components/connector-icon/ConnectorIcon';
export type { ConnectorIconProps } from './components/connector-icon/ConnectorIcon';
export { CountTabs } from './components/count-tabs/CountTabs';
export type { CountTab, CountTabsProps } from './components/count-tabs/CountTabs';
export { DataTable } from './components/data-table/DataTable';
export type { SortDirection, SortState, DataColumn, DataTableProps } from './components/data-table/DataTable';
export { toDate, DateTime } from './components/date-time/DateTime';
export type { DateInput, DateTimeProps } from './components/date-time/DateTime';
export { DensitySwitcher } from './components/density-switcher/DensitySwitcher';
export type { DensitySwitcherProps } from './components/density-switcher/DensitySwitcher';
export { Dialog, DialogClose } from './components/dialog/Dialog';
export type { DialogProps } from './components/dialog/Dialog';
export { DropdownMenu } from './components/dropdown-menu/DropdownMenu';
export type { MenuEntry, DropdownMenuProps } from './components/dropdown-menu/DropdownMenu';
export { EmptyState } from './components/empty-state/EmptyState';
export type { EmptyStateProps } from './components/empty-state/EmptyState';
export { ErrorFallback, ErrorBoundary } from './components/error-boundary/ErrorBoundary';
export type { ErrorFallbackProps, ErrorBoundaryProps } from './components/error-boundary/ErrorBoundary';
export { ExecutionTimeline } from './components/execution-timeline/ExecutionTimeline';
export type { ExecutionTimelineProps, TimelineStep } from './components/execution-timeline/ExecutionTimeline';
export { PlainError } from './components/plain-error/PlainError';
export type { PlainErrorProps } from './components/plain-error/PlainError';
export { ErrorSummary } from './components/error-summary/ErrorSummary';
export type { FieldError, ErrorSummaryProps } from './components/error-summary/ErrorSummary';
export { FilterChip } from './components/filter-chip/FilterChip';
export type { FilterChipProps } from './components/filter-chip/FilterChip';
export { FormField } from './components/form-field/FormField';
export type { FormFieldProps } from './components/form-field/FormField';
export { FieldContext, useFieldControlProps, useField } from './components/form-field/field-context';
export type { FieldContextValue, ControlOwnProps } from './components/form-field/field-context';
export { ProblemErrorsProvider, useFieldProblemError } from './components/form-field/problem-errors';
export { IconButton } from './components/icon-button/IconButton';
export type { IconButtonProps } from './components/icon-button/IconButton';
export { Input } from './components/input/Input';
export type { InputProps } from './components/input/Input';
export { Kbd, KbdCombination } from './components/kbd/Kbd';
export { Link } from './components/link/Link';
export type { LinkProps } from './components/link/Link';
export { languageName, LocaleSwitcher } from './components/locale-switcher/LocaleSwitcher';
export type { LocaleSwitcherProps } from './components/locale-switcher/LocaleSwitcher';
export { ListRow } from './components/list-row/ListRow';
export type { ListRowProps } from './components/list-row/ListRow';
export { Mascot } from './components/mascot/Mascot';
export type { MascotPose, MascotProps } from './components/mascot/Mascot';
export { NavItem } from './components/nav-item/NavItem';
export type { NavItemProps } from './components/nav-item/NavItem';
export { OrgSwitcher } from './components/org-switcher/OrgSwitcher';
export type { OrgSwitcherOrganization, OrgSwitcherProps } from './components/org-switcher/OrgSwitcher';
export { NotificationBell } from './components/notification-bell/NotificationBell';
export type { NotificationBellProps } from './components/notification-bell/NotificationBell';
export { NumberFormat, formatDuration, Duration } from './components/number-format/NumberFormat';
export type { NumberFormatProps } from './components/number-format/NumberFormat';
export { Pagination } from './components/pagination/Pagination';
export type { PaginationProps } from './components/pagination/Pagination';
export { Popover } from './components/popover/Popover';
export type { PopoverProps } from './components/popover/Popover';
export { ProblemAlert } from './components/problem-alert/ProblemAlert';
export type { ProblemAlertProps } from './components/problem-alert/ProblemAlert';
export { problemMessages, knownProblemMessages, fieldIdForName, problemFieldErrors, errorsByPointer } from './components/problem-alert/problem';
export type { ProblemDetails } from './components/problem-alert/problem';
export { RadioGroup, RadioGroupItem } from './components/radio-group/RadioGroup';
export type { RadioGroupProps, RadioGroupItemProps } from './components/radio-group/RadioGroup';
export { RedactedValue, ProtectedValue } from './components/redacted-value/RedactedValue';
export type { ProtectedValueProps } from './components/redacted-value/RedactedValue';
export { relativeParts, RelativeTime } from './components/relative-time/RelativeTime';
export type { RelativeParts, RelativeTimeProps } from './components/relative-time/RelativeTime';
export { ScrollArea } from './components/scroll-area/ScrollArea';
export type { ScrollAreaProps } from './components/scroll-area/ScrollArea';
export { Select } from './components/select/Select';
export type { SelectOption, SelectProps } from './components/select/Select';
export { Separator } from './components/separator/Separator';
export type { SeparatorProps } from './components/separator/Separator';
export { Sheet } from './components/sheet/Sheet';
export type { SheetProps } from './components/sheet/Sheet';
export { SidebarButton, SidebarLink, SidebarNav, useSidebarCollapsed } from './components/sidebar-nav/SidebarNav';
export type { SidebarButtonProps, SidebarItem, SidebarLinkProps, SidebarNavProps, SidebarSection } from './components/sidebar-nav/SidebarNav';
export { Skeleton } from './components/skeleton/Skeleton';
export type { SkeletonProps } from './components/skeleton/Skeleton';
export { Spinner } from './components/spinner/Spinner';
export type { SpinnerProps } from './components/spinner/Spinner';
export { UncertainResolutionPanel } from './components/uncertain-resolution/UncertainResolutionPanel';
export type { UncertainResolutionAction, UncertainResolutionPanelProps } from './components/uncertain-resolution/UncertainResolutionPanel';
export { StatusBadge } from './components/status-badge/StatusBadge';
export type { StatusBadgeProps } from './components/status-badge/StatusBadge';
export { statusMessages, statusMap, statusKeys } from './components/status-badge/status-map';
export type { StatusFamily, StatusKey, StatusEntry } from './components/status-badge/status-map';
export { Switch } from './components/switch/Switch';
export type { SwitchProps } from './components/switch/Switch';
export { Table, TableCaption, TableHeader, TableBody, TableRow, TableHead, TableCell } from './components/table/Table';
export type { TableCaptionProps, TableRowProps, TableHeadProps, TableCellProps } from './components/table/Table';
export { Tabs } from './components/tabs/Tabs';
export type { TabItem, TabsProps } from './components/tabs/Tabs';
export { Textarea } from './components/textarea/Textarea';
export type { TextareaProps } from './components/textarea/Textarea';
export { ThemeSwitcher } from './components/theme-switcher/ThemeSwitcher';
export type { ThemeSwitcherProps } from './components/theme-switcher/ThemeSwitcher';
export { Toaster } from './components/toast/Toast';
export { Tooltip } from './components/tooltip/Tooltip';
export type { TooltipProps } from './components/tooltip/Tooltip';
export { TopBar } from './components/top-bar/TopBar';
export type { TopBarProps } from './components/top-bar/TopBar';
export { UserMenu } from './components/user-menu/UserMenu';
export type { UserMenuItem, UserMenuProps } from './components/user-menu/UserMenu';

// Canvas (style A)
export { CanvasNode, tileRadius } from './canvas/canvas-node/CanvasNode';
export type { CanvasNodeProps } from './canvas/canvas-node/CanvasNode';
export { NodeStatusBadge } from './canvas/node-status-badge/NodeStatusBadge';
export type { NodeStatusBadgeProps } from './canvas/node-status-badge/NodeStatusBadge';
export { NodeStatusChip } from './canvas/node-status-chip/NodeStatusChip';
export type { ChipKind, NodeStatusChipProps } from './canvas/node-status-chip/NodeStatusChip';
export { PinnedBadge } from './canvas/pinned-badge/PinnedBadge';
export type { PinnedBadgeProps } from './canvas/pinned-badge/PinnedBadge';
export { NodePort } from './canvas/node-port/NodePort';
export type { NodePortProps } from './canvas/node-port/NodePort';
export { PortAddButton } from './canvas/port-add-button/PortAddButton';
export type { PortAddButtonProps } from './canvas/port-add-button/PortAddButton';
export { OutputStub } from './canvas/output-stub/OutputStub';
export type { OutputStubProps } from './canvas/output-stub/OutputStub';
export { CanvasEdge, edgeGeometry } from './canvas/canvas-edge/CanvasEdge';
export type { CanvasEdgeData, CanvasFlowEdge, EdgeGeometry } from './canvas/canvas-edge/CanvasEdge';
export { EdgeLabel, useEdgeLabelText } from './canvas/edge-label/EdgeLabel';
export type { EdgeLabelProps } from './canvas/edge-label/EdgeLabel';
export { EdgeActions } from './canvas/edge-actions/EdgeActions';
export type { EdgeActionsProps } from './canvas/edge-actions/EdgeActions';
export { NodeToolbar } from './canvas/node-toolbar/NodeToolbar';
export type { NodeToolbarProps } from './canvas/node-toolbar/NodeToolbar';
export { StickyNote } from './canvas/sticky-note/StickyNote';
export type { StickyNoteProps } from './canvas/sticky-note/StickyNote';
export { CanvasControls } from './canvas/canvas-controls/CanvasControls';
export type { CanvasControlsProps } from './canvas/canvas-controls/CanvasControls';
export { CanvasMinimap } from './canvas/canvas-minimap/CanvasMinimap';
export type { CanvasMinimapProps } from './canvas/canvas-minimap/CanvasMinimap';
export { AddFirstStep } from './canvas/add-first-step/AddFirstStep';
export type { AddFirstStepProps } from './canvas/add-first-step/AddFirstStep';
export { AiBlock } from './canvas/ai-block/AiBlock';
export type { AiBlockProps } from './canvas/ai-block/AiBlock';
export { NodePalettePanel } from './canvas/node-palette-panel/NodePalettePanel';
export type { NodePalettePanelProps, PaletteSuggestions } from './canvas/node-palette-panel/NodePalettePanel';
export { CanvasOutline } from './canvas/canvas-outline/CanvasOutline';
export type { CanvasOutlineProps } from './canvas/canvas-outline/CanvasOutline';
export { WorkflowCanvas } from './canvas/WorkflowCanvas';
export { applyOverlay } from './canvas/WorkflowCanvas';
export type { EdgeExecutionState, ExecutionOverlay, NodeExecutionState, WorkflowCanvasHandle, WorkflowCanvasProps } from './canvas/WorkflowCanvas';
export { useAnnouncer, ANNOUNCE_INTERVAL_MS } from './canvas/announcer';

// Node details view (style A): three panes in a dialog from lg, a docked Parameters pane from md
export { NodeDetailsView } from './editor/node-details-view/NodeDetailsView';
export type { DetailsTab, NodeDetailsViewProps } from './editor/node-details-view/NodeDetailsView';
export { ExpressionChip } from './editor/expression-chips/ExpressionChip';
export type { ExpressionChipProps } from './editor/expression-chips/ExpressionChip';
export { DataPane } from './editor/data-pane/DataPane';
export type { DataPaneProps, OutputPort } from './editor/data-pane/DataPane';
export { FIELD_DRAG_TYPE } from './editor/model';
export type { DataItem, DataRun, DataView, FieldPath, PaneData } from './editor/model';
export type {
  AiBlockKind,
  AiBlockSpec,
  CatalogItem,
  ConnectError,
  ConnectRequest,
  DiffMark,
  Direction,
  EdgeState,
  Graph,
  GraphEdge,
  GraphNode,
  LogicType,
  NodeIconName,
  NodeKind,
  PortSpec,
  StickyColor,
  StickyNoteData,
} from './canvas/model';
export {
  COLUMN_PITCH,
  GRID,
  addNode,
  addNodeAfter,
  connect,
  connectError,
  insertOnEdge,
  itemCounts,
  moveNodes,
  nextFocus,
  readingOrder,
  removeEdge,
  removeNodes,
} from './canvas/model';

// Screen templates (sample data, no HTTP)
export { AppearanceSettingsScreen } from './screens/AppearanceSettingsScreen';
export { ApprovalScreen } from './screens/ApprovalScreen';
export type { ApprovalOutcome, ApprovalScreenProps } from './screens/ApprovalScreen';
export { AttentionInboxScreen } from './screens/AttentionInboxScreen';
export type { AttentionInboxScreenProps } from './screens/AttentionInboxScreen';
export { EmptyStatesScreen } from './screens/EmptyStatesScreen';
export { ErrorPageScreen } from './screens/ErrorPageScreen';
export type { ErrorPageScreenProps } from './screens/ErrorPageScreen';
export { ExecutionDetailScreen } from './screens/ExecutionDetailScreen';
export type { ExecutionDetailScreenProps } from './screens/ExecutionDetailScreen';
export { ExecutionListScreen } from './screens/ExecutionListScreen';
export type { ExecutionListScreenProps } from './screens/ExecutionListScreen';
export { FlowEditorScreen } from './screens/FlowEditorScreen';
export type { FlowEditorScreenProps } from './screens/FlowEditorScreen';
export { FlowListScreen } from './screens/FlowListScreen';
export type { FlowListScreenProps } from './screens/FlowListScreen';
export { NotFoundScreen } from './screens/NotFoundScreen';
export { PageHeader } from './screens/PageHeader';
export type { PageHeaderProps } from './screens/PageHeader';
export { ProductShell } from './screens/ProductShell';
export type { ProductShellProps, ShellSection } from './screens/ProductShell';
export { WelcomeContent, WelcomeScreen } from './screens/WelcomeScreen';
export type { WelcomeScreenProps } from './screens/WelcomeScreen';
export { FIXTURE_NOW } from './screens/fixtures';

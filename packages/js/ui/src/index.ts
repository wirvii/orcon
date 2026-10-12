// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export { cn } from './lib/cn';
export { ThemeProvider, useTheme, applyTheme, themePreferences, type ThemePreference } from './providers/ThemeProvider';
export { DensityProvider, useDensity, applyDensity, densities, type Density } from './providers/DensityProvider';
export { I18nProvider, useLocale } from './providers/I18nProvider';
export { Providers } from './providers/Providers';
export { locales, productLocales, matchProductLocale, type Locale, type ProductLocale } from './i18n/locales';

// Components
export { AlertDialog } from './components/alert-dialog/AlertDialog';
export type { AlertDialogProps } from './components/alert-dialog/AlertDialog';
export { AppShell } from './components/app-shell/AppShell';
export type { NavEntry, AppShellProps } from './components/app-shell/AppShell';
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
export { ConnectorIcon } from './components/connector-icon/ConnectorIcon';
export type { ConnectorIconProps } from './components/connector-icon/ConnectorIcon';
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
export { ErrorSummary } from './components/error-summary/ErrorSummary';
export type { FieldError, ErrorSummaryProps } from './components/error-summary/ErrorSummary';
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
export { Mascot } from './components/mascot/Mascot';
export type { MascotPose, MascotProps } from './components/mascot/Mascot';
export { NavItem } from './components/nav-item/NavItem';
export type { NavItemProps } from './components/nav-item/NavItem';
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

// Canvas
export { OrconEdge, loopBackPath } from './canvas/Edge';
export type { OrconEdgeData, OrconFlowEdge } from './canvas/Edge';
export { EdgeCount } from './canvas/EdgeCount';
export type { EdgeCountProps } from './canvas/EdgeCount';
export { CanvasOutline } from './canvas/CanvasOutline';
export type { CanvasOutlineProps } from './canvas/CanvasOutline';
export { CanvasToolbar } from './canvas/CanvasToolbar';
export type { CanvasToolbarProps } from './canvas/CanvasToolbar';
export { ControlNode } from './canvas/ControlNode';
export type { ControlNodeProps } from './canvas/ControlNode';
export { NodeCard } from './canvas/NodeCard';
export type { NodeCardProps } from './canvas/NodeCard';
export { NodeStatus } from './canvas/NodeStatus';
export type { NodeStatusProps } from './canvas/NodeStatus';
export { Port } from './canvas/Port';
export type { PortProps } from './canvas/Port';
export { WorkflowCanvas } from './canvas/WorkflowCanvas';
export type { WorkflowCanvasProps, NodeExecutionState } from './canvas/WorkflowCanvas';
export { useAnnouncer, ANNOUNCE_INTERVAL_MS } from './canvas/announcer';
export type {
  CatalogItem,
  ConnectError,
  ConnectRequest,
  ControlType,
  Direction,
  Graph,
  GraphEdge,
  GraphNode,
  NodeKind,
  PortSpec,
} from './canvas/model';
export {
  GRID,
  addNode,
  connect,
  connectError,
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
export { ProductShell } from './screens/ProductShell';
export type { ProductShellProps, ShellSection } from './screens/ProductShell';
export { WelcomeScreen } from './screens/WelcomeScreen';
export type { WelcomeScreenProps } from './screens/WelcomeScreen';
export { FIXTURE_NOW } from './screens/fixtures';

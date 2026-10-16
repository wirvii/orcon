// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Link } from './Link';
import { states } from './Link.meta';

const meta = {
  title: 'Components/Link',
  component: Link,
  args: { href: '#docs', children: 'Read the documentation' },
} satisfies Meta<typeof Link>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Active: Story = { parameters: { pseudo: { active: true } } };
export const External: Story = { args: { external: true, href: 'https://example.com' } };
export const Prose: Story = { args: { variant: 'prose' } };

/** Inside running text the target size is exempt; the container declares it with data-prose. */
export const InText: Story = {
  render: () => (
    <p data-prose className="max-w-screen-sm">
      Credentials are stored encrypted. See the{' '}
      <Link inText href="#security">
        security notes
      </Link>{' '}
      before sharing a flow.
    </p>
  ),
};

const stateProps: Record<(typeof states)[number], { className?: string; external?: boolean; variant?: 'prose' | 'plain' }> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  active: { className: 'pseudo-active' },
  external: { external: true },
  prose: { variant: 'prose' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Link states"
      columns={[...states]}
      rows={[
        {
          label: 'link',
          cells: states.map((state) => (
            <Link key={state} href="#docs" {...stateProps[state]}>
              Documentation
            </Link>
          )),
        },
      ]}
    />
  ),
};

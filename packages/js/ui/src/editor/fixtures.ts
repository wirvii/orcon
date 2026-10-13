// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Sample data for stories, tests and the editor template. No request is made.

import { protectedValue, type DataItem } from './model';

export const orderItems: DataItem[] = [
  {
    id: 'ord_1001',
    customer: 'Ana Torres',
    total: 128.5,
    paid: true,
    coupon: null,
    lines: [
      { sku: 'A-100', qty: 2 },
      { sku: 'B-900', qty: 1 },
    ],
    shipping: { city: 'Neiva', zip: '410001', country: 'CO' },
  },
  {
    id: 'ord_1002',
    customer: 'Luis Pérez',
    total: 54,
    paid: false,
    coupon: 'WELCOME10',
    lines: [{ sku: 'C-310', qty: 3 }],
    shipping: { city: 'Bogotá', zip: '110111', country: 'CO' },
  },
  {
    id: 'ord_1003',
    customer: 'Marta Ruiz',
    total: 310.75,
    paid: true,
    coupon: null,
    lines: [],
    shipping: { city: 'Cali', zip: '760001', country: 'CO' },
  },
];

/** Items with every kind of value the panel renders: protected values, a file, nested data and null. */
export const mixedItems: DataItem[] = [
  {
    id: 'ord_2001',
    customer: 'Ana Torres',
    card: protectedValue('permission'),
    apiToken: protectedValue('secret'),
    internalNote: protectedValue('history'),
    invoice: { $binary: { name: 'invoice-2001.pdf', mediaType: 'application/pdf', size: 48213, href: '#invoice-2001.pdf' } },
    coupon: null,
    shipping: { city: 'Neiva', zip: '410001' },
  },
  { id: 'ord_2002', customer: 'Luis Pérez', card: protectedValue('permission'), apiToken: protectedValue('secret'), internalNote: null, invoice: null, coupon: 'WELCOME10', shipping: { city: 'Bogotá', zip: '110111' } },
];

/** A long list, for the windowing story and the unit test. */
export const manyItems = (count: number): DataItem[] =>
  Array.from({ length: count }, (_, index) => ({ id: `ord_${3000 + index}`, customer: `Customer ${index + 1}`, total: Math.round(index * 13.7 * 100) / 100, paid: index % 2 === 0, coupon: index % 5 === 0 ? 'WELCOME10' : null }));

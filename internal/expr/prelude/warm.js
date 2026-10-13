// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Warm-up of the caches inside Luxon, run before the memory snapshot of the
// runtime is taken (ADR-0018 section 3: "Luxon is preloaded in a snapshot").
// Luxon builds formatters, zone objects, locale objects and format parsers the
// first time an operation needs them and keeps them in closure variables;
// doing that here, with fixed instants, means no evaluation pays for it.
//
// Luxon is not modified and its Settings are not touched: every operation
// names its zone, so the default zone and clock stay as the runtime sets them
// for each call. The snapshot must be the same every time it is built
// (TestSnapshotDeterministic), so only fixed instants and no randomness are
// used.
(function () {
  'use strict';
  const { DateTime, Duration, Interval } = globalThis.luxon;
  const FIXED = 1773484200000; // 2026-03-14T10:30:00Z
  const zones = ['UTC', 'America/Bogota', 'America/New_York', 'Europe/Madrid', 'Europe/London', 'Asia/Tokyo'];
  // A warm-up step that fails only leaves a cache cold.
  const attempt = (fn) => { try { fn(); } catch (e) { /* ignored on purpose */ } };

  for (const zone of zones) {
    attempt(() => {
      const dt = DateTime.fromMillis(FIXED, { zone });
      dt.toISO();
      dt.toFormat('yyyy-MM-dd');
      dt.toFormat('yyyy-MM-dd HH:mm:ss');
      dt.toFormat('dd/MM/yyyy');
      dt.toFormat('MMMM d, yyyy h:mm a');
      dt.toFormat('ccc, dd LLL yyyy HH:mm ZZ');
      dt.toLocaleString(DateTime.DATE_FULL);
      dt.toLocaleString(DateTime.DATETIME_MED);
      dt.plus({ days: 7 }).toISO();
      dt.plus({ hours: 5, minutes: 30 });
      dt.minus({ months: 1 }).toISODate();
      dt.startOf('day');
      dt.startOf('week');
      dt.endOf('month');
      dt.setZone('UTC').toISO();
      dt.diff(DateTime.fromMillis(FIXED - 86400000, { zone }), ['days', 'hours']).toObject();
      dt.toRelative({ base: DateTime.fromMillis(FIXED + 3600000, { zone }) });
      dt.toMillis();
      dt.toJSDate();
    });
  }
  attempt(() => {
    DateTime.fromISO('2026-03-14T10:30:00Z', { zone: 'UTC' }).toFormat('dd/MM/yyyy');
    DateTime.fromISO('2026-03-14', { zone: 'UTC' });
    DateTime.fromISO('2026-03-14T10:30:00-05:00', { setZone: true });
    DateTime.fromFormat('14/03/2026 10:30', 'dd/MM/yyyy HH:mm', { zone: 'UTC' });
    DateTime.fromFormat('2026-03-14', 'yyyy-MM-dd', { zone: 'UTC' });
    DateTime.fromSQL('2026-03-14 10:30:00', { zone: 'UTC' });
    DateTime.fromRFC2822('Sat, 14 Mar 2026 10:30:00 +0000', { zone: 'UTC' });
    DateTime.fromSeconds(FIXED / 1000, { zone: 'UTC' });
    DateTime.fromJSDate(new Date(FIXED), { zone: 'UTC' });
    DateTime.fromObject({ year: 2026, month: 3, day: 14 }, { zone: 'UTC' });
    DateTime.utc(2026, 3, 14);
    Duration.fromObject({ hours: 2, minutes: 5 }).toFormat('hh:mm');
    Duration.fromObject({ days: 1 }).toISO();
    Duration.fromMillis(90061000).shiftTo('days', 'hours', 'minutes').toObject();
    Interval.after(DateTime.fromMillis(FIXED, { zone: 'UTC' }), { days: 1 }).toISO();
  });
  attempt(() => {
    const d = new Date(FIXED);
    d.toISOString();
    d.toLocaleDateString('en-US');
    d.toLocaleTimeString('en-US');
    d.toLocaleString('en-US', { dateStyle: 'medium' });
    (1234.5).toLocaleString('en-US');
    (1234.5).toLocaleString('en-US', { style: 'currency', currency: 'USD' });
  });
  attempt(() => {
    'abc'.hash('md5');
    'abc'.hash('sha256');
  });
})();

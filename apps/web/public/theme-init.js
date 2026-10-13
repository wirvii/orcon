// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Runs before the app. It puts the person's saved theme, density and language on <html> so the
// first paint already matches them. Without a saved theme nothing is set and the system setting
// decides (prefers-color-scheme). The keys are the ones the @orcon/ui providers write.
(function () {
  try {
    var root = document.documentElement;
    var theme = window.localStorage.getItem('orcon.theme');
    if (theme === 'light' || theme === 'dark') root.setAttribute('data-theme', theme);
    if (window.localStorage.getItem('orcon.density') === 'compact') root.setAttribute('data-density', 'compact');
    var locale = window.localStorage.getItem('orcon.locale');
    if (locale === 'es' || locale === 'en') root.setAttribute('lang', locale);
  } catch {
    // Storage can be blocked (private mode). The defaults apply.
  }
})();

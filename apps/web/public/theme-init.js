// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Runs before the app. It puts the person's saved theme, density and language on <html> so the
// first paint already matches them. Without a saved theme the app starts dark (SPEC-019 R-UX-GEN-02);
// a person who chose "like the system" has `system` saved and nothing is set, so the system setting
// decides (prefers-color-scheme). The keys are the ones the @orcon/ui providers write.
(function () {
  try {
    var root = document.documentElement;
    var theme = window.localStorage.getItem('orcon.theme');
    if (theme === 'light' || theme === 'dark') root.setAttribute('data-theme', theme);
    else if (theme !== 'system') root.setAttribute('data-theme', 'dark');
    if (window.localStorage.getItem('orcon.density') === 'compact') root.setAttribute('data-density', 'compact');
    var locale = window.localStorage.getItem('orcon.locale');
    if (locale === 'es' || locale === 'en') root.setAttribute('lang', locale);
  } catch {
    // Storage can be blocked (private mode). The defaults apply.
  }
})();

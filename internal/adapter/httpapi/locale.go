// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
)

// The languages the API answers in. Names and descriptions it shows to people follow the
// Accept-Language header of the request; any other language gets English.
const (
	// LocaleEnglish is the default language.
	LocaleEnglish = "en"
	// LocaleSpanish is the other language of the product.
	LocaleSpanish = "es"
)

type localeKey struct{}

// withLocale puts the language of the request in its context, read from Accept-Language.
func withLocale() middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), localeKey{}, negotiateLocale(r.Header.Get("Accept-Language")))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LocaleFrom returns the language of the request: [LocaleSpanish] or [LocaleEnglish].
func LocaleFrom(ctx context.Context) string {
	if l, ok := ctx.Value(localeKey{}).(string); ok {
		return l
	}
	return LocaleEnglish
}

// WithLocale returns a context that carries the language, for code that runs outside a request.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

// negotiateLocale picks the first language of the header that the API speaks, in the order the
// header lists them (quality values are not weighed: the header of a browser lists them by
// preference).
func negotiateLocale(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		primary, _, _ := strings.Cut(strings.ToLower(tag), "-")
		switch primary {
		case LocaleSpanish:
			return LocaleSpanish
		case LocaleEnglish:
			return LocaleEnglish
		}
	}
	return LocaleEnglish
}

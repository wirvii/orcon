// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

func TestClassifyStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		want   Class
	}{
		{status: 429, want: ClassTransient},
		{status: 500, want: ClassTransient},
		{status: 503, want: ClassTransient},
		{status: 529, want: ClassTransient},
		{status: 400, want: ClassPermanent},
		{status: 401, want: ClassPermanent},
		{status: 403, want: ClassPermanent},
		{status: 404, want: ClassPermanent},
		{status: 422, want: ClassPermanent},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			t.Parallel()
			if got := Classify(tt.status); got != tt.want {
				t.Errorf("Classify(%d) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsTransientSeesWrappedErrors(t *testing.T) {
	t.Parallel()
	transient := fmt.Errorf("step 3: %w", &Error{Class: ClassTransient, Status: 429})
	permanent := &Error{Class: ClassPermanent, Status: 401}
	if !IsTransient(transient) {
		t.Error("IsTransient(wrapped transient) = false, want true")
	}
	if IsTransient(permanent) {
		t.Error("IsTransient(permanent) = true, want false")
	}
	if IsTransient(errors.New("plain")) {
		t.Error("IsTransient(plain error) = true, want false")
	}
}

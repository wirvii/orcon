// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const sample = `#include <string.h>

static int checked_loop(JSContext *ctx, JSString *p)
{
    int i;
    for (i = 0; i < p->len; i++) {
        if (js_poll_work(ctx, 8))
            return -1;
    }
    return 0;
}

static JSValue
unchecked_move(JSContext *ctx, uint8_t *dst, const uint8_t *src, int len)
{
    memmove(dst, src + 1, len);
    memset(dst, 0, sizeof(*dst));
    return JS_UNDEFINED;
}

static int interrupts(JSContext *ctx, int len)
{
    int i;
    for (i = 0; i < len; i++) {
        if (js_poll_interrupts(ctx))
            return -1;
    }
    return 0;
}

static int charged(JSContext *ctx, int count)
{
    while (count > 0) {
        js_charge_work(ctx, 4);
        count--;
    }
    return 0;
}

static int constant(void)
{
    for (int i = 0; i < 3; i++) {}
    return 0;
}
`

func TestScan(t *testing.T) {
	t.Parallel()
	got := map[string]Func{}
	for _, f := range Scan(sample) {
		got[f.Name] = f
	}
	if f := got["checked_loop"]; f.Loops != 1 || f.Polls != 1 || !f.Checked() {
		t.Errorf("checked_loop = %+v", f)
	}
	if f := got["unchecked_move"]; f.Moves != 1 || f.Checked() {
		t.Errorf("unchecked_move = %+v, want one variable move (the sizeof memset is constant) and no check", f)
	}
	if f := got["interrupts"]; f.Loops != 1 || f.Polls != 1 || !f.Checked() {
		t.Errorf("interrupts = %+v", f)
	}
	if f := got["charged"]; f.Loops != 1 || f.Charges != 1 || !f.Checked() {
		t.Errorf("charged = %+v", f)
	}
	if f := got["constant"]; f.Loops != 0 || f.Moves != 0 {
		t.Errorf("constant = %+v, want nothing", f)
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	out := Render(Scan(sample))
	for _, want := range []string{"checked_loop 1 0 1 0 checked\n", "unchecked_move 0 1 0 0 unchecked\n", "charged 1 0 0 1 checked\n", "# 4 functions, 1 unchecked\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "constant ") {
		t.Errorf("a function with no input-dependent loop is listed:\n%s", out)
	}
}

func TestRunWritesAFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "quickjs.c"), filepath.Join(dir, "out.txt")
	if err := os.WriteFile(src, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{"-src", src, "-out", dst}, &stdout); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(dst); err != nil || !strings.Contains(string(data), "charged") { //nolint:gosec // a temporary test file
		t.Errorf("output file: %v %q", err, data)
	}
	if err := run(nil, &stdout); err == nil {
		t.Error("a missing -src is accepted")
	}
}

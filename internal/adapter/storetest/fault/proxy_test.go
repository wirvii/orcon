// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package fault_test

import (
	"context"
	"io"
	"net"
	"net/url"
	"sync"
	"testing"
)

// cutProxy forwards the connections of a child process to the database. Cut ends
// every forwarded connection and refuses new ones until Restore, which is a network
// cut between a worker and its database (ADR-0019 FI-07). The address does not
// change across cuts, so the worker's pool reconnects to the same place.
type cutProxy struct {
	t      *testing.T
	target string
	addr   string

	mu     sync.Mutex
	ln     net.Listener
	conns  map[net.Conn]struct{}
	closed bool
	wg     sync.WaitGroup
}

// newCutProxy starts a proxy to target, a host:port, on a port of its own.
func newCutProxy(t *testing.T, target string) *cutProxy {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for the proxy: %v", err)
	}
	p := &cutProxy{t: t, target: target, addr: ln.Addr().String(), conns: map[net.Conn]struct{}{}}
	p.serve(ln)
	t.Cleanup(p.Close)
	return p
}

// DSN returns dsn with its host and port replaced by the proxy's.
func (p *cutProxy) DSN(dsn string) string {
	p.t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		p.t.Fatalf("parse the DSN: %v", err)
	}
	u.Host = p.addr
	return u.String()
}

// serve accepts connections on ln until it is closed.
func (p *cutProxy) serve(ln net.Listener) {
	p.mu.Lock()
	p.ln = ln
	p.mu.Unlock()
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			p.wg.Add(1)
			go p.forward(c)
		}
	}()
}

// forward copies the bytes both ways until either side ends.
func (p *cutProxy) forward(client net.Conn) {
	defer p.wg.Done()
	server, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", p.target)
	if err != nil {
		_ = client.Close()
		return
	}
	if !p.track(client, server) {
		_ = client.Close()
		_ = server.Close()
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(server, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, server); done <- struct{}{} }()
	<-done
	p.untrack(client, server)
}

// track registers a pair of connections so that Cut can end them. It reports
// false when the proxy is closed.
func (p *cutProxy) track(conns ...net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	for _, c := range conns {
		p.conns[c] = struct{}{}
	}
	return true
}

func (p *cutProxy) untrack(conns ...net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range conns {
		delete(p.conns, c)
		_ = c.Close()
	}
}

// Cut ends the connections in flight and stops accepting new ones.
func (p *cutProxy) Cut() {
	p.t.Helper()
	p.mu.Lock()
	ln := p.ln
	p.ln = nil
	for c := range p.conns {
		_ = c.Close()
	}
	p.conns = map[net.Conn]struct{}{}
	p.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
}

// Restore listens again on the same address.
func (p *cutProxy) Restore() {
	p.t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", p.addr)
	if err != nil {
		p.t.Fatalf("restore the proxy on %s: %v", p.addr, err)
	}
	p.serve(ln)
}

// Close cuts the proxy for good.
func (p *cutProxy) Close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.Cut()
	p.wg.Wait()
}

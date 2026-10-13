// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// NoticeLog collects the notices a connection receives. The auto_explain
// module reports nested plans as notices (auto_explain.log_level = notice).
type NoticeLog struct {
	mu   sync.Mutex
	msgs []string
}

// Handler is the pgconn notice handler that fills the log.
func (n *NoticeLog) Handler(_ *pgconn.PgConn, notice *pgconn.Notice) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.msgs = append(n.msgs, notice.Message)
}

func (n *NoticeLog) take() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.msgs
	n.msgs = nil
	return out
}

// ConnectForPlans opens a connection whose notices land in the returned log,
// as CheckPlansPostgres needs for engine functions.
func ConnectForPlans(ctx context.Context, connString string) (*pgx.Conn, *NoticeLog, error) {
	cfg, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, nil, err
	}
	log := &NoticeLog{}
	cfg.OnNotice = log.Handler
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return conn, log, nil
}

// EngineCall is a statement that runs an orcon_engine function with literal
// arguments, for example SELECT * FROM orcon_engine.claim_tokens('w', 10).
type EngineCall struct {
	// Name identifies the function in findings.
	Name string
	// SQL is the statement to run. It runs in a transaction that is rolled back.
	SQL string
	// List is true when the function returns an ordered list, so a sort step
	// in a nested plan is a finding.
	List bool
}

// planNode is the part of an EXPLAIN (FORMAT JSON) plan the rules read.
type planNode struct {
	NodeType     string     `json:"Node Type"`
	RelationName string     `json:"Relation Name"`
	Plans        []planNode `json:"Plans"`
}

// CheckPlansPostgres checks the plans of the hot queries and of the engine
// functions with the deterministic method of checks.md CK-10 and CK-11.
//
// Queries: SET enable_seqscan = off (and enable_sort = off for list queries
// with ORDER BY), PREPARE, plan_cache_mode = force_generic_plan, and EXPLAIN
// (FORMAT JSON) EXECUTE with zero values of the inferred parameter types. With
// enable_seqscan off the planner picks Seq Scan only when no index is usable, so
// any Seq Scan node is a finding, and so is a Sort node in a list.
//
// Engine functions: SECURITY DEFINER bodies are not expanded by EXPLAIN, so the
// connection loads auto_explain (a superuser connection of the test database;
// never production) with log_nested_statements, log_min_duration = 0,
// log_format = json and log_level = notice, runs the call, reads the nested
// plans from the notices in log and applies the same rules. The connection must
// come from ConnectForPlans when calls are given.
func CheckPlansPostgres(ctx context.Context, conn *pgx.Conn, queries []Query, calls []EngineCall, log *NoticeLog) []Finding {
	var out []Finding
	for _, q := range queries {
		if !q.Hot {
			continue
		}
		plan, err := postgresPlan(ctx, conn, q)
		if err != nil {
			out = append(out, Finding{Rule: "QY-4", Object: "query " + q.Name, Message: "cannot plan the query: " + err.Error()})
			continue
		}
		out = append(out, planFindings("query "+q.Name, plan, q.List() && q.Ordered)...)
	}
	if len(calls) > 0 && log == nil {
		return append(out, Finding{Rule: "QY-4", Object: "engine functions", Message: "plans of engine functions need a connection from ConnectForPlans and its NoticeLog"})
	}
	for _, c := range calls {
		plans, err := autoExplainPlans(ctx, conn, log, c)
		if err != nil {
			out = append(out, Finding{Rule: "QY-4", Object: "function " + c.Name, Message: "cannot capture the nested plans: " + err.Error()})
			continue
		}
		if len(plans) == 0 {
			out = append(out, Finding{Rule: "QY-4", Object: "function " + c.Name, Message: "auto_explain reported no plan; the call ran nothing it could explain"})
			continue
		}
		for _, p := range plans {
			out = append(out, planFindings("function "+c.Name, p, c.List)...)
		}
	}
	return out
}

// planFindings applies the QY-4 rules to one plan: a Seq Scan, and a sort in a list.
func planFindings(object string, root planNode, list bool) []Finding {
	var out []Finding
	var walk func(n planNode)
	walk = func(n planNode) {
		switch {
		case n.NodeType == "Seq Scan":
			out = append(out, Finding{Rule: "QY-4", Object: object, Message: "full table scan (Seq Scan on " + n.RelationName + " with enable_seqscan = off); add or reorder an index (QY-7) or use keyset pagination (QY-5)"})
		case list && (n.NodeType == "Sort" || n.NodeType == "Incremental Sort"):
			out = append(out, Finding{Rule: "QY-4", Object: object, Message: "list query sorts (" + n.NodeType + " with enable_sort = off); an index must give the order"})
		}
		for _, c := range n.Plans {
			walk(c)
		}
	}
	walk(root)
	return out
}

func parsePlan(raw string) (planNode, error) {
	var wrapped []struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil && len(wrapped) == 1 {
		return wrapped[0].Plan, nil
	}
	var single struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(raw), &single); err != nil || single.Plan.NodeType == "" {
		return planNode{}, fmt.Errorf("unreadable plan %q", truncate(raw, 120))
	}
	return single.Plan, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

const preparedName = "dbrules_plan"

// postgresPlan prepares q and returns its generic plan.
func postgresPlan(ctx context.Context, conn *pgx.Conn, q Query) (plan planNode, err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return planNode{}, err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) && err == nil {
			err = rbErr
		}
	}()
	simple := pgx.QueryExecModeSimpleProtocol
	settings := []string{"SET LOCAL enable_seqscan = off", "SET LOCAL plan_cache_mode = force_generic_plan"}
	if q.List() && q.Ordered {
		settings = append(settings, "SET LOCAL enable_sort = off")
	}
	for _, s := range settings {
		if _, err := tx.Exec(ctx, s, simple); err != nil {
			return planNode{}, err
		}
	}
	if _, err := tx.Exec(ctx, "PREPARE "+preparedName+" AS "+q.SQL, simple); err != nil {
		return planNode{}, err
	}
	types, err := preparedParameterTypes(ctx, tx)
	if err != nil {
		return planNode{}, err
	}
	call := "EXPLAIN (FORMAT JSON) EXECUTE " + preparedName
	if len(types) > 0 {
		values := make([]string, len(types))
		for i, t := range types {
			values[i] = zeroValue(t)
		}
		call += "(" + strings.Join(values, ", ") + ")"
	}
	var raw string
	if err := tx.QueryRow(ctx, call, simple).Scan(&raw); err != nil {
		return planNode{}, err
	}
	if _, err := tx.Exec(ctx, "DEALLOCATE "+preparedName, simple); err != nil {
		return planNode{}, err
	}
	return parsePlan(raw)
}

func preparedParameterTypes(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT t::text FROM pg_prepared_statements ps, unnest(ps.parameter_types) WITH ORDINALITY u(t, ord) WHERE ps.name = $1 ORDER BY u.ord`, preparedName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// zeroValue is the zero literal of a parameter type; a type without one gets a
// typed NULL, which a generic plan treats the same.
func zeroValue(typ string) string {
	switch typ {
	case "uuid":
		return "'00000000-0000-0000-0000-000000000000'::uuid"
	case "bigint", "integer", "smallint":
		return "0"
	case "text", "character varying", "name":
		return "''"
	case "boolean":
		return "false"
	case "bytea":
		return `'\x'::bytea`
	case "timestamp with time zone":
		return "'epoch'::timestamptz"
	}
	if strings.HasSuffix(typ, "[]") {
		return "'{}'::" + typ
	}
	return "NULL::" + typ
}

// autoExplainPlans runs an engine call with auto_explain loaded and returns
// the plans of the statements it ran.
func autoExplainPlans(ctx context.Context, conn *pgx.Conn, log *NoticeLog, c EngineCall) (plans []planNode, err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) && err == nil {
			err = rbErr
		}
	}()
	simple := pgx.QueryExecModeSimpleProtocol
	statements := []string{
		"LOAD 'auto_explain'",
		"SET LOCAL auto_explain.log_min_duration = 0",
		"SET LOCAL auto_explain.log_nested_statements = on",
		"SET LOCAL auto_explain.log_format = 'json'",
		"SET LOCAL auto_explain.log_level = 'notice'",
		"SET LOCAL enable_seqscan = off",
	}
	if c.List {
		statements = append(statements, "SET LOCAL enable_sort = off")
	}
	for _, s := range statements {
		if _, err := tx.Exec(ctx, s, simple); err != nil {
			return nil, err
		}
	}
	log.take()
	rows, err := tx.Query(ctx, c.SQL, simple)
	if err != nil {
		return nil, err
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, msg := range log.take() {
		_, after, found := strings.Cut(msg, "plan:")
		if !found {
			continue
		}
		p, err := parsePlan(strings.TrimSpace(after))
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, nil
}

# Observability contract: Tool approval denial reasons

**Status**: Design detail of SR-001–SR-003 and research R8; not a new public API.
**Owners**: `internal/domain/approval`, `internal/adapters/storage/postgres`,
`internal/extproc/approval`, and `internal/extproc/server`.
**Binding standards**: `coding-standards/LOGGING.md` and `coding-standards/OPENTELEMETRY.md`.

## Data boundary

The denial reason, raw deny/claim bodies, rendered MCP text, subject token, and client assertion
must never occur in logs, exported log records, span names/attributes/events/status descriptions,
metric names/attributes, diagnostics, or errors. No hashed, encoded, or truncated substitute is
allowed. The reason is not cached or synchronized. The browser emits no console output or
telemetry containing it and never places it in a URL. Response-body logging must remain disabled
on MCP routes in the external proxy, as documented in the gateway guide.

`DenialReason.LogValue`, `String`, and `GoString` return `[REDACTED]`. Adapters explicitly obtain
`Text()` only when persisting or serializing an authorized response. No full approval, request,
response, or delivered-decision struct is logged.

## Audit records

| Event | Level / message | When | Attributes |
|---|---|---|---|
| `approval.tool_approval.decision_delivered` | Info / `approval delivered` | After a successful broker-side delivery claim commits; never for no-match or failure | `approval_id` (typed ApprovalID), `principal` (typed `id.Principal` under the central policy), `agent_id` (typed AgentID), `denial_reason_present` (boolean) |

Use the request-context logger; context enrichment owns trace and caller metadata. The existing
denial audit remains content-free. Validation errors use fixed sentinels. Dependency failures
are classified without logging wrapped response bodies, SQL parameter values, or credential-bearing
errors. Successful claim does not prove that the agent received the response: the name describes
the at-most-once delivery attempt, not an acknowledgement from the MCP client.

## Spans and attribute families

| Span | Scope | Kind | Attributes / behavior |
|---|---|---|---|
| `approval.deliver` (new) | `approval` | Internal | `approval.id` = claimed typed UUID when known; `approval.outcome` = `delivered` or `none` on success; dependency failures use constant error status and classified `error.type` |
| `storage.claim_denial_delivery.tool_approval` (new) | `storage` | Client | Reuse `startStorageSpan`; `db.system=postgresql`, `db.operation=ClaimDenialDelivery`; expected no-match is normal flow; failures use the helper's closed `storage.outcome` / `error.type` classifications |
| Outbound claim HTTP transport (existing instrumentation) | `telemetryhttp` | Client | Reuse the shared `telemetryhttp.NewTransport` construction; allowlisted `http.request.method=POST` and integer `http.response.status_code` only; no URL, headers, body, or user text |
| `extproc.approval.gate` (existing) | `extproc` | Internal | `approval.outcome` remains `deny`, `proceed`, or `elicit`; a delivered denial is `deny`, not an error caused by the user |

All names are constant. The `approval` and `storage` namespaces already exist; no namespace
registration or provider is added. Do not add principal span attributes while the standard's
#356 restriction applies. Do not call `RecordError`, use raw error text as status, or attach
reason presence/content to spans. Caller cancellation is an outcome, not an error; repository
no-match is not an error. Reuse lifecycle trace-context linking where the operation has the
stored approval context available.

## Metrics

No new metric instrument, attribute key, or value is introduced. Existing
`extproc.approval.gate.decisions` (Int64Counter, scope `extproc`) records one decision per gate
evaluation with the unchanged attribute `outcome ∈ {deny, proceed, elicit}`. A delivered denial
increments `outcome=deny`. No approval ID, principal, tool name, reason text, or reason-presence
attribute is added. Retain the existing unit and constructor semantics unless modified code
requires the minimum compliance correction under the binding standard; no metric rename is
planned.

## Required safety tests

- Value-object tests assert `slog`, `%v`, and `%#v` redaction for a distinctive synthetic reason.
- Service/handler tests inject production-constructor JSON buffer loggers and test span recorders;
  successful, invalid, unauthorized, expired, repeated, and storage-failure denial/claim paths
  contain neither the reason nor credentials in any signal or error. Assert audit event and
  permitted typed IDs; no-match and failure emit no successful delivery audit.
- Assert new span names, kinds, statuses, and bounded attributes. Use the existing storage span
  helper's tests and a claim-path test; do not assert status-description wording.
- ExtProc client/gate/server safety tests include 200, 204, malformed and error bodies, cancellation,
  timeout, and transport failure. Inspect logs, span attributes/events/status, and an
  `sdkmetric.ManualReader`; assert the unchanged gate metric attribute set and deny increment.
- Full acceptance Scenario 3.5 exercises successful and rejected browser/broker/ExtProc journeys;
  browser tests fail on console output containing the synthetic reason. Scenario 3.6 verifies
  literal markup display without script execution. Test instrumentation belongs in `_test.go`
  and `tests/`, never new test-only production APIs.

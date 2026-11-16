# Research: Tool approval denial reasons

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Each entry records the decision, its rationale, and the main alternatives. Entries marked
**Proposed** need the maintainer's sign-off through the approval item named in
[plan.md](./plan.md#approval-items). Other entries follow existing repository patterns.

## Current behavior (baseline)

- `POST /api/approvals/{id}/deny` accepts an optional body `{"persistence":"permanent"}` and records
  `status=denied`, `persistence`, and `denied_at` (`internal/adapters/http/handlers/approval/deny_handler.go`,
  `internal/domain/approval/service.go` `DenyApproval`, `internal/domain/storage/tool_approval.go` `Deny`).
- ExtProc creates a pending approval and answers the agent's `tools/call` with an MCP URL
  elicitation (`-32042`, `internal/extproc/server/server.go` `urlElicitationResponse`). The MCP
  client retries the original call after the user acts.
- On the retry, the ExtProc gate (`internal/extproc/approval/gate.go`) proceeds for a matching
  approval and returns HTTP 403 `{"error":"access_denied","error_description":"tool permanently denied"}`
  for a permanent denial of the tool. A **temporary** denial is not matched, so the retry creates a
  new pending approval and a new elicitation (`TestGateDenialDoesNotBlockOtherScopes`, E2E
  `US2-S3`/`US3-S4` in `tests/e2e/approval_api_test.go`). Today the agent therefore never learns
  that a temporary denial happened.
- The sync snapshot (`GET /api/approvals`) maps records to `ToolApprovalSummary` explicitly in
  `sync_handler.go`, so new entity fields do not leak into it by accident.

## R1 — Where the reason lives

**Decision**: Store the reason on the existing `ToolApproval` aggregate as an optional immutable
value object `DenialReason`, set only inside `ToolApproval.Deny`. Persist it in a nullable
`tool_approvals.denial_reason` column.

**Rationale**: The spec defines the reason as part of the decision, not a separate comment
(Domain Model). Setting it inside the same state transition that writes `status=denied` makes the
decision and reason indivisible (FR-005) and immutable (FR-006) without a second write.

**Alternatives considered**: A separate `approval_denial_reasons` table — rejected because it
needs a second write or a join and allows a denied row without its reason during a race. A generic
"comment" entity — rejected as out of scope (no comment history).

## R2 — Validation rules for the reason

**Decision**: `storage.NewDenialReason(raw)` returns no reason for empty or whitespace-only input
(Go `unicode.IsSpace`). Otherwise it keeps `raw` exactly and rejects it when it has more than
500 Unicode code points (`utf8.RuneCountInString`) or contains U+0000. The HTTP adapter decodes JSON
(invalid UTF-8 is not representable in a Go string from `encoding/json`; it becomes U+FFFD, which
counts as one code point). PostgreSQL adds `CHECK (char_length(denial_reason) BETWEEN 1 AND 500)`,
which counts characters, equal to code points in a UTF-8 database.

**Rationale**: FR-002/FR-003 and the Edge Cases fix the limit and absence semantics. PostgreSQL
`TEXT` cannot store U+0000, so accepting it would fail later with a `500`; rejecting it early keeps
the error actionable. No other character class is rejected, because FR-003 requires exact
retention and SR-004 makes display safe instead.

**Alternatives considered**: Count bytes or UTF-16 units — rejected by the spec. Trim leading and
trailing whitespace — rejected; FR-003 retains nonblank text exactly. Reject all control
characters — rejected as rewriting user text without a storage need.

## R3 — Browser and non-browser input contract

**Decision** (part of **API-DENY**, Proposed): `DenyRequest` gains optional `denial_reason`
(`string`, nullable; conditional schema allows at most 500 code points OR all-whitespace
input). Omitted, `null`, empty, or whitespace-only means no reason, normalized before length
validation. A non-string/non-null value, nonblank text over 500 code points, U+0000, or a body
larger than 16 KiB returns
`400` with error code `invalid_denial_reason` and a constant description that never echoes the
text. Body validation, including the reason, runs before the ownership and state checks, as for
`persistence` today; an invalid body never changes the approval. `DenyResponse.data` and `ToolApprovalDetail` gain `denial_reason`,
present only when a reason was recorded. Existing reason-free bodies stay valid.

**Rationale**: One name (`denial_reason`) across request, response, and read models keeps the
field searchable and unambiguous next to other "reason" concepts (OPA `reasons`, logging
`reason`). JSON Schema `maxLength` counts code points; an `anyOf` whitespace-pattern branch
models normalization before nonblank length validation (FR-002/FR-003). Its explicit Unicode
White_Space set matches Go `unicode.IsSpace`, not JavaScript `trim()` or `\\s` (which differ on
U+0085 and U+FEFF). Browser validation uses that same set; 501 whitespace code points are
accepted as absent on both pages, by the handler, and by the domain constructor. The independent
16-KiB body bound accommodates even 500 astral code points escaped as surrogate pairs (≈ 6 KiB
plus the other fields), bounds blank input too, and closes the current unbounded `io.ReadAll` in
`deny_handler.go`.

**Alternatives considered**: `reason` — shorter but collides with existing reason vocabulary.
`422 Unprocessable Entity` (as `invalid_pattern`) — rejected because the limit is a schema
constraint that OpenAPI conditionally expresses; `400` matches the existing `persistence` validation on
this endpoint. A separate `413` for oversized bodies — rejected to avoid a new status code on this
endpoint.

## R4 — How the requesting agent receives the outcome

**Decision** (**DEC-DELIVERY**, **API-MCP**, Proposed): The ExtProc approval gate delivers the
original approval's decision when the agent repeats the **original invocation** — same verified
`(principal, agent_id)`, same tool name, and same canonical arguments hash — after the decision and
before the approval's existing `expires_at`, provided the denial is unrevoked and no matching
approval takes precedence under FR-009. Delivery is **at most once** per denied decision and
is claimed atomically at the broker (R5). The agent receives an MCP `CallToolResult` with
`isError: true`, a fixed text that contains the reason, and structured metadata under
`_meta["io.agentic-identity-broker/tool-approval"]`
([contract](./contracts/extproc-denied-outcome.md)). Later invocations keep today's behavior:
a temporary denial lets a later call create a new approval; a permanent denial returns the existing
`403 tool permanently denied` without a reason (accepted FR-010).

**Rationale**:

- MCP URL elicitation completes out of band; a compliant client retries the same `tools/call`.
  The retry is the only agent-facing channel every MCP client already uses, so no agent changes
  are needed (FR-007 forbids treating internal sync as delivery).
- A tool-execution error (`isError: true`) is shown to the model, unlike a transport 403 or a
  JSON-RPC protocol error, so the agent can adjust its next step — the issue's goal.
- The retry carries no approval identifier. Without one, the gate cannot tell a retry from a later
  identical call. Claiming the delivery once at the broker gives that distinction a single
  authority across ExtProc instances and restarts. This is the only reading that satisfies the
  accepted FR-010 (later identical calls receive no reason) while still delivering the reason.
- The existing `expires_at` bounds the window, so no new retention or configuration is introduced
  (FR-008, Assumptions).

**Consequences**: A lost response is not redelivered to the agent (same at-most-once guarantee as
`once` approvals, ADR 036-extproc-approval-cache-and-gate). The owner can still reread the reason.
A temporary denial now gives the agent one denied outcome on its first eligible retry instead of
a fresh prompt (**DEC-TEMP**); it never overrides a valid matching approval.

**Revocation and overlap** (part of **DEC-DELIVERY**, **DB-048**, Proposed): Revoking a permanent
denial clears `denial_delivery_pending` atomically with clearing its permanent scope, preserving its reason and
any prior delivery timestamp. New claims require `denial_delivery_pending = TRUE`; a claim committed first
may already be in flight and is not recalled. A revoked prior grant is excluded by `approved_at`
as well. A valid matching approval still proceeds without a delivery call when no permanent
denial blocks. An overlapping temporary denial stays unclaimed until an eligible repeat or
expiry. Concurrent changes retain existing freshness-bounded snapshot semantics, not an added
broker serialization on the approved hot path. See Journey 1's overlap variant and Journey 3
Step 7 for both-adapter/gateway cases.

**Main alternatives**: Retain delivery after revocation — rejected because the spec excludes
withdrawn denials. Make temporary denial override valid approvals — rejected because it changes
accepted authorization precedence and the no-broker-call approved path.

**Alternatives considered**:

1. Repeat delivery to every identical retry until `expires_at` — rejected: it contradicts accepted
   FR-010 and Journey 3 Step 6 (later identical calls must not receive the reason) and turns a
   temporary denial into a timed block on identical calls.
2. Agent reads the outcome by approval ID (new agent-facing endpoint or a `_meta` approval ID on
   the retry) — rejected: requires agent changes, a new authentication boundary for agents, and
   is not used by generic MCP clients.
3. MCP `notifications/elicitation/complete` — rejected: ExtProc sees request/response bodies only
   and cannot originate server notifications on the agent's session.
4. JSON-RPC error object with the reason in `error.data` — rejected: many MCP clients surface
   protocol errors to the host, not to the model.
5. Put the reason in the sync snapshot and deliver locally — rejected: broadcasts personal text to
   every ExtProc cache, and per-instance memory cannot enforce at-most-once across instances.

## R5 — Delivery claim endpoint and authentication

**Decision** (**API-DELIVER**, Proposed): New gateway endpoint `POST /api/approvals/deliveries`
with body `{"tool_name": "...", "arguments": {...}}`. It requires the same dual authentication as
`POST /api/approvals` (subject token + `X-Client-Assertion`). The broker derives principal and agent
from the authenticated context, computes `arguments_hash` with the same function as creation, and
atomically claims the newest undelivered, never-approved denied approval of that pair, tool, and
hash whose `expires_at` is in the future and `denial_delivery_pending` is true. `200` returns the claimed decision with
`denial_reason` when present; `204` means nothing is deliverable. Errors: `400` (malformed body),
`401` (missing or invalid credentials), `403` (agent-subject token,
ADR 045-agent-subject-impersonation decision 6a), `503` (authentication unavailable), and `500`
for claim/storage timeouts, connection failures, or unexpected claim/service failures. The `500`
uses the existing `ApprovalError` envelope with constant `error: internal_error` and
`message: failed to deliver approval decision`, following the create handler's internal-error
convention. It never echoes dependency errors or reason text. A committed-but-lost response
remains claimed; no error can reset delivery state or enable redelivery. Service/handler/client
tests cover each failure class and gateway fail-closed behavior. Among several candidates the
newest `denied_at` wins.

**Rationale**: The endpoint returns user-authored text to the gateway that minted the prompt, so it
takes the strongest existing boundary: proof of both the user/agent context and the trusted
gateway caller (ADR 018-approval-endpoint-auth-boundaries rationale for create). Correlating on the broker keeps the hash
computation in one place and needs one call; ExtProc only pre-filters with synced metadata (R6).
`204` makes "no delivery" a normal outcome instead of an error.

**Alternatives considered**: `POST /api/approvals/{id}/deliver` — rejected because the gate would
have to call once per candidate and replicate the arguments-hash function to pick the record.
Subject-token-only authentication (as `consume`) — rejected because this read returns
user-authored content, which should require the trusted gateway proof. Reusing `consume` —
rejected because `consumed=true` removes the record from sync and would drop permanent-denial
enforcement.

## R6 — Sync snapshot additions and gate pre-filter

**Decision** (**API-SYNC**, Proposed): `ToolApprovalSummary` gains `expires_at` (always) and
nullable `denial_delivered_at` and required boolean `denial_delivery_pending`. The reason itself is never in the snapshot. The ExtProc gate calls
the delivery endpoint only when the pair's fresh snapshot holds a candidate: `status=denied`,
`approved_at` and `denial_delivered_at` absent, `denial_delivery_pending` true, `expires_at` in the future, and `tool_name`
equal to the invoked tool. The delivery claim updates the row, so the existing sync trigger bumps
the version and every ExtProc instance drops the candidate.

**Gate order** (extends ADR 036-extproc-approval-cache-and-gate, does not change its authorization outcomes):

1. Fresh permanent denial for the tool → if a candidate exists, claim; `200` → denied outcome with
   reason; `204` → existing `tool permanently denied`.
2. Fresh approved match → existing proceed/consume path; no claim.
3. Otherwise authoritative read (existing), then steps 1–2 again, then claim if a candidate exists;
   `200` → denied outcome; `204` → existing create/elicit path.
4. A claim that fails (transport error, `5xx`, unexpected status, malformed body) denies the call
   with `approval decision could not be delivered` and creates no approval (fail closed).

**Rationale**: The pre-filter keeps the broker off the hot path: a candidate exists only for a few
minutes after a denial. A delivered or expired denial never triggers a call again.

**Alternatives considered**: Compute the arguments hash in ExtProc and compare locally — rejected:
duplicates broker canonicalization outside the shared `toolpattern` package (ADR 035-shared-tool-pattern-matching boundary)
and risks number-format drift. Always call the broker on a miss — rejected as unnecessary load.

## R7 — Persistence

**Decision** (**DB-048**, Proposed): Atomic migration
`048_add_tool_approval_denial_reason.{up,down}.sql`:

- `ALTER TABLE tool_approvals ADD COLUMN denial_reason TEXT NULL, ADD COLUMN denial_delivered_at TIMESTAMPTZ NULL, ADD COLUMN denial_delivery_pending BOOLEAN NOT NULL DEFAULT FALSE`;
- `CHECK (denial_reason IS NULL OR (status = 'denied' AND char_length(denial_reason) BETWEEN 1 AND 500))`;
- `CHECK (denial_delivered_at IS NULL OR status = 'denied')`;
- `CHECK (NOT denial_delivery_pending OR (status = 'denied' AND approved_at IS NULL AND denial_delivered_at IS NULL))`;
- partial index `idx_tool_approvals_deliverable_denials ON (principal, agent_id, tool_name, arguments_hash) WHERE status = 'denied' AND approved_at IS NULL AND denial_delivered_at IS NULL AND denial_delivery_pending`.

Existing rows get `NULL` reason/delivery time and false delivery-pending (no backfill, DB-001).
Historical decisions are not newly delivered, because their temporary-denial and revoked-denial
states cannot be distinguished reliably. New `Deny` writes set delivery-pending true; claim and
permanent revocation clear it atomically. DOWN drops the index, constraints,
and columns; it discards recorded reasons, which is documented in the migration test. The reason is
stored as plaintext, like `arguments` and `description`.

**Rationale**: Two nullable columns, a delivery-pending flag, and checks distinguish deliverable
user denials from claimed, revoked, and historical records in both adapters. The flag remains
false after permanent scope is cleared; setting `denial_delivered_at` on revocation would falsely
report a delivery.
The partial index serves the claim query. The existing row trigger already publishes sync
changes for the new updates.

**Alternatives considered**: Application-layer encryption with the envelope encryption stack —
rejected: tool `arguments` carry comparable personal data in plaintext today, the spec makes no
new encryption promise, and ADR 008-encryption-context-optimization's single-subject encryption
context has no approval subject. A delivery-state enum column — rejected; the timestamp carries
claim time, while the pending flag safely excludes historical revocations and claim-before-revoke
races. A new revocation timestamp alone cannot exclude ambiguous historical records without
an extra eligibility bit or invented backfill. Reuse the delivery
timestamp to suppress revocation — rejected because it invents a delivery that did not happen.

## R8 — Logging, tracing, and metrics

**Decision**: The reason never appears in a log record, span (name, attribute, event, status),
metric, or error string in the broker, ExtProc, or browser. Concretely:

- `DenialReason` implements `slog.LogValuer`, `fmt.Stringer`, and `fmt.GoStringer` returning
  `[REDACTED]` (LOGGING.md §6.4), so an accidental `slog.Any` cannot leak it.
- Validation errors are sentinel errors with constant text.
- The existing `approval denied` log record is unchanged. The delivery claim writes a new audit
  record `approval delivered` with `event=approval.tool_approval.decision_delivered`,
  `approval_id`, `principal` (`id.Principal`), `agent_id`, and `denial_reason_present` (bool).
- New span `approval.deliver` (scope `approval`, kind internal) with `approval.id` and
  `approval.outcome` (`delivered` | `none`). ExtProc keeps the gate span and its
  `extproc.approval.gate.decisions` metric value set unchanged; a delivered denial counts as
  `deny`.
- LOGGING.md §6.1 "Never Log" gains the row "User-authored free text: tool approval denial
  reasons", with the same exclusion applying to telemetry through OPENTELEMETRY.md §6.1.
- The SPA writes no console output containing the reason and never puts it in a URL.

**Rationale**: SR-001/SR-002 and Principle XV. Adding a constant boolean keeps the decision
auditable without content.

**Alternatives considered**: Hash or truncate the reason in logs — forbidden by SR-001.

## R9 — Gateway logging outside this repository

**Decision**: ExtProc is this repository's gateway component and is tested for leaks. The
agentgateway proxy forwards the immediate response body; its default access log records no
bodies. The token-exchange gateway guide states that operators must not enable body logging on
MCP responses, because denied outcomes may contain user text.

**Rationale**: The repository cannot control third-party proxy configuration; documenting the
requirement completes SR-001 for the deployed flow.

**Alternatives considered**: A Docker E2E that scrapes agentgateway logs — rejected as testing
third-party defaults rather than this feature.

## R10 — Frontend interaction

**Decision**: Both denial surfaces (`ApprovalReviewPage` and the pending card on
`ToolAuthorizationsPage`) open a shared denial form built from design-system `TextArea`, `Button`,
and status components. The form shows the optional label, live `n / 500 characters` count
(counted with `Array.from(text).length`), and the sharing notice. It normalizes all-whitespace
input using R3's explicit Unicode set before applying the nonblank limit, never disabling submit
just because 501 whitespace code points are present, keeps the text on validation
error, and discards it on cancel. **Deny** and **Deny permanently** both open the form with their
existing scope. The confirmation renders the reason as a React text node (no
`dangerouslySetInnerHTML`, no Markdown) inside a read-only block and omits the heading when there is
no reason.

**Rationale**: FR-001, FR-011, FR-012, SR-004, and the design-system rules. A feature-local
component composed from existing primitives avoids a new design-system component.

**Alternatives considered**: Native `window.prompt` — rejected (inaccessible, unstyled). Adding a
new design-system component — not needed for two call sites.

## R11 — End-user API version

**Decision** (**API-VERSION**, Proposed): Bump `api/enduser/openapi.yaml` `info.version` from
`1.2.0` to `1.3.0`.

**Rationale**: All changes are additive and backward compatible: optional request field, optional
response fields, a new endpoint. Existing clients keep working.

**Alternatives considered**: `2.0.0` — not warranted; no field is removed or changed in type.

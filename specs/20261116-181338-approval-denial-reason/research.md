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
- `RevokePermanent` sets `status=denied`, `persistence=NULL`, and overwrites `denied_at` with the
  revocation time, for both permanent grants and permanent denials. Nothing records that a
  revocation happened, so a revoked permanent denial looks like a temporary denial
  (`internal/adapters/storage/{memory,postgres}/tool_approval_repository.go`). The revoke
  response reuses the `DenyResponse` schema.
- `expires_at` is the review deadline of a pending approval (`approvals.pending_ttl`, default
  `10m`).
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

**Single authority** (architecture review F2): `storage.NewDenialReason` is the only place that
decides absence, the code-point limit, and U+0000. The deny handler, the OpenAPI `anyOf` pattern,
the SPA form, and the PostgreSQL CHECK never decide whether text is a valid reason. The handler
passes the raw string to the domain; the contract pattern and browser checks are advisory
pre-checks that help clients; the CHECK is a storage backstop for invariant 1. A divergence in
an advisory check can only produce a domain `400`, never a stored invalid reason.

**Alternatives considered**: Count bytes or UTF-16 units — rejected by the spec. Trim leading and
trailing whitespace — rejected; FR-003 retains nonblank text exactly. Reject all control
characters — rejected as rewriting user text without a storage need.

## R3 — Browser and non-browser input contract

**Decision** (part of **API-DENY**, Proposed): `DenyRequest` gains optional `denial_reason`
(`string`, nullable; conditional schema allows at most 500 code points OR all-whitespace
input). Omitted, `null`, empty, or whitespace-only means no reason, normalized before length
validation. A non-string/non-null value, nonblank text over 500 code points, U+0000, or a body
larger than 16 KiB returns `400` with error code `invalid_denial_reason` and a constant
description that never echoes the text. Reason validation runs before the ownership and state
checks, as for `persistence` today; an invalid body never changes the approval.
`DenyResponse.data` and `ToolApprovalDetail` gain `denial_reason`, present only when a reason
was recorded. Existing reason-free bodies stay valid.

**Layering** (architecture review F2; Principle VI): `deny_handler.go` only

1. bounds the body at 16 KiB (transport concern),
2. decodes JSON and checks that `denial_reason` is a string or `null` (input parsing),
3. passes the raw string unchanged in `approval.DenyRequest.Reason` (`*string`), and
4. maps the domain sentinels `storage.ErrDenialReasonTooLong` and `storage.ErrDenialReasonInvalid`
   to `400 invalid_denial_reason`.

`approval.Service.DenyApproval` calls `storage.NewDenialReason` before it loads the approval,
so the observable order (reason errors before ownership and state errors) stays as specified.
The handler never trims, counts, or inspects the text.

**Rationale**: One name (`denial_reason`) across request, response, and read models keeps the
field searchable and unambiguous next to other "reason" concepts (OPA `reasons`, logging
`reason`). JSON Schema `maxLength` counts code points; an `anyOf` whitespace-pattern branch
models normalization before nonblank length validation (FR-002/FR-003) for client generators.
Its explicit Unicode White_Space set matches Go `unicode.IsSpace`, not JavaScript `trim()` or
`\\s` (which differ on U+0085 and U+FEFF). The browser pre-check uses that same set; 501
whitespace code points are accepted as absent on both pages and by the domain constructor. The
independent 16-KiB body bound accommodates even 500 astral code points escaped as surrogate
pairs (≈ 6 KiB plus the other fields), bounds blank input too, and closes the current unbounded
`io.ReadAll` in `deny_handler.go`.

**Body bound scope**: The bound applies to the deny endpoint, whose body this feature changes.
Create, approve, scope-preview, and the new denial-delivery endpoint keep today's unbounded
decoding. The denial-delivery body repeats the tool arguments that create already accepted, so a
smaller bound there would fail closed on legitimate repeats. A shared limit for every approval
route is outside this feature and is tracked as the follow-up topic
"Shared approval request body limit" ([plan.md](./plan.md#follow-up-topics)).

**Alternatives considered**: `reason` — shorter but collides with existing reason vocabulary.
`422 Unprocessable Entity` (as `invalid_pattern`) — rejected because the limit is a schema
constraint that OpenAPI conditionally expresses; `400` matches the existing `persistence`
validation on this endpoint. A separate `413` for oversized bodies — rejected to avoid a new
status code on this endpoint. Validate in the handler as well as the domain — rejected: two
authorities for one rule can drift (architecture review F2).

## R4 — How the requesting agent receives the outcome

**Decision** (**DEC-DELIVERY**, **API-MCP**, Proposed): The ExtProc approval gate performs a
**Denial Delivery** when the agent repeats the **original invocation** — same verified
`(principal, agent_id)`, same tool name, and same canonical arguments hash — after the denial
and before the approval's **denial delivery deadline** (R13), provided the denial is unrevoked
and no matching approval takes precedence under FR-009. Delivery is **at most once** per denied
decision and is claimed atomically at the broker (R5). The agent receives the **Denied Outcome**:
an MCP `CallToolResult` with `isError: true`, a fixed text that contains the reason, and
structured metadata under `_meta["io.agentic-identity-broker/tool-approval"]`
([contract](./contracts/extproc-denied-outcome.md)). Later invocations keep today's behavior:
a temporary denial lets a later call create a new approval; a permanent denial returns the
existing `403 tool permanently denied` without a reason (accepted FR-010).

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
- A fixed window measured from the denial bounds delivery without a new retention period or
  configuration key (FR-008, Assumptions, R13).

**Consequences**: A lost response is not redelivered to the agent (same at-most-once guarantee as
`once` approvals, ADR 036-extproc-approval-cache-and-gate). The owner can still reread the reason.
A temporary denial now gives the agent one Denied Outcome on its first eligible retry instead of
a fresh prompt (**DEC-TEMP**); it never overrides a valid matching approval. A stale ExtProc
snapshot can still show a denial candidate after a concurrent grant; the claim then denies a call
that a fresher snapshot would have allowed. This fails closed and stays inside the existing
freshness bound of ADR 036-extproc-approval-cache-and-gate.

**Revocation and overlap** (part of **DEC-DELIVERY** and **DEC-REVOKE**, Proposed): Revoking a
permanent denial records `revoked_at` atomically with clearing its permanent scope, preserving
its `denied_at`, reason, and any prior delivery timestamp (R7). A revoked denial is never
deliverable; a claim committed first may already be in flight and is not recalled. A revoked
prior grant has no denial delivery deadline and is never deliverable. A valid matching approval
still proceeds without a delivery call when no permanent denial blocks. An overlapping temporary
denial stays unclaimed until an eligible repeat or its deadline. Concurrent changes retain
existing freshness-bounded snapshot semantics, not an added broker serialization on the approved
hot path. See Journey 1's overlap variant and Journey 3 Step 7 for both-adapter/gateway cases.

**Main alternatives**: Retain delivery after revocation — rejected because the spec excludes
withdrawn denials. Make temporary denial override valid approvals — rejected because it changes
accepted authorization precedence and the no-broker-call approved path.

**Alternatives considered**:

1. Repeat delivery to every identical retry until the deadline — rejected: it contradicts accepted
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

## R5 — Denial-delivery claim endpoint, authentication, and relation to `once` consumption

**Decision** (**API-DELIVER**, **DEC-CLAIM**, Proposed): New gateway endpoint
`POST /api/approvals/denial-deliveries` (operation `claimDenialDelivery`) with body
`{"tool_name": "...", "arguments": {...}}`. It requires the same dual authentication as
`POST /api/approvals` (subject token + `X-Client-Assertion`). The broker derives principal and
agent from the authenticated context, computes `arguments_hash` with the same function as
creation, and claims the newest deliverable denial of that pair, tool, and hash (deliverability
is the domain predicate in [data-model.md](./data-model.md#invariants)). `200` returns the
claimed denial with `denial_reason` when present and `denial_delivered_at`; `204` means nothing
is deliverable. Errors: `400` (malformed body), `401` (missing or invalid credentials), `403`
(agent-subject token, ADR 045-agent-subject-impersonation decision 6a), `503` (authentication
unavailable), and `500` for claim/storage timeouts, connection failures, or unexpected
claim/service failures. The `500` uses the existing `ApprovalError` envelope with constant
`error: internal_error` and `message: failed to deliver approval denial`, following the create
handler's internal-error convention. It never echoes dependency errors or reason text. A
committed-but-lost response remains claimed; no error can reset delivery state or enable
redelivery. Among several candidates the newest `denied_at` wins, then the lowest ID.

**Claim mechanics** (architecture review F6): The domain service owns the claim. It lists
candidates for the invocation key through the port, selects the newest one for which
`ToolApproval.IsDenialDeliverable(now)` holds, applies `ToolApproval.ClaimDenialDelivery(now)`,
and persists with a compare-and-set `MarkDenialDelivered` that succeeds only while the two
mutable predicate inputs (`denial_delivered_at`, `revoked_at`) are still `NULL`. A lost
compare-and-set (concurrent claim or revocation) moves to the next candidate or returns none.

**A second at-most-once claim, next to `once` consumption** (architecture review F1): ADR
036-extproc-approval-cache-and-gate already has one at-most-once mechanism: ExtProc reserves a
cached `once` approval locally and then calls `POST /api/approvals/{id}/consume` with the subject
token. Denial Delivery is a second, deliberately separate mechanism:

| Aspect | `once` consumption (ADR 036) | Denial Delivery (this feature) |
|---|---|---|
| Record | Approved, `persistence=once` | Newly denied, inside its delivery deadline |
| Lookup | By approval ID from the snapshot | By invocation key; the broker computes the hash |
| Returns | Nothing user-authored | User-authored reason |
| Authentication | Subject token | Subject token + client assertion (create's boundary) |
| Effect on sync | `consumed=true` removes the record | Record stays; `denial_deliverable_until` becomes absent |
| Local reservation | ExtProc CAS before the call | None; the broker claim is the only authority |

Each difference follows from the record kind: consumption authorizes execution of an approval
already in the snapshot, while Denial Delivery hands user-authored text to the gateway that
minted the prompt and must keep permanent-denial enforcement in sync. The ADR draft records both
mechanisms as the approval system's two at-most-once claims.

**Rationale**: The endpoint returns user-authored text to the gateway that minted the prompt, so it
takes the strongest existing boundary: proof of both the user/agent context and the trusted
gateway caller (ADR 018-approval-endpoint-auth-boundaries rationale for create). Correlating on the
broker keeps the hash computation in one place and needs one call; ExtProc only pre-filters with
synced metadata (R6). `204` makes "no delivery" a normal outcome instead of an error.

**Alternatives considered**:

- **Fold the claim into `POST /api/approvals`** (main alternative, architecture review F1):
  create would return `200` with a deliverable Denied Outcome instead of `201`/`200` with a
  pending approval. Rejected because (1) the permanent-denial path never calls create today, so
  create would have to run while a permanent denial blocks, mixing enforcement with creation and
  its rate limits; (2) create's existing `200` already means "existing pending approval returned",
  so a deployed ExtProc during a rolling upgrade would read a denied record as pending and elicit
  again; (3) an idempotent create would gain a one-time side effect. A separate additive endpoint
  keeps create's contract unchanged.
- `POST /api/approvals/{id}/deliver` — rejected because the gate would have to call once per
  candidate and replicate the arguments-hash function to pick the record.
- Subject-token-only authentication (as `consume`) — rejected because this read returns
  user-authored content, which should require the trusted gateway proof.
- Reusing `consume` — rejected because `consumed=true` removes the record from sync and would
  drop permanent-denial enforcement.
- A single atomic `UPDATE … WHERE <deliverability>` in each adapter — rejected (architecture
  review F6): it puts the deliverability rule in adapter SQL and memory filters, and the domain
  service would only delegate.

## R6 — Sync snapshot addition and gate pre-filter

**Decision** (**API-SYNC**, Proposed): `ToolApprovalSummary` gains one nullable field,
`denial_deliverable_until`. The sync handler sets it from the domain method
`ToolApproval.DenialDeliverableUntil()`: the record's denial delivery deadline while the denial
is unclaimed and unrevoked, and absent otherwise (pending, approved, claimed, revoked, revoked
grant, or historical). The reason itself is never in the snapshot. The ExtProc gate calls the
denial-delivery endpoint only when the pair's fresh snapshot holds a candidate: `status=denied`,
`denial_deliverable_until` later than now, and `tool_name` equal to the invoked tool. A claim or
revocation updates the row, so the existing sync trigger bumps the version and every ExtProc
instance drops the candidate.

**Rationale**: One derived field gives ExtProc exactly what it needs and nothing that can
contradict another field (architecture review data-authority and coupling notes). The broker
domain stays the only author of deliverability; ExtProc only compares a timestamp and a tool
name, without importing broker domain code (ADR 035-shared-tool-pattern-matching).

**Gate order** (extends ADR 036-extproc-approval-cache-and-gate, does not change its authorization outcomes):

1. Fresh permanent denial for the tool → if a candidate exists, claim; `200` → Denied Outcome
   with reason; `204` → existing `tool permanently denied`.
2. Fresh approved match → existing proceed/consume path; no claim.
3. Otherwise authoritative read (existing), then steps 1–2 again, then claim if a candidate exists;
   `200` → Denied Outcome; `204` → existing create/elicit path.
4. A claim that fails (transport error, `5xx`, unexpected status, malformed body) denies the call
   with `approval denial could not be delivered` and creates no approval (fail closed).

**Alternatives considered**: Synchronize the raw facts (`expires_at`, `denial_delivered_at`,
and a pending flag) and let ExtProc combine them — rejected: the facts overlap and ExtProc would
hold a second copy of the deliverability rule (architecture review F6). Compute the arguments
hash in ExtProc and compare locally — rejected: duplicates broker canonicalization outside the
shared `toolpattern` package (ADR 035-shared-tool-pattern-matching boundary) and risks
number-format drift. Always call the broker on a miss — rejected as unnecessary load.

## R7 — Persistence and explicit revocation

**Decision** (**DB-048**, **DEC-REVOKE**, Proposed): Atomic migration
`048_add_tool_approval_denial_reason.{up,down}.sql`:

- `ALTER TABLE tool_approvals ADD COLUMN denial_reason TEXT NULL, ADD COLUMN denial_delivery_deadline TIMESTAMPTZ NULL, ADD COLUMN denial_delivered_at TIMESTAMPTZ NULL, ADD COLUMN revoked_at TIMESTAMPTZ NULL`;
- `CHECK (denial_reason IS NULL OR (status = 'denied' AND char_length(denial_reason) BETWEEN 1 AND 500))`;
- `CHECK (denial_delivery_deadline IS NULL OR (status = 'denied' AND approved_at IS NULL))`;
- `CHECK (denial_delivered_at IS NULL OR denial_delivery_deadline IS NOT NULL)`;
- `CHECK (revoked_at IS NULL OR status = 'denied')`;
- index `idx_tool_approvals_denial_delivery_candidates ON (principal, agent_id, tool_name, arguments_hash, denial_delivery_deadline) WHERE denial_delivery_deadline IS NOT NULL`.

Each business fact has one column:

| Fact | Column | Written by |
|---|---|---|
| Time of the denial decision | `denied_at` (existing) | `Deny`; revocation of a denial no longer overwrites it |
| Reason | `denial_reason` | `Deny` only |
| End of the Denial Delivery window | `denial_delivery_deadline` | `Deny` only: `denied_at + DenialDeliveryWindow` (R13) |
| Denial Delivery claimed | `denial_delivered_at` | `ClaimDenialDelivery` only |
| Permanent approval or denial revoked | `revoked_at` | `RevokePermanent` only |
| Review deadline of a pending approval | `expires_at` (existing, unchanged) | Create |

`RevokePermanent` sets `revoked_at` and keeps its existing status and scope effect. It writes
`denied_at = COALESCE(denied_at, revoked_at)`: a revoked denial keeps its decision time, while a
revoked grant (which has no denial time) still records the revocation time in `denied_at` as
today. Existing rows get `NULL` in every new column (no backfill, DB-001), so no historical record
is deliverable and no historical revocation is inferred. DOWN drops the index, constraints, and
columns; it discards recorded reasons, deadlines, delivery times, and revocation times, which the
migration test documents. The reason is stored as plaintext, like `arguments` and `description`.

**Rationale** (architecture review F3/F4): An explicit `revoked_at` makes revocation a recorded
fact instead of an inference from `persistence=NULL` or a cleared flag, and preserving `denied_at`
keeps the reason next to its decision time on owner reads. A NULL deadline marks every record
that never became deliverable (pending, approved, revoked grants, and all historical rows), so no
separate eligibility flag is needed. The index serves the candidate lookup only; it is a storage
access path, not a second statement of deliverability. The existing row trigger already publishes
sync changes for the new updates.

**Alternatives considered**: Keep the `denial_delivery_pending` flag as the only revocation
marker and document it (main alternative, architecture review F3) — rejected: revocation stays
implicit, the flag's name suggests delivery in flight, and owner reads show the reason next to the
revocation time. Keep overwriting `denied_at` on revocation and add only `revoked_at` — rejected:
`denied_at` would still stop being the decision time. Application-layer encryption with the
envelope encryption stack — rejected: tool `arguments` carry comparable personal data in
plaintext today, the spec makes no new encryption promise, and ADR
008-encryption-context-optimization's single-subject encryption context has no approval subject.
A delivery-state enum column — rejected; the timestamps carry the facts directly. Reuse the
delivery timestamp to suppress revocation — rejected because it invents a delivery that did not
happen.

## R8 — Logging, tracing, and metrics

**Decision**: The reason never appears in a log record, span (name, attribute, event, status),
metric, or error string in the broker, ExtProc, or browser. Concretely:

- `DenialReason` implements `slog.LogValuer`, `fmt.Stringer`, and `fmt.GoStringer` returning
  `[REDACTED]` (LOGGING.md §6.4), so an accidental `slog.Any` cannot leak it.
- Validation errors are sentinel errors with constant text.
- The existing `approval denied` log record is unchanged. A committed Denial Delivery claim writes
  a new audit record `approval denial delivered` with
  `event=approval.tool_approval.denial_delivered`, `approval_id`, `principal` (`id.Principal`),
  `agent_id`, and `denial_reason_present` (bool).
- New span `approval.claim_denial_delivery` (scope `approval`, kind internal) with `approval.id`
  and `approval.outcome` (`delivered` | `none`). ExtProc keeps the gate span and its
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
input using R3's explicit Unicode set before applying the nonblank limit as an advisory
pre-check (the broker's `storage.NewDenialReason` stays the authority, R2), never disabling submit
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

**Rationale**: The changes are additive: optional request field, optional response fields, one
sync field, a new endpoint. Existing clients keep working. One value changes meaning: after this
feature, the revoke response and owner reads of a revoked permanent **denial** report its original
`denied_at` instead of the revocation time, which moves to the new `revoked_at` (**DEC-REVOKE**).
The field keeps its name, type, and documented meaning ("time of denial"); revoked grants are
unchanged.

**Alternatives considered**: `2.0.0` — not warranted; no field is removed or changed in type.

## R12 — Delivery vocabulary

**Decision** (architecture review F5): **Denial Delivery** is the one canonical term for
claiming and returning a newly denied approval's outcome to the requesting agent. Only denials are
ever delivered, so no name uses "decision delivery". Every name derives from it:

| Layer | Name |
|---|---|
| HTTP path / operation | `POST /api/approvals/denial-deliveries` / `claimDenialDelivery` |
| Schemas | `DenialDeliveryClaimRequest`, `DenialDeliveryClaimResponse` |
| Claim timestamp (column, claim response) | `denial_delivered_at` |
| Window end (column) | `denial_delivery_deadline` |
| Sync field | `denial_deliverable_until` (present only while still deliverable) |
| Domain | `DenialDeliveryWindow`, `ToolApproval.IsDenialDeliverable`, `DenialDeliverableUntil`, `ClaimDenialDelivery`, `ErrDenialNotDeliverable` |
| Service / port | `approval.Service.ClaimDenialDelivery`, `ports.ToolApprovalDenialDeliveryRepository` |
| ExtProc | `Broker.ClaimDenialDelivery`, `DeliveredDenial`, `Outcome.DeliveredDenial`, `Cache.HasDeliverableDenial` |
| Audit / span | `approval.tool_approval.denial_delivered` / `approval.claim_denial_delivery` |
| Constant errors | `failed to deliver approval denial` (broker `500`), `approval denial could not be delivered` (ExtProc `403`) |
| Agent-facing payload | **Denied Outcome** (`_meta["io.agentic-identity-broker/tool-approval"]`) |

`denial_deliverable_until` replaces the earlier `denial_delivery_pending`, whose name suggested a
delivery in flight while it meant "still eligible".

**Alternatives considered**: Keep the broader "Decision Delivery" term and define in the glossary
why it covers only denials — rejected: a name that promises more than it covers invites a second
meaning later.

## R13 — Denial delivery window

**Decision** (**DEC-WINDOW**, Proposed): `ToolApproval.Deny` records
`denial_delivery_deadline = denied_at + DenialDeliveryWindow`, where `DenialDeliveryWindow` is a
fixed domain constant of 10 minutes. The deadline is stored per record, so a later change of the
constant never moves an already-recorded deadline. The review deadline `expires_at` stays
unchanged and no longer bounds delivery. This is a product default, like the 500-code-point
limit, not a configuration key.

**Rationale** (architecture review F4): The review deadline and the delivery window are different
facts. Reusing `expires_at` gave a denial made late in review almost no delivery time, and a change
to `approvals.pending_ttl` would silently change delivery. Ten minutes matches the default review
TTL and covers an MCP client's retry after the user acts, while keeping the window in which a
reason can reach the agent short.

**Alternatives considered**:

- Keep reusing `expires_at` and document the coupling and the late-denial edge case (architecture
  review's alternative) — rejected for the two reasons above.
- A new configuration key such as `approvals.denial_delivery_window` — rejected for this feature:
  it adds a documented key, environment variable, Helm value, and examples for a value with no
  known operator need. A later feature can add it without a schema change because the deadline is
  stored per record.

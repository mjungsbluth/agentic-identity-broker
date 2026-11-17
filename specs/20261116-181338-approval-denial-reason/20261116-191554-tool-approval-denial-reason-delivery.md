# ADR 20261116-191554-tool-approval-denial-reason-delivery: Tool Approval Denial Reasons and Denied-Outcome Delivery

**Status**: Proposed
**Date**: 2026-11-16
**Feature**: `specs/20261116-181338-approval-denial-reason`

---

## Context

A user can deny a pending tool approval, but cannot say why. The requesting agent learns little:
after a permanent denial, its retry receives HTTP 403 `tool permanently denied`; after a temporary
denial, its retry is not matched and creates a new approval prompt
(`internal/extproc/approval/gate.go`). The feature lets the denying user add an optional reason of
at most 500 Unicode code points and delivers it to the requesting agent with the denied outcome.

The accepted clarification (FR-010) shares a reason only with the original approval's denied
outcome, never with later invocations blocked by a permanent denial. MCP URL elicitation completes
out of band, and the agent's retry of the same `tools/call` carries no approval identifier.

Relevant accepted decisions: `018-approval-endpoint-auth-boundaries` (per-endpoint approval
authentication), `036-extproc-approval-cache-and-gate` (broker authority, cache freshness, gate
order, at-most-once `once` consumption), `014-long-poll-listen-notify` (sync),
`035-shared-tool-pattern-matching` (the only broker-domain import in ExtProc), and
`045-agent-subject-impersonation` (agent-subject tokens rejected on subject-token approval
endpoints, decision 6a).

Today `RevokePermanent` overwrites `denied_at` and clears `persistence`; nothing records that a
revocation happened. `expires_at` is the review deadline of a pending approval.

**Vocabulary.** **Denial Delivery** is the at-most-once claim and return of a newly denied
approval's outcome to the requesting agent. The **Denied Outcome** is the agent-facing MCP payload
it produces. Only denials are delivered.

## Decision

### 1. The reason belongs to the denial

`ToolApproval` gains an optional immutable `DenialReason` value object, set only in the
pending → denied transition and stored in `tool_approvals.denial_reason`. Empty or whitespace-only
input means no reason, normalized before nonblank length validation (including 501 whitespace
code points within the independent 16-KiB body bound); other input is kept exactly. The domain
constructor `storage.NewDenialReason` is the only authority for these rules. The deny handler
bounds the body, checks the JSON type, and passes the raw text to the domain; the OpenAPI pattern
and the browser check are advisory, and store constraints (denied status, 1–500 characters) are a
backstop.

### 2. Denial Delivery happens on the agent's repeat of the original invocation, at most once

The ExtProc approval gate performs a Denial Delivery when the agent repeats the original
invocation (same verified principal and agent, tool name, and canonical arguments hash) before
the denial's **denial delivery deadline**, while the denial is unrevoked and no valid matching
approval takes precedence under FR-009. `Deny` stores the deadline as `denied_at` plus a fixed
10-minute `DenialDeliveryWindow`, independent of the review deadline `expires_at`; a later change
of the constant never moves a stored deadline. The broker claims the delivery through
`POST /api/approvals/denial-deliveries` and records `denial_delivered_at`. Each denial is
delivered at most once.

Deliverability is one domain predicate, `ToolApproval.IsDenialDeliverable(now)`: a deadline exists
and has not passed, and the denial is unclaimed and unrevoked. The domain service lists
candidates for the invocation key through the port, applies the predicate and the aggregate's
`ClaimDenialDelivery` transition, and persists with a compare-and-set that succeeds only while
the claim and revocation timestamps are still empty. Adapters narrow the candidate search and
guard concurrency; they do not decide deliverability.

A later invocation, including an identical one after delivery, follows the existing behavior: a temporary denial lets it create a new approval, and a permanent denial blocks
it with `tool permanently denied` and no reason.

This adds a step to the gate order of `036-extproc-approval-cache-and-gate` without changing any
authorization outcome of that ADR: a fresh permanent denial still blocks, a fresh approval still
proceeds without a delivery call, and creation still follows the authoritative read. The gate calls
the broker for delivery only when the fresh snapshot shows an undelivered, unrevoked denial for
the invoked tool inside its window. A temporary denial never overrides a matching approval;
it remains unclaimed until an eligible repeat or its deadline. Concurrent grants or revocations retain
the existing snapshot freshness semantics, without a new globally serialized precedence check.
A failed delivery call denies the invocation with `approval denial could not be delivered` and
creates no approval. Claim/storage timeouts,
connection failures, or unexpected service failures return `500` in the existing `ApprovalError`
envelope with constant `internal_error` and `failed to deliver approval denial`, never raw
errors or reason text. A committed-but-lost claim is not reset.

### 3. Denial-delivery endpoint authentication and relation to `once` consumption

`POST /api/approvals/denial-deliveries` requires the subject token and the client assertion, the same
boundary that `018-approval-endpoint-auth-boundaries` assigns to `POST /api/approvals`. The broker
derives principal and agent from the authenticated context; the request cannot name another pair.
Agent-subject tokens are rejected as on the other subject-token approval endpoints; this extends
the endpoint list of `045-agent-subject-impersonation` decision 6a with one endpoint.

Denial Delivery is the approval system's second at-most-once claim, next to the `once`
consumption of `036-extproc-approval-cache-and-gate`. They stay separate because the records and
risks differ: consumption is looked up by approval ID, returns nothing user-authored, needs only
the subject token, and removes the record from sync; Denial Delivery is looked up by invocation
key with a broker-computed hash, returns user-authored text, needs the gateway's client assertion,
and keeps the record in sync so permanent-denial enforcement continues. Folding the claim into
`POST /api/approvals` is rejected because the permanent-denial path never calls create, create's
`200` already means "existing pending approval" to deployed gateways, and create would gain a
one-time side effect.

### 4. Revocation is recorded; the reason is not part of synchronization

Atomic migration `048` adds four nullable columns — `denial_reason`, `denial_delivery_deadline`,
`denial_delivered_at`, and `revoked_at` — four structural checks, and a candidate index, as
specified in `data-model.md`. Each fact has one column. `RevokePermanent` records `revoked_at`
and no longer overwrites the `denied_at` of a denial, so a revoked denial keeps its decision time
next to its reason; a revoked grant still records the revocation time in `denied_at`, as before.
A revoked denial is never deliverable. Historical rows keep `NULL` in every new column, so no
historical record is deliverable and no historical revocation is inferred. Claim and revocation
serialize on the row: revocation first suppresses delivery; claim first may already be in flight
and is not recalled. Owner rereads retain the immutable reason in either case.

The sync snapshot adds one field, `denial_deliverable_until`, which the broker derives from the
domain predicate (deadline while unclaimed and unrevoked, absent otherwise). ExtProc only compares
it with the current time and the invoked tool name. The reason travels only in the claim response
to the one ExtProc instance that renders it.

### 5. Agent-facing representation (Denied Outcome)

The agent receives an MCP `CallToolResult` with `isError: true`, a fixed text that contains the
reason, and `_meta["io.agentic-identity-broker/tool-approval"]` with the approval ID, status,
optional persistence, and optional reason. Clients show tool errors to the model, so the agent can
adjust its next step.

### 6. Confidentiality

The reason never enters logs, spans, span events, metrics, diagnostics, or error strings in the
broker, ExtProc, or the SPA. `DenialReason` redacts itself in `slog`, `fmt`, and `%#v` output.
`coding-standards/LOGGING.md` §6.1 lists denial reasons as never-logged data. Audit records state
only whether a reason was present.

## Rationale

- Recording the reason in the denial transition gives indivisibility and immutability without a
  second write.
- The retry is the only channel that every MCP client already uses. A broker-side claim gives one
  authority for "already delivered" across ExtProc instances and restarts, which is required to
  separate the original outcome from later identical invocations (FR-010).
- Keeping the reason out of sync limits the copies of personal text to the database, the owner's
  browser, and the single delivery response.
- A separate stored delivery deadline keeps the review deadline and the delivery window as two
  facts, so a late denial still gets the full window and a review TTL change cannot silently
  change delivery. A fixed constant adds no configuration key.
- An explicit `revoked_at` makes revocation a recorded fact instead of an inference.

## Consequences

### Positive

- Agents learn eligible temporary and permanent denials, with the user's explanation when given;
  unclaimed withdrawn denials are suppressed and valid matching approvals retain precedence.
- Existing clients and reason-free denials keep working; all API changes are additive.
- No new cache, configuration key, or dependency.
- Revocation time and decision time are both recorded and readable by the owner.

### Negative

- A temporary denial now gives the agent one denied outcome on its first eligible retry instead
  of a fresh approval prompt, unless a valid matching approval takes precedence.
- A lost delivery response is not redelivered to the agent.
- A repeated invocation inside the window adds one broker call while an undelivered denial for that
  tool exists.
- Reasons are stored as plaintext, like tool arguments.
- The approval system now has two at-most-once claim mechanisms (`once` consumption and Denial
  Delivery) with different lookups and authentication boundaries.
- A stale ExtProc snapshot can still show a delivery candidate after a concurrent grant; the
  claim then blocks a call that a fresher snapshot would have allowed. This fails closed within
  the existing freshness bound.
- The revoke response and owner reads of a revoked permanent denial now report the original
  `denied_at` instead of the revocation time, which moves to `revoked_at`.
- Delivery after a denial is limited to a fixed 10-minute window that operators cannot tune.

## Alternatives Considered

1. **Deliver on every identical retry until the deadline**: rejected; contradicts FR-010 and turns a
   temporary denial into a timed block.
2. **Agent-facing read by approval ID**: rejected; needs agent changes and a new agent
   authentication boundary.
3. **Server notification (`notifications/elicitation/complete`)**: rejected; ExtProc cannot
   originate notifications on the agent's session.
4. **JSON-RPC error with the reason in `error.data`**: rejected; many clients do not show protocol
   errors to the model.
5. **Reason in the sync snapshot with local delivery**: rejected; broadcasts personal text and
   cannot enforce at-most-once across instances.
6. **Reuse `POST /api/approvals/{id}/consume`**: rejected; consumption removes the record from sync
   and would end permanent-denial enforcement.
7. **Application-layer encryption of the reason**: rejected for this feature; arguments of the same
   sensitivity are plaintext, and the envelope encryption context has no approval subject.
8. **Deliver a denial after permanent revocation**: rejected; the spec excludes withdrawn denials.
   Reusing the delivery timestamp to suppress it falsely records a delivery.
9. **Temporary denials override valid approvals**: rejected; changes accepted authorization
   precedence and adds a broker check to the approved hot path.
10. **Fold the claim into `POST /api/approvals`** (create returns a deliverable denied outcome as
    `200` instead of a pending approval): rejected for the reasons in Decision 3.
11. **Revocation marked only by a cleared delivery-pending flag**: rejected; revocation stays
    implicit, the flag's name suggests a delivery in flight, and owner reads show the reason next
    to the revocation time.
12. **Reuse `expires_at` as the delivery window**: rejected; one column would hold two facts, a
    late denial would get almost no delivery time, and a review TTL change would silently change
    delivery.
13. **Configurable delivery window**: deferred; it adds a configuration key with no known operator
    need, and the per-record deadline allows adding it later without a schema change.
14. **Deliverability in adapter SQL and memory filters only**: rejected; the domain service would
    only delegate, and the rule would live in several adapters.

## Relationships

- **`018-approval-endpoint-auth-boundaries`**: Extended with one gateway endpoint that uses the
  create boundary. No existing boundary changes.
- **`036-extproc-approval-cache-and-gate`**: Extended with a Denial Delivery step and a second
  at-most-once claim next to `once` consumption; freshness, identity, revocation, and `once`
  consumption rules are unchanged.
- **`045-agent-subject-impersonation`**: Decision 6a's rejection of agent-subject tokens on
  subject-token approval endpoints is extended to `POST /api/approvals/denial-deliveries`.
- **`014-long-poll-listen-notify`**: The delivery claim is a row update, so the existing trigger
  publishes it.
- **`035-shared-tool-pattern-matching`**: Unchanged. ExtProc does not compute argument hashes.

## References

- Feature specification: `specs/20261116-181338-approval-denial-reason/spec.md`
- Contracts: `specs/20261116-181338-approval-denial-reason/contracts/`
- MCP specification: tools (`CallToolResult.isError`), URL-mode elicitation, `_meta` key naming

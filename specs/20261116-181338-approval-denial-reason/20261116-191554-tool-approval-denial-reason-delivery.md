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
order, at-most-once `once` consumption), `014-long-poll-listen-notify` (sync), and
`035-shared-tool-pattern-matching` (the only broker-domain import in ExtProc).

## Decision

### 1. The reason belongs to the denial

`ToolApproval` gains an optional immutable `DenialReason` value object, set only in the
pending → denied transition and stored in `tool_approvals.denial_reason`. Empty or whitespace-only
input means no reason, normalized before nonblank length validation (including 501 whitespace
code points within the independent 16-KiB body bound); other input is kept exactly. Store constraints require a denied status and
1–500 characters.

### 2. Delivery happens on the agent's repeat of the original invocation, at most once

The ExtProc approval gate delivers a denied decision when the agent repeats the original
invocation (same verified principal and agent, tool name, and canonical arguments hash) before the
approval's existing `expires_at`, while the denial is unrevoked and no valid matching approval
takes precedence under FR-009. The broker claims the delivery atomically through
`POST /api/approvals/deliveries` and records `denial_delivered_at`. Each denied decision is
delivered at most once. A later invocation, including an identical one after delivery, follows the
existing behavior: a temporary denial lets it create a new approval, and a permanent denial blocks
it with `tool permanently denied` and no reason.

This adds a step to the gate order of `036-extproc-approval-cache-and-gate` without changing any
authorization outcome of that ADR: a fresh permanent denial still blocks, a fresh approval still
proceeds without a delivery call, and creation still follows the authoritative read. The gate calls
the broker for delivery only when the fresh snapshot shows an undelivered, unrevoked denial for
the invoked tool inside its window. A temporary denial never overrides a matching approval;
it remains unclaimed until an eligible repeat or expiry. Concurrent grants or revocations retain
the existing snapshot freshness semantics, without a new globally serialized precedence check.
A failed delivery call denies the invocation and creates no approval. Claim/storage timeouts,
connection failures, or unexpected service failures return `500` in the existing `ApprovalError`
envelope with constant `internal_error` and `failed to deliver approval decision`, never raw
errors or reason text. A committed-but-lost claim is not reset.

### 3. Delivery endpoint authentication

`POST /api/approvals/deliveries` requires the subject token and the client assertion, the same
boundary that `018-approval-endpoint-auth-boundaries` assigns to `POST /api/approvals`. The broker
derives principal and agent from the authenticated context; the request cannot name another pair.
Agent-subject tokens are rejected as on the other subject-token approval endpoints.

### 4. The reason is not part of synchronization

The sync snapshot adds only `expires_at`, `denial_delivered_at`, and `denial_delivery_pending` so the gate can
pre-filter. Atomic migration `048` adds nullable `denial_delivered_at` and `denial_reason`,
`denial_delivery_pending BOOLEAN NOT NULL DEFAULT FALSE`, three checks, and a partial
deliverability index, as specified in `data-model.md`. New pending → denied decisions set the flag
true; claim and permanent revocation clear it atomically, preserving reason and any prior delivery
time. Historical records default false: previous revocations cannot be inferred reliably, and
are never newly delivered as user denials. New claims require `denial_delivery_pending = TRUE`
and `approved_at IS NULL`, distinguishing a revoked permanent denial from a temporary denial
and excluding revoked grants. Claim and revocation serialize under the same row/write lock:
revocation first suppresses delivery; claim first may already be in flight and is not recalled.
Owner rereads retain the immutable reason in either case.
The reason travels only in the delivery claim response to the one ExtProc instance that renders it.

### 5. Agent-facing representation

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
- Reusing `expires_at` as the delivery window adds no retention period or configuration.

## Consequences

### Positive

- Agents learn eligible temporary and permanent denials, with the user's explanation when given;
  unclaimed withdrawn denials are suppressed and valid matching approvals retain precedence.
- Existing clients and reason-free denials keep working; all API changes are additive.
- No new cache, configuration key, or dependency.

### Negative

- A temporary denial now gives the agent one denied outcome on its first eligible retry instead
  of a fresh approval prompt, unless a valid matching approval takes precedence.
- A lost delivery response is not redelivered to the agent.
- A repeated invocation inside the window adds one broker call while an undelivered denial for that
  tool exists.
- Reasons are stored as plaintext, like tool arguments.

## Alternatives Considered

1. **Deliver on every identical retry until `expires_at`**: rejected; contradicts FR-010 and turns a
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
   Reusing the delivery timestamp to suppress it falsely records a delivery, so track pending
   delivery with a separate false-default flag; revocation clears it. DOWN discards all three new fields without changing existing data.
9. **Temporary denials override valid approvals**: rejected; changes accepted authorization
   precedence and adds a broker check to the approved hot path.

## Relationships

- **`018-approval-endpoint-auth-boundaries`**: Extended with one gateway endpoint that uses the
  create boundary. No existing boundary changes.
- **`036-extproc-approval-cache-and-gate`**: Extended with a delivery step; freshness, identity,
  revocation, and `once` consumption rules are unchanged.
- **`014-long-poll-listen-notify`**: The delivery claim is a row update, so the existing trigger
  publishes it.
- **`035-shared-tool-pattern-matching`**: Unchanged. ExtProc does not compute argument hashes.

## References

- Feature specification: `specs/20261116-181338-approval-denial-reason/spec.md`
- Contracts: `specs/20261116-181338-approval-denial-reason/contracts/`
- MCP specification: tools (`CallToolResult.isError`), URL-mode elicitation, `_meta` key naming

# Data Model: Tool approval denial reasons

**Feature**: [spec.md](./spec.md) | **Research**: [research.md](./research.md)

No new entity or entity ID. The feature extends the existing `ToolApproval` aggregate
(`internal/domain/storage/tool_approval.go`) with one value object and one delivery timestamp.

## ToolApproval (aggregate root, extended)

| Field | Type | Rule |
|---|---|---|
| `DenialReason` | `*DenialReason` (`denial_reason TEXT NULL`) | Set only by `Deny`, in the same transition as `Status=denied`. Never set for pending or approved records. Never changed afterwards. |
| `DenialDeliveredAt` | `*time.Time` (`denial_delivered_at TIMESTAMPTZ NULL`) | Set once by `ClaimDenialDelivery` when the requesting agent receives the denied outcome. Only for denied records. |

All existing fields and their meaning are unchanged, including `Persistence`, `ParamsPattern`
clearing on permanent denial, `ExpiresAt`, and `RevokePermanent`.

### Invariants

1. `DenialReason != nil` ⇒ `Status == denied` (FR-005).
2. `DenialReason` is set at most once, by the pending → denied transition; later `Deny` calls fail
   with `ErrApprovalNotPending` and leave it unchanged (FR-006).
3. `DenialDeliveredAt != nil` ⇒ `Status == denied`.
4. A record is **deliverable** at time `now` for invocation `(principal, agent, tool, argsHash)`
   when: `Status == denied`, `ApprovedAt == nil`, `DenialDeliveredAt == nil`,
   `now < ExpiresAt`, and the four values equal the record's `Principal`, `AgentID`, `ToolName`,
   and `ArgumentsHash` (DEC-DELIVERY).
5. Claiming delivery sets `DenialDeliveredAt = now` and is the only mutation after denial besides
   the existing permanent revocation. It never changes `DenialReason`.

### State transitions

```mermaid
stateDiagram-v2
    [*] --> pending: CreatePendingApproval
    pending --> approved: Approve (no reason)
    pending --> denied_undelivered: Deny(persistence?, reason?)
    denied_undelivered --> denied_delivered: ClaimDenialDelivery (original invocation, before expires_at)
    denied_undelivered --> denied_undelivered: later invocation / after expires_at (no change)
    approved --> denied_revoked: RevokePermanent (existing; never deliverable, approved_at set)
    denied_undelivered --> denied_undelivered: RevokePermanent of permanent denial (persistence cleared)
    denied_delivered --> denied_delivered: RevokePermanent of permanent denial (persistence cleared)
```

The reason survives every transition after denial, including revocation, and is returned on
every owner read of that record.

## DenialReason (value object, new)

| Aspect | Rule |
|---|---|
| Constructor | `storage.NewDenialReason(raw string) (*DenialReason, error)` |
| Absence | `""` or only `unicode.IsSpace` runes → `nil, nil` |
| Retention | Otherwise the exact `raw` string, including surrounding and internal whitespace |
| Limit | `utf8.RuneCountInString(raw) <= 500`, else `ErrDenialReasonTooLong` |
| Forbidden | U+0000 → `ErrDenialReasonInvalid` (PostgreSQL `TEXT` cannot store it) |
| Accessor | `Text() string` for the HTTP adapters and storage only |
| Redaction | `LogValue()`, `String()`, `GoString()` return `[REDACTED]` (LOGGING.md §6.4) |
| Authority | None. It never changes matching, enforcement, or policy (SR-004) |

Errors are package sentinels with constant messages; no error wraps the text.

## Domain service changes (`internal/domain/approval`)

| Operation | Change |
|---|---|
| `DenyApproval(ctx, id, principal, DenyRequest{Persistence, Reason})` | Passes the validated reason into `ToolApproval.Deny` and the repository in one write. Ownership, expiry, and pending checks are unchanged and run before the write. |
| `GetApproval`, `ListPendingApprovals`, `ListPermanentApprovals` | `ApprovalDetail` carries `DenialReason` for the owner. |
| `ClaimDenialDelivery(ctx, DeliveryRequest{Principal, AgentID, ToolName, Arguments})` (new) | Computes `storage.ComputeArgumentsHash(Arguments)`, calls the repository claim with `now`, publishes the sync mutation, writes the `approval.tool_approval.decision_delivered` audit record, and returns the claimed approval or `nil` when none is deliverable. |
| `GetSyncState` | Unchanged query; summaries add `ExpiresAt` and `DenialDeliveredAt` in the HTTP adapter. |

## Ports (`internal/ports/storage.go`)

- `ToolApprovalRepository.Deny(ctx, id, persistence, reason *storage.DenialReason, deniedAt)` —
  existing method gains the reason; the conditional update (`status='pending' AND expires_at > NOW()`)
  stays the single atomic write.
- New ISP interface `ToolApprovalDeliveryRepository` with
  `ClaimDenialDelivery(ctx, principal id.Principal, agentID id.AgentID, toolName, argumentsHash string, now time.Time) (*storage.ToolApproval, error)`;
  returns `nil, nil` when no row qualifies. Implemented by the memory and PostgreSQL adapters and
  wired in `internal/app/builder.go`.

## PostgreSQL schema — migration `048_add_tool_approval_denial_reason`

```sql
-- up (one transaction)
ALTER TABLE tool_approvals
    ADD COLUMN denial_reason TEXT NULL,
    ADD COLUMN denial_delivered_at TIMESTAMPTZ NULL;
ALTER TABLE tool_approvals
    ADD CONSTRAINT chk_tool_approvals_denial_reason
        CHECK (denial_reason IS NULL OR (status = 'denied' AND char_length(denial_reason) BETWEEN 1 AND 500)),
    ADD CONSTRAINT chk_tool_approvals_denial_delivered
        CHECK (denial_delivered_at IS NULL OR status = 'denied');
CREATE INDEX idx_tool_approvals_deliverable_denials
    ON tool_approvals (principal, agent_id, tool_name, arguments_hash)
    WHERE status = 'denied' AND denial_delivered_at IS NULL;

-- down (one transaction)
DROP INDEX IF EXISTS idx_tool_approvals_deliverable_denials;
ALTER TABLE tool_approvals
    DROP CONSTRAINT IF EXISTS chk_tool_approvals_denial_delivered,
    DROP CONSTRAINT IF EXISTS chk_tool_approvals_denial_reason,
    DROP COLUMN IF EXISTS denial_delivered_at,
    DROP COLUMN IF EXISTS denial_reason;
```

The claim is one statement, so concurrent claims from several ExtProc instances deliver once:

```sql
UPDATE tool_approvals
SET denial_delivered_at = $6
WHERE id = (
    SELECT id FROM tool_approvals
    WHERE principal = $1 AND agent_id = $2 AND tool_name = $3 AND arguments_hash = $4
      AND status = 'denied' AND approved_at IS NULL AND denial_delivered_at IS NULL
      AND expires_at > $5
    ORDER BY denied_at DESC, id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, principal, agent_id, tool_name, status, persistence, denied_at,
          denial_reason, denial_delivered_at, expires_at;
```

Every `SELECT`/`RETURNING` list that scans a full `ToolApproval` adds `denial_reason` and
`denial_delivered_at` explicitly (no wildcards). The deny update sets
`denial_reason = $4` in the same statement as `status = 'denied'`. The existing
`trigger_tool_approvals_sync` row trigger publishes both updates.

## In-memory adapter

`internal/adapters/storage/memory/tool_approval_repository.go` applies the same conditions under
its existing write lock, selects the newest qualifying record for a claim, and returns copies so
callers cannot mutate stored reasons.

## ExtProc cache record (`internal/extproc/approval`)

`Record` gains `ExpiresAt *time.Time` (`expires_at`) and `DenialDeliveredAt *time.Time`
(`denial_delivered_at`). `Cache` gains `HasDeliverableDenial(identity, tool, now) bool` using
invariant 4 without the hash check. `Broker` gains
`ClaimDelivery(ctx, subjectToken string, request DeliveryRequest) (*DeliveredDecision, error)`;
`DeliveredDecision` holds `ApprovalID`, `Persistence`, and `DenialReason` and is never logged.
`Outcome` gains `Denied *DeliveredDecision` for the server to render the agent response.

## Frontend types (`web/src/types/approval.ts`)

`DenyRequest.denial_reason?: string`, `DenyResponseData.denial_reason?: string`,
`ToolApprovalDetail.denial_reason?: string`.

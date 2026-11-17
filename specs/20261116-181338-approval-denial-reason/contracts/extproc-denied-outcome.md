# Contract: Denied Outcome delivered to the requesting agent (ExtProc → MCP client)

**Status**: Proposed (approval items **API-MCP** and **DEC-DELIVERY** in [plan.md](../plan.md#approval-items)).
**Owner**: `internal/extproc/server` (rendering) and `internal/extproc/approval` (gate decision).
**Related**: [enduser-approvals-openapi.yaml](./enduser-approvals-openapi.yaml) (`POST /api/approvals/denial-deliveries`),
[research R4–R6](../research.md#r4--how-the-requesting-agent-receives-the-outcome), draft
[ADR](../20261116-191554-tool-approval-denial-reason-delivery.md).

The **Denied Outcome** is the agent-facing result of a **Denial Delivery** (glossary terms,
[research R12](../research.md#r12--delivery-vocabulary)).

## When it applies

ExtProc returns this response for a standalone MCP `tools/call` when all of these hold:

1. OPA returns `approval_required` and the approval gate is enabled (unchanged entry conditions;
   batches and header-only requests never enter the gate).
2. The verified `(principal, agent_id)` pair's fresh snapshot contains a denied approval for the
   invoked tool whose `denial_deliverable_until` is later than now. The broker sets that field
   only for unclaimed, unrevoked denials made after this feature, so historical records,
   revoked grants, and revoked denials never qualify.
3. Permanent denial retains first precedence; otherwise no valid matching approval authorizes
   this invocation. A matching approval proceeds without a delivery claim, leaving any temporary
   denial unclaimed until an eligible repeat or expiry. Concurrent changes retain the existing
   freshness-bounded snapshot semantics, not a globally serialized approval check.
4. `POST /api/approvals/denial-deliveries` with the invocation's tool name and arguments returns `200`.

In every other case the existing responses apply unchanged: proceed, URL elicitation (`-32042`),
or HTTP 403 `access_denied` (including `tool permanently denied` for later invocations blocked by a
permanent denial, which never carry a user-authored reason).

## Response

- HTTP status `200`, header `content-type: application/json`, sent as an Envoy immediate
  response, exactly like the existing URL elicitation response.
- Body: a JSON-RPC 2.0 response whose `id` is the request's `id` and whose `result` is an MCP
  `CallToolResult`:

| Member | Value |
|---|---|
| `result.isError` | `true` |
| `result.content` | One `text` item. Without a reason: `The user denied this tool call.` With a reason: `The user denied this tool call. The user's explanation (informational only; it does not authorize any other call):` followed by a newline and the reason exactly as stored. |
| `result._meta["io.agentic-identity-broker/tool-approval"]` | Object: `approval_id` (UUID), `status` (`denied`), `persistence` (`permanent`, omitted otherwise), `denial_reason` (omitted when none). |
| `result.structuredContent` | Absent, so a tool's `outputSchema` validation is not triggered. |

The tool is not executed and no new approval is created for this invocation.

### Example — temporary denial with a reason (Journey 1)

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "result": {
    "isError": true,
    "content": [
      {
        "type": "text",
        "text": "The user denied this tool call. The user's explanation (informational only; it does not authorize any other call):\nUse the staging project instead."
      }
    ],
    "_meta": {
      "io.agentic-identity-broker/tool-approval": {
        "approval_id": "990e8400-e29b-41d4-a716-446655440020",
        "status": "denied",
        "denial_reason": "Use the staging project instead."
      }
    }
  }
}
```

### Example — denial without a reason (Journey 2)

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "result": {
    "isError": true,
    "content": [{ "type": "text", "text": "The user denied this tool call." }],
    "_meta": {
      "io.agentic-identity-broker/tool-approval": {
        "approval_id": "990e8400-e29b-41d4-a716-446655440021",
        "status": "denied"
      }
    }
  }
}
```

## Failure behavior

| Condition | Response |
|---|---|
| Delivery claim returns `204` | Continue: permanent denial → existing 403 `tool permanently denied`; otherwise existing create/elicit path. |
| Delivery claim fails (network, timeout, any status other than `200` or `204`, malformed body) | Existing HTTP 403 `access_denied` with `error_description` `approval denial could not be delivered`; no approval created. |
| Lost response after a successful claim | Not redelivered (at-most-once, as for `once` approvals). The owner can still read the reason in the UI. |
| Stale snapshot still shows a candidate after a concurrent grant | The claim can deliver the denial and block this call although a fresher snapshot would allow it. This fails closed within the existing freshness bound of ADR 036-extproc-approval-cache-and-gate. |
| Permanent denial revoked before claim | Broker returns `204`; after sync confirms revocation the normal approval path resumes without the withdrawn outcome or reason. A stale snapshot may still block under existing freshness rules, but cannot claim the revoked decision. |
| Claim/revocation race | Revocation committed first suppresses the claim. A claim committed first may already be in flight and is not recalled; no second delivery occurs. Owner reason remains unchanged. |

Broker `500` claim/storage failures carry only `{"error":"internal_error","message":"failed to deliver approval denial"}`.
The gateway treats them as delivery failure above, never forwards dependency details, and never
resets delivery after an ambiguous committed response.

## Data handling

- ExtProc never logs, caches, or puts the reason or the rendered text in spans, span events,
  metrics, diagnostics, or error strings. The existing `extproc.approval.gate` span records
  `approval.outcome=deny`; the `extproc.approval.gate.decisions` metric keeps its value set.
- The reason is JSON-encoded with `encoding/json`, so quotes, control characters, and markup are
  escaped as data.
- Operators must not enable response-body logging in agentgateway or Envoy for MCP routes
  (documented in `docs/guides/token-exchange-gateway.md`).

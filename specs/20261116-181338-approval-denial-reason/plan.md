# Implementation Plan: Tool approval denial reasons

**Branch**: `speckit/issue-5` | **Date**: 2026-11-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/20261116-181338-approval-denial-reason/spec.md`

**Status**: Planned. FR-010 was accepted on the clarification comment. This revision applies
the maintainer-directed architecture review findings F1–F7
([Architecture review resolution](#architecture-review-resolution)). Every item in
[Approval Items](#approval-items) needs the maintainer's 👍 on the approval comment before
implementation.

## Summary

Let the denying user attach an optional reason of at most 500 Unicode code points to a tool
approval denial, and deliver it once to the requesting agent with the denied outcome.

- **Domain.** `ToolApproval.Deny` takes an optional immutable `DenialReason` value object and
  records it, with a Denial Delivery deadline 10 minutes after the decision, in the same
  transition as `status=denied`. The value object is the only authority for absence semantics,
  the code-point limit, and redaction in logs. `ToolApproval.IsDenialDeliverable` is the only
  deliverability rule ([data model](./data-model.md), [R1–R2](./research.md#r1--where-the-reason-lives),
  [R13](./research.md#r13--denial-delivery-window)).
- **Browser API.** `POST /api/approvals/{id}/deny` accepts `denial_reason`; deny responses and
  approval reads return it to the owner. The handler bounds the body and checks the JSON type;
  the domain validates the text. Invalid input returns `400 invalid_denial_reason` and leaves the
  approval pending ([R3](./research.md#r3--browser-and-non-browser-input-contract)).
- **Denial Delivery.** When the agent repeats the original `tools/call` within the denial's
  delivery window, while the denial is unrevoked and no matching approval takes precedence under
  FR-009, the ExtProc gate claims it through the new dual-auth gateway endpoint
  `POST /api/approvals/denial-deliveries` and answers with the Denied Outcome, an MCP tool error
  result that contains the reason. Each denial is delivered at most once; subsequent calls keep
  today's behavior (accepted FR-010). The broker enforces at-most-once claiming across instances, unlike the best-effort local `once`
  reservation within one ExtProc instance ([R4–R6](./research.md#r4--how-the-requesting-agent-receives-the-outcome),
  [agent contract](./contracts/extproc-denied-outcome.md)).
- **Sync.** Summaries add one derived field, `denial_deliverable_until`, so the gate calls the
  broker only while an unclaimed, unrevoked denial exists; the reason never enters sync.
- **Persistence.** Atomic migration `048` adds four nullable columns (`denial_reason`,
  `denial_delivery_deadline`, `denial_delivered_at`, `revoked_at`), four check constraints, and a
  candidate index. Revocation records `revoked_at` and keeps a denial's `denied_at`; both adapters
  implement atomic deny, the compare-and-set claim, and revocation
  ([R7](./research.md#r7--persistence-and-explicit-revocation)).
- **Confidentiality.** No log, span, metric, or error carries the reason; LOGGING.md §6.1 gains
  the rule ([R8–R9](./research.md#r8--logging-tracing-and-metrics)).
- **UI.** Both denial surfaces get an accessible optional-reason form and a read-only recorded
  reason ([R10](./research.md#r10--frontend-interaction)).
- **Decision record.** Draft
  [ADR 20261116-191554-tool-approval-denial-reason-delivery](./20261116-191554-tool-approval-denial-reason-delivery.md) (Proposed).

## Technical Context

**Language/Version**: Go 1.27.4 (broker and ExtProc); TypeScript 6 with React 19 and Vite 7 (SPA).

**Primary Dependencies**: chi v5, sqlx/pgx v5 and `lib/pq`, `golang-migrate`, `log/slog`,
OpenTelemetry, `github.com/mark3labs/mcp-go` (existing MCP types in ExtProc), Envoy ExtProc
protos, Ginkgo v2/Gomega, Playwright harness. No new dependency.

**Storage**: PostgreSQL (production) and in-memory (development/test). Migration
`048_add_tool_approval_denial_reason.{up,down}.sql`; no new table.

**Testing**: Go unit tests (value object, aggregate, service, handlers, memory adapter, ExtProc
gate/client/server, log and span safety); Vitest for SPA components; PostgreSQL integration tests
for the adapter and migration; Ginkgo E2E with the production broker and in-process ExtProc
(`tests/e2e/bootstrap/extproc.go`); Playwright frontend E2E with screenshots; one `docker`-labelled
restart journey.

**Target Platform**: Linux containers for the broker and ExtProc; evergreen browsers for the SPA.

**Project Type**: Web application in the Go monorepo: broker (`internal/`), standalone ExtProc
(`internal/extproc/`), SPA (`web/`).

**Performance Goals**: No added broker call on the gate's hot path except while an undelivered
denial for the invoked tool exists inside its window (at most one claim per such invocation). The
claim uses a partial index.

**Constraints**: Fail closed on any delivery failure; the reason never leaves the database, the
owner's browser, and the single claim response; ExtProc imports nothing new from the broker
domain (`035-shared-tool-pattern-matching`); no configuration change.

**Scale/Scope**: Approval rows per user–agent pair are bounded by the existing rate limits. Two
browser surfaces, five handlers, one new handler, one migration.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Before proceeding, verify compliance with [.specify/memory/constitution.md](../../.specify/memory/constitution.md):

**Design Preconditions (BLOCKING)**:

- [x] **Domain Model**: [data-model.md](./data-model.md) documents the extended `ToolApproval`
  aggregate, the `DenialReason` value object, invariants, and state transitions.
- [x] **Domain Concepts**: Implementation adds **Denial Reason** (user-authored text with no
  authority, unlike OPA `reasons`), **Denial Delivery**, **Denial Delivery Deadline**, and
  **Denied Outcome** to the `ARCHITECTURE.md` glossary and the root `AGENTS.md` glossary, defines
  `revoked_at` in the ToolApproval entry, and extends the `ApprovalDenied` event text
  ([R12](./research.md#r12--delivery-vocabulary)).
- [x] **Entity IDs**: No new entity; `id.ApprovalID` exists (ADR 013-strongly-typed-entity-ids).
- [x] **Configuration Design**: No configuration key changes. The delivery window is a fixed
  domain constant stored per denial (DEC-WINDOW); `approvals.pending_ttl` keeps bounding only the
  review.
- [x] **Config Examples**: Not applicable.
- [x] **Helm Deployment Contract**: Not applicable; the migration Job runs `048` unchanged.
- [x] **API Design First**: [contracts/enduser-approvals-openapi.yaml](./contracts/enduser-approvals-openapi.yaml)
  and [contracts/extproc-denied-outcome.md](./contracts/extproc-denied-outcome.md) define all
  changes before implementation.
- [x] **API Documentation**: `api/enduser/openapi.yaml`, `docs/api/approval-endpoints.md`,
  `docs/api/enduser-api.md`, and `docs/guides/token-exchange-gateway.md`.
- [ ] **API Changes**: Pending maintainer sign-off on API-DENY, API-READ, API-SYNC, API-DELIVER,
  API-MCP, API-VERSION, and the revoke-response change in DEC-REVOKE. This gate stays open until the approval comment receives a 👍.
- [x] **Database Design**: `048_add_tool_approval_denial_reason.{up,down}.sql`
  ([data-model.md](./data-model.md#postgresql-schema--migration-048_add_tool_approval_denial_reason)).
- [x] **E2E Acceptance Tests**: Planned for all 13 scenarios before implementation
  ([Testing Strategy](#testing-strategy)).
- [x] **E2E Test Mapping**: One `It()` per scenario.
- [x] **E2E Red Phase**: Tests assert status codes, JSON bodies, MCP results, stored state,
  captured log/span buffers, and rendered elements.
- [x] **Frontend Playwright E2E**: `tests/e2e/frontend/approval_denial_reason_test.go`.
- [x] **Frontend Screenshots**: Pending review, reason entry, validation error, and recorded
  decision on both surfaces.

**Implementation Considerations**:

- [x] **Security-First**: Existing ownership and expiry checks gate the reason; the delivery
  endpoint takes the create endpoint's dual authentication; delivery failures fail closed; the
  reason is untrusted text everywhere (SR-001–SR-004).
- [x] **Architecture Docs**: `ARCHITECTURE.md` approval sections (endpoint list, gate order, sync
  fields, glossary) and the ADR index in the root `AGENTS.md`.
- [x] **ADRs**: Draft [ADR 20261116-191554-tool-approval-denial-reason-delivery](./20261116-191554-tool-approval-denial-reason-delivery.md)
  (Proposed) extends `018-approval-endpoint-auth-boundaries`,
  `036-extproc-approval-cache-and-gate`, and `045-agent-subject-impersonation` decision 6a
  without deviating from them. Implementation moves it to
  `adrs/` after approval.
- [x] **Library-First Security**: No cryptography.
- [x] **Zalando Guidelines**: snake_case fields and `data` envelopes. Additive fields plus the
  changed `denied_at` value on revoked denials require End-User API `2.0.0` on existing routes
  (API-VERSION); consumer guidance moves revocation-time reads to `revoked_at`. The action-style
  paths follow the existing approval API conventions.
- [x] **End-User Docs**: `docs/api/approval-endpoints.md` gains deny-with-reason and delivery examples.
- [x] **Migration Testing**: Up/down/reapply against PostgreSQL in `tests/integration/migrations/`.
- [x] **Hexagonal Architecture**: Reason validation lives only in `DenialReason`; deliverability
  lives only in `ToolApproval.IsDenialDeliverable`, which `approval.Service.ClaimDenialDelivery`
  applies before a compare-and-set write. Handlers parse and map errors; adapters narrow
  candidates and guard concurrency; ExtProc reads a broker-derived sync field and talks to the
  broker only over HTTP.
- [x] **Persistence Patterns**: ISP port `ToolApprovalDenialDeliveryRepository`; memory and
  PostgreSQL adapters; `StorageError` wrapping (ADR 004-storage-layer-architecture).
- [x] **Coding standards**: LOGGING.md and OPENTELEMETRY.md review checklists apply to the new
  audit record and span; safety tests assert the reason is absent from logs and spans.
  [contracts/observability.md](./contracts/observability.md) specifies the event, span families,
  existing gate metric, and safety assertions; it adds no metric or attribute namespace.

**Post-design re-check (after Phase 1)**: All blocking design preconditions pass except the API
confirmation gate, which is the expected sign-off. No violation needs Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/20261116-181338-approval-denial-reason/
├── spec.md
├── plan.md                    # This file
├── research.md                # Phase 0
├── data-model.md              # Phase 1
├── quickstart.md              # Phase 1
├── contracts/
│   ├── enduser-approvals-openapi.yaml   # End-User API 2.0.0 delta
│   ├── extproc-denied-outcome.md        # Agent-facing MCP result
│   └── observability.md                 # Content-free audit, spans, and safety tests
├── 20261116-191554-tool-approval-denial-reason-delivery.md   # ADR draft (Proposed)
├── checklists/requirements.md
├── wireframes/
└── tasks.md                   # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/domain/storage/tool_approval.go          # Deny(reason), DenialDeliveryWindow, IsDenialDeliverable, ClaimDenialDelivery
internal/domain/storage/denial_reason.go          # DenialReason value object
internal/domain/approval/service.go               # DenyApproval(raw reason), ClaimDenialDelivery, ApprovalDetail
internal/ports/storage.go                         # Deny signature, ToolApprovalDenialDeliveryRepository
internal/adapters/storage/memory/tool_approval_repository.go
internal/adapters/storage/postgres/tool_approval_repository.go
internal/adapters/http/handlers/approval/         # deny, revoke, get/pending/permanent, sync, new denial_delivery_handler.go
internal/adapters/http/routing/enduser.go         # POST /api/approvals/denial-deliveries (dual auth)
internal/app/builder.go                           # wire delivery repository and handler
internal/extproc/approval/{cache,client,gate}.go  # Record.DenialDeliverableUntil, ClaimDenialDelivery, gate step
internal/extproc/server/server.go                 # render denied CallToolResult
migrations/048_add_tool_approval_denial_reason.{up,down}.sql
api/enduser/openapi.yaml
web/src/types/approval.ts, web/src/services/api/approvals.ts, web/src/hooks/useApproval.ts
web/src/components/approvals/                     # DenialReasonForm, ApprovalReviewPage, confirmation
web/src/pages/ToolAuthorizationsPage.tsx
tests/e2e/approval_denial_reason_test.go
tests/e2e/frontend/approval_denial_reason_test.go
tests/e2e/pages/{approval_page,tool_authorizations_page}.go
tests/integration/migrations/                     # 048 up/down
docs/api/approval-endpoints.md, docs/api/enduser-api.md, docs/guides/token-exchange-gateway.md
coding-standards/LOGGING.md                       # §6.1 row
ARCHITECTURE.md, AGENTS.md, internal/adapters/AGENTS.md, internal/extproc/AGENTS.md
```

**Structure Decision**: Extend the existing approval bounded context (`internal/domain/approval`,
`internal/domain/storage`), its adapters, the ExtProc approval package, and the existing SPA
approval components. No new package or bounded context.

## Implementation Phase Overview

| Phase | Purpose | Required? |
|-------|---------|-----------|
| **Phase 0** | Pre-implementation refactoring | Skipped |
| **Phase 1** | Setup | Minimal: record approval, move ADR |
| **Phase 2** | Design preconditions: OpenAPI, ADR, migration, red E2E | **MANDATORY** |
| **Phase 2.7** | Entity boilerplate | Skipped |
| **Phase 3+** | User Stories (P1: US1, US3; P2: US2) | Yes |
| **Phase N** | Constitution compliance | **MANDATORY** |

- [x] Phase 0 (refactoring): skip — changes extend existing types and signatures in place; no
  structural move is needed first.
- [x] Phase 2.7 (entity boilerplate): skip — no new entity.

Suggested order inside Phase 3+: domain value object and aggregate → ports and both adapters with
migration → deny and revoke handlers and reads → denial-delivery service, handler, route → ExtProc
client, cache, gate, renderer → SPA → docs and glossary.

## Testing Strategy

### End-to-End (E2E) Acceptance Tests

**Test Locations**: Ten backend scenarios in `tests/e2e/approval_denial_reason_test.go` (broker +
in-process ExtProc through `bootstrap.ExtProcEnvironment`, as in
`tests/e2e/extproc_approval_journey_test.go`); three browser scenarios in
`tests/e2e/frontend/approval_denial_reason_test.go` using the existing frontend lifecycle.

**Framework**: Ginkgo/Gomega, `Describe("Tool approval denial reasons")` → `Context` per user story → one `It` per scenario.

**Scenario Mapping**:

| Spec Scenario | E2E Test Location | Test Description |
|---------------|-------------------|------------------|
| User Story 1, Scenario 1 | `tests/e2e/approval_denial_reason_test.go` | `It("delivers the denial reason with the denied outcome on the agent's repeat and does not execute the tool")` |
| User Story 1, Scenario 2 | `tests/e2e/frontend/approval_denial_reason_test.go` | `It("offers an optional reason with limit and sharing notice on both denial surfaces and cancels without change")` |
| User Story 1, Scenario 3 | `tests/e2e/approval_denial_reason_test.go` | `It("accepts 500 valid code points intact end to end, rejects 501 or U+0000 with 400 while pending, and accepts the corrected resubmission")` |
| User Story 2, Scenario 1 | `tests/e2e/approval_denial_reason_test.go` | `It("delivers a reason-free denied outcome for a bodiless deny")` |
| User Story 2, Scenario 2 | `tests/e2e/approval_denial_reason_test.go` | `It("treats a whitespace-only reason as no reason")` |
| User Story 2, Scenario 3 | `tests/e2e/approval_denial_reason_test.go` | `It("keeps approval unchanged and attaches no reason")` |
| User Story 3, Scenario 1 | `tests/e2e/approval_denial_reason_test.go` | `It("rejects reason authorship and reads by unauthenticated callers, other users, and the agent")` |
| User Story 3, Scenario 2 | `tests/e2e/approval_denial_reason_test.go` | `It("keeps the first decision and reason on any later deny attempt")` |
| User Story 3, Scenario 3 | `tests/e2e/approval_denial_reason_test.go` | `It("records no reason for expired or approved approvals and keeps only the winner's reason under competing decisions")` |
| User Story 3, Scenario 4 | `tests/e2e/approval_denial_reason_test.go` | `It("returns the same decision and reason after a PostgreSQL broker restart and delivers it once", Label("docker"))` |
| User Story 3, Scenario 5 | `tests/e2e/frontend/approval_denial_reason_test.go` | `It("keeps the reason out of browser, broker, and ExtProc logs, spans, and metrics in successful and rejected journeys")` |
| User Story 3, Scenario 6 | `tests/e2e/frontend/approval_denial_reason_test.go` | `It("stores markup-like reasons as data and renders them literally")` |
| User Story 3, Scenario 7 | `tests/e2e/approval_denial_reason_test.go` | `It("blocks later permanent-denial calls without a reason and suppresses unclaimed delivery on revocation while retaining the owner's reason")` |

*Line numbers are added when the tests are written (Phase 2).*

**Red Phase Requirements**: Tests compile and fail on realistic assertions (status codes, MCP
`result.isError`, `_meta` reason, stored fields, absence of text in captured buffers). No
`XIt`/`PIt`/`Skip`, no red-phase comments.

**Test Data Strategy**: Existing agent and principal fixtures in `tests/e2e/fixtures/`; an OPA
policy returning `approval_required` for `deploy_release` as in the existing approval journey; a
distinctive synthetic reason constant per test for leak searches.

**Bootstrap Strategy**: Production broker via `bootstrap.TestServer` with in-memory storage, and
PostgreSQL through `bootstrap.StorageFactory` for the `docker` restart journey; ExtProc via
`bootstrap.ExtProcEnvironment` with injected JSON log buffers and in-memory span recorders.

**Helper Utilities**: Extend `tests/e2e/extproc/helpers` with a parser for the MCP denied result;
reuse `tests/e2e/helpers/approval_types.go` with `DenialReason` fields.

### Frontend Playwright E2E Tests

**Test Location**: `tests/e2e/frontend/approval_denial_reason_test.go`, using page objects
`tests/e2e/pages/approval_page.go` and `tests/e2e/pages/tool_authorizations_page.go`.

**UI Scenario Mapping**:

| UI Scenario | Playwright Test | Screenshot Filename |
|-------------|-----------------|---------------------|
| Pending review (approval page) | Scenario 1.2 | `tests/e2e/screenshots/approval_denial_pending_review.png` |
| Reason entry with count and notice | Scenario 1.2 | `tests/e2e/screenshots/approval_denial_enter_reason.png` |
| Over-limit validation, still pending | Scenario 1.2 | `tests/e2e/screenshots/approval_denial_reason_validation_error.png` |
| Reason entry from the pending list | Scenario 1.2 | `tests/e2e/screenshots/approval_denial_pending_list_reason.png` |
| Recorded denial with reason | Scenario 1.2 | `tests/e2e/screenshots/approval_denial_recorded_reason.png` |
| Literal markup display | Scenario 3.6 | `tests/e2e/screenshots/approval_denial_literal_markup.png` |

Scenario 3.5 is the single full browser/broker/ExtProc privacy journey. It uses the frontend
suite's existing Playwright lifecycle, production broker, and test-only in-process ExtProc
bootstrap with captured broker/gateway logs, spans, and metrics; no browser lifecycle is added
to the backend suite. Capture successful and rejected submissions and delivery failures, then
assert no reason content in any component's operational signals and permitted non-content audit
metadata. Scenarios 1.2 and 3.6 also retain console-safety assertions, without duplicating 3.5.
Each frontend test fails on any browser console message containing the submitted reason.
Capture with `E2E_CAPTURE_SCREENSHOTS=true GINKGO_FRONTEND_PROCS=1 just test-e2e-frontend`;
the harness writes `tests/e2e/frontend/coverage/screenshots/`, and the existing screenshot
workflow publishes captures to the paths above. Include each meaningful state on both denial
surfaces, using surface-specific filenames when a state occurs on both pages.

### Unit & Integration Tests

- **Domain**: `DenialReason` absence before length validation (including 501 whitespace code
  points), exact retention, nonblank 500/501 code points (`é`, emoji), U+0000,
  redaction through `slog`, `%v`, `%#v`; `ToolApproval.Deny` invariants and deadline;
  the deliverability conformance table ([data model](./data-model.md#deliverability-conformance-table))
  against `IsDenialDeliverable`, `DenialDeliverableUntil`, and `ClaimDenialDelivery`; service deny
  (domain validation before ownership) and claim paths, including candidate ordering, lost
  compare-and-set, ownership, expiry, competing decisions, and audit record content.
- **Handlers**: deny body parsing (body bound, JSON type, mapping of domain sentinels without
  echo, no handler-side trimming or counting), revoke response (`denied_at` kept, `revoked_at`),
  read models, sync `denial_deliverable_until` per conformance row, denial-delivery handler status
  mapping including constant `500 internal_error` for claim/storage timeout, connection, and
  unexpected service failures without content echo.
- **Adapters**: memory and PostgreSQL deny with reason and deadline, candidate listing, the
  conformance table through the service, compare-and-set under concurrency, revocation recording
  `revoked_at` and keeping a denial's `denied_at`, column lists; deny-permanently → revoke → repeat
  within the window separately from revoked grants, with claim/revoke lock-order races.
- **Migration**: `048` up, down, reapply; every check constraint; historical rows with all new
  columns `NULL`.
- **ExtProc**: cache candidate detection from `denial_deliverable_until` per conformance row, client claim (200/204/errors, no logging of body), gate
  order (permanent with/without candidate, approved precedence over temporary denial,
  grant/revocation overlap races under existing freshness semantics, read-then-claim, fail closed),
  server rendering of the MCP result, and a safety test asserting the reason is absent from logs,
  spans, and diagnostics.
- **SPA**: `DenialReasonForm` accessibility (label, `aria-describedby` count and error, keyboard),
  count by code points, 501 whitespace code points normalize to absence before nonblank length
  validation on both pages, cancel discards, error retains text; both pages send `denial_reason`;
  confirmation renders literally and omits an empty heading.

**Test Coverage Goals**: All acceptance scenarios (13) in E2E; all new domain branches in unit
tests; both adapters for every new repository method.

## Approval Items

| ID | Type | Decision | Details |
|---|---|---|---|
| API-DENY | API | `POST /api/approvals/{id}/deny` accepts optional `denial_reason` (whitespace-only normalizes to absence before limiting nonblank text to 500 code points, including 501 whitespace code points; U+0000, non-string/non-null, or body >16 KiB → `400 invalid_denial_reason` without echo; the domain validates, the schema pattern is advisory) and `DenyResponse` returns it | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:105-233 |
| API-READ | API | `ToolApprovalDetail` (get, pending, permanent) adds owner-visible `denial_reason` and `revoked_at`, each present only when recorded; the revoke response (`DenyResponse`) adds `denial_reason` and `revoked_at` | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:158-247 |
| API-SYNC | API | `ToolApprovalSummary` adds one nullable field, `denial_deliverable_until` (the Denial Delivery deadline while unclaimed and unrevoked, absent otherwise); the reason is never synchronized | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:248-260 |
| API-DELIVER | API | New gateway endpoint `POST /api/approvals/denial-deliveries` (`claimDenialDelivery`) with subject token + client assertion: claims the newest deliverable denial for the caller's pair, tool, and arguments hash; `200` with denial, reason, and `denial_delivered_at`, `204` when none, constant `500 internal_error`/`failed to deliver approval denial` for claim/storage failures; committed-but-lost responses stay claimed | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:12-104 |
| API-MCP | API | Agent receives the Denied Outcome as HTTP 200 MCP `CallToolResult` with `isError: true`, fixed text containing the reason, and `_meta["io.agentic-identity-broker/tool-approval"]`; delivery failures return 403 `approval denial could not be delivered` | specs/20261116-181338-approval-denial-reason/contracts/extproc-denied-outcome.md:9-106 |
| API-VERSION | API | Proposed: End-User API `info.version` `1.2.0` → `2.0.0` on existing routes, with consumer guidance using `revoked_at` for revocation time; the changed `denied_at` value on revoked denials requires a major version | specs/20261116-181338-approval-denial-reason/research.md:400-415 |
| ADR-DELIVERY | ADR | Draft ADR 20261116-191554-tool-approval-denial-reason-delivery: denial reason on `ToolApproval`, at-most-once Denial Delivery on the agent's repeat of the original invocation via a dual-auth claim beside `once` consumption, stored delivery deadline, explicit revocation, reason excluded from sync and telemetry | specs/20261116-181338-approval-denial-reason/20261116-191554-tool-approval-denial-reason-delivery.md:1-224 |
| DEC-DELIVERY | Decision | Deliver each new denied outcome to the agent at most once (historical records excluded), on its first eligible repeat of the original invocation within the denial delivery window; revocation suppresses unclaimed delivery, while claims committed first may be in flight; valid matching approvals retain precedence over temporary denials under existing freshness semantics, and a stale snapshot fails closed; a lost response is not redelivered | specs/20261116-181338-approval-denial-reason/research.md:121-185 |
| DEC-TEMP | Decision | Behavior change: after a temporary denial, the agent's first eligible repeat of the original call receives the Denied Outcome instead of a new approval prompt, unless a valid matching approval authorizes it | specs/20261116-181338-approval-denial-reason/spec.md:38-40 |
| DEC-CLAIM | Decision | Keep Denial Delivery as a separate additive endpoint with broker-enforced at-most-once claiming across instances, beside the unchanged best-effort local `once` reservation within one instance in ADR 036-extproc-approval-cache-and-gate, with the domain predicate applied before a compare-and-set write. Alternative: fold the claim into `POST /api/approvals` (return a deliverable denied outcome as `200` instead of a pending approval) | specs/20261116-181338-approval-denial-reason/research.md:186-256 |
| DEC-WINDOW | Decision | Bound Denial Delivery by a stored `denial_delivery_deadline` = `denied_at` + a fixed 10-minute domain constant, independent of the review deadline `expires_at`; no configuration key. Alternatives: keep reusing `expires_at`; add a configuration key | specs/20261116-181338-approval-denial-reason/research.md:444-466 |
| DEC-REVOKE | Decision | Record permanent revocation in a new `revoked_at`; revocation of a denial keeps its original `denied_at` (backward-incompatible value change in the revoke response and owner reads of revoked denials), revoked grants keep today's `denied_at`; the delivery-pending flag is dropped. Alternative: keep a delivery-pending flag as the only revocation marker and document it | specs/20261116-181338-approval-denial-reason/research.md:291-341 |
| DB-048 | Decision | Migration `048`: nullable `denial_reason TEXT`, `denial_delivery_deadline`, `denial_delivered_at`, and `revoked_at TIMESTAMPTZ`, four check constraints, candidate index; no backfill (historical rows never deliverable, no inferred revocation); DOWN discards all four new fields; reasons stored as plaintext like tool arguments | specs/20261116-181338-approval-denial-reason/data-model.md:133-200 |

Implementation must not start any item above until the maintainer's 👍 on the approval comment
covers it. The implementation phase then records that approval in the ADR's Status line and moves
the draft to `adrs/20261116-191554-tool-approval-denial-reason-delivery.md`.

## Task integration details

- Scenario 3.5's canonical `It()` lives in `tests/e2e/frontend/approval_denial_reason_test.go`
  and includes real browser submission before broker and ExtProc signal assertions. Reuse the
  existing frontend browser/context cleanup, never a backend browser wrapper. Test-only shared
  bootstrap may expose log/span/metric capture and ExtProc calls, but owns no browser lifecycle.
- Scenario 1.1 includes valid-approval/temporary-denial overlap and synchronized grant/revocation
  race variants; Scenario 3.7 includes revoke-before-delivery and claim/revoke lock-order variants,
  separate from revoking a previous grant. Keep one canonical `It()` per existing scenario.
- When implementation moves the approved ADR into `adrs/`, leave a link-only redirect at the
  feature-local draft path so links from the frozen specification and plan still resolve.
- `tasks.md` orders the two P1 stories (US1, then US3) before the P2 compatibility story (US2).
  US1 is a candidate MVP slice, not a release that can omit privacy or compatibility gates.

## Architecture review resolution

The maintainer directed this planning run to address findings F1–F7 of the plan's architecture
review. Each finding's recommended change was applied; none conflicts with an accepted ADR or the
accepted FR-010 answer. F1, F3, and F4 have real alternatives, so each is registered below as a
Proposed decision with its alternative.

| Finding | Resolution | Where |
|---|---|---|
| F1 Separate claim beside `once` consumption | Keep a separate additive endpoint; justify it against create and consume and distinguish broker-enforced cross-instance Denial Delivery from unchanged best-effort local `once` reservation (**DEC-CLAIM**; alternative: fold the claim into `POST /api/approvals`) | [R5](./research.md#r5--denial-delivery-claim-endpoint-authentication-and-relation-to-once-consumption), ADR draft Decision 3 and Consequences |
| F2 Deny handler duplicates `DenialReason` rules | Handler bounds the body, checks the JSON type, passes raw text, and maps domain sentinels; the domain constructor is the only authority; contract pattern and browser check are advisory | [R2–R3](./research.md#r3--browser-and-non-browser-input-contract), ADR draft Decision 1 |
| F3 Revocation recorded only by an implicit flag | Explicit `revoked_at`; revocation keeps a denial's `denied_at`; the delivery-pending flag is removed (**DEC-REVOKE**; alternative: keep and document the flag) | [R7](./research.md#r7--persistence-and-explicit-revocation), [data model](./data-model.md) |
| F4 `expires_at` holds two facts | Stored `denial_delivery_deadline` = `denied_at` + fixed 10 minutes (**DEC-WINDOW**; alternatives: keep `expires_at`, or a configuration key) | [R13](./research.md#r13--denial-delivery-window) |
| F5 Inconsistent delivery vocabulary | One term, **Denial Delivery**; `/api/approvals/denial-deliveries`; `denial_delivered_at` in every schema; sync field `denial_deliverable_until` | [R12](./research.md#r12--delivery-vocabulary) |
| F6 Anemic claim service, scattered predicate | `ToolApproval.IsDenialDeliverable` is the only rule; the service applies it before a compare-and-set; one conformance table pins both adapters, sync, and ExtProc | [R5](./research.md#r5--denial-delivery-claim-endpoint-authentication-and-relation-to-once-consumption), [data model](./data-model.md#deliverability-conformance-table) |
| F7 ADR draft omits `045-agent-subject-impersonation` | Added to Context, Decision 3, and Relationships | ADR draft |

The review's other notes are also applied: the ADR's Negative consequences list the fail-closed
stale-snapshot case; the glossary entries contrast Denial Reason with OPA `reasons` and name the
Denied Outcome payload; the sync snapshot carries one non-overlapping field.

`tasks.md` uses the final proposed endpoint, schema, port, and sync names and the four-column
migration design. API-VERSION proposes `2.0.0`, rather than the earlier `1.3.0`, because
DEC-REVOKE changes the timestamp value returned on existing routes; this follows Constitution
Principle IV and the repository's API-versioning rule, not an exception to either.

## Follow-up topics

After maintainer authorization, implementation opens or reuses one follow-up issue per topic
under the repository's follow-up rules; deferred remediation stays outside the feature tasks:

- **Shared approval request body limit** — apply one request-body bound to every approval route
  (create, approve, scope-preview, denial-delivery); this feature bounds only the deny endpoint
  ([R3](./research.md#r3--browser-and-non-browser-input-contract)).

## Complexity Tracking

No constitution violation needs justification.

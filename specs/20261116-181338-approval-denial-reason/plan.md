# Implementation Plan: Tool approval denial reasons

**Branch**: `speckit/issue-5` | **Date**: 2026-11-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/20261116-181338-approval-denial-reason/spec.md`

**Status**: Planned. FR-010 was accepted on the clarification comment. Every item in
[Approval Items](#approval-items) needs the maintainer's 👍 on the approval comment before
implementation.

## Summary

Let the denying user attach an optional reason of at most 500 Unicode code points to a tool
approval denial, and deliver it once to the requesting agent with the denied outcome.

- **Domain.** `ToolApproval.Deny` takes an optional immutable `DenialReason` value object and
  records it in the same transition as `status=denied`. The value object enforces absence
  semantics, the code-point limit, and redaction in logs
  ([data model](./data-model.md), [R1–R2](./research.md#r1--where-the-reason-lives)).
- **Browser API.** `POST /api/approvals/{id}/deny` accepts `denial_reason`; deny responses and
  approval reads return it to the owner. Invalid input returns `400 invalid_denial_reason` and
  leaves the approval pending ([R3](./research.md#r3--browser-and-non-browser-input-contract)).
- **Agent delivery.** When the agent repeats the original `tools/call` before the approval's
  `expires_at`, the ExtProc gate claims the denied decision through the new dual-auth gateway
  endpoint `POST /api/approvals/deliveries` and answers with an MCP tool error result that
  contains the reason. Each decision is delivered at most once; later calls keep today's behavior
  (accepted FR-010) ([R4–R6](./research.md#r4--how-the-requesting-agent-receives-the-outcome),
  [agent contract](./contracts/extproc-denied-outcome.md)).
- **Sync.** Summaries add `expires_at` and `denial_delivered_at` so the gate calls the broker only
  while an undelivered denial exists; the reason never enters the sync snapshot.
- **Persistence.** Atomic migration `048` adds two nullable columns, two check constraints, and a
  partial index; both adapters implement the atomic deny and claim ([R7](./research.md#r7--persistence)).
- **Confidentiality.** No log, span, metric, or error carries the reason; LOGGING.md §6.1 gains
  the rule ([R8–R9](./research.md#r8--logging-tracing-and-metrics)).
- **UI.** Both denial surfaces get an accessible optional-reason form and a read-only recorded
  reason ([R10](./research.md#r10--frontend-interaction)).
- **Decision record.** Draft
  [ADR 20261116-191554](./20261116-191554-tool-approval-denial-reason-delivery.md) (Proposed).

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
- [x] **Domain Concepts**: Implementation adds **Denial Reason** and **Decision Delivery** to the
  `ARCHITECTURE.md` glossary and the root `AGENTS.md` glossary, and extends the
  `ApprovalDenied` event text.
- [x] **Entity IDs**: No new entity; `id.ApprovalID` exists (ADR 013).
- [x] **Configuration Design**: No configuration key changes. The delivery window reuses each
  approval's existing `expires_at`.
- [x] **Config Examples**: Not applicable.
- [x] **Helm Deployment Contract**: Not applicable; the migration Job runs `048` unchanged.
- [x] **API Design First**: [contracts/enduser-approvals-openapi.yaml](./contracts/enduser-approvals-openapi.yaml)
  and [contracts/extproc-denied-outcome.md](./contracts/extproc-denied-outcome.md) define all
  changes before implementation.
- [x] **API Documentation**: `api/enduser/openapi.yaml`, `docs/api/approval-endpoints.md`,
  `docs/api/enduser-api.md`, and `docs/guides/token-exchange-gateway.md`.
- [ ] **API Changes**: Pending maintainer sign-off on API-DENY, API-READ, API-SYNC, API-DELIVER,
  API-MCP, and API-VERSION. This gate stays open until the approval comment receives a 👍.
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
- [x] **ADRs**: Draft [ADR 20261116-191554](./20261116-191554-tool-approval-denial-reason-delivery.md)
  (Proposed) extends `018-approval-endpoint-auth-boundaries` and
  `036-extproc-approval-cache-and-gate` without deviating from them. Implementation moves it to
  `adrs/` after approval.
- [x] **Library-First Security**: No cryptography.
- [x] **Zalando Guidelines**: snake_case fields, `data` envelopes, additive changes, minor version
  bump. The action-style paths follow the existing approval API conventions.
- [x] **End-User Docs**: `docs/api/approval-endpoints.md` gains deny-with-reason and delivery examples.
- [x] **Migration Testing**: Up/down/reapply against PostgreSQL in `tests/integration/migrations/`.
- [x] **Hexagonal Architecture**: Validation and deliverability live in the domain
  (`DenialReason`, `ToolApproval`, `approval.Service`); handlers parse and map errors; adapters
  implement ports; ExtProc talks to the broker only over HTTP.
- [x] **Persistence Patterns**: ISP port `ToolApprovalDeliveryRepository`; memory and PostgreSQL
  adapters; `StorageError` wrapping (ADR 004).
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
│   ├── enduser-approvals-openapi.yaml   # End-User API 1.3.0 delta
│   ├── extproc-denied-outcome.md        # Agent-facing MCP result
│   └── observability.md                 # Content-free audit, spans, and safety tests
├── 20261116-191554-tool-approval-denial-reason-delivery.md   # ADR draft (Proposed)
├── checklists/requirements.md
├── wireframes/
└── tasks.md                   # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/domain/storage/tool_approval.go          # DenialReason, Deny(reason), deliverability
internal/domain/approval/service.go               # DenyApproval(reason), ClaimDenialDelivery, ApprovalDetail
internal/ports/storage.go                         # Deny signature, ToolApprovalDeliveryRepository
internal/adapters/storage/memory/tool_approval_repository.go
internal/adapters/storage/postgres/tool_approval_repository.go
internal/adapters/http/handlers/approval/         # deny, get/pending/permanent, sync, new delivery_handler.go
internal/adapters/http/routing/enduser.go         # POST /api/approvals/deliveries (dual auth)
internal/app/builder.go                           # wire delivery repository and handler
internal/extproc/approval/{cache,client,gate}.go  # Record fields, ClaimDelivery, gate step
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
migration → deny handler and reads → delivery service, handler, route → ExtProc client, cache, gate,
renderer → SPA → docs and glossary.

## Testing Strategy

### End-to-End (E2E) Acceptance Tests

**Test Location**: `tests/e2e/approval_denial_reason_test.go` (broker + in-process ExtProc through
`bootstrap.ExtProcEnvironment`, as in `tests/e2e/extproc_approval_journey_test.go`).

**Framework**: Ginkgo/Gomega, `Describe("Tool approval denial reasons")` → `Context` per user story → one `It` per scenario.

**Scenario Mapping**:

| Spec Scenario | E2E Test Location | Test Description |
|---------------|-------------------|------------------|
| User Story 1, Scenario 1 | `tests/e2e/approval_denial_reason_test.go` | `It("delivers the denial reason with the denied outcome on the agent's repeat and does not execute the tool")` |
| User Story 1, Scenario 2 | `tests/e2e/frontend/approval_denial_reason_test.go` | `It("offers an optional reason with limit and sharing notice on both denial surfaces and cancels without change")` |
| User Story 1, Scenario 3 | `tests/e2e/approval_denial_reason_test.go` | `It("accepts 500 code points intact end to end, rejects 501 with 400 while pending, and accepts the corrected resubmission")` |
| User Story 2, Scenario 1 | `tests/e2e/approval_denial_reason_test.go` | `It("delivers a reason-free denied outcome for a bodiless deny")` |
| User Story 2, Scenario 2 | `tests/e2e/approval_denial_reason_test.go` | `It("treats a whitespace-only reason as no reason")` |
| User Story 2, Scenario 3 | `tests/e2e/approval_denial_reason_test.go` | `It("keeps approval unchanged and attaches no reason")` |
| User Story 3, Scenario 1 | `tests/e2e/approval_denial_reason_test.go` | `It("rejects reason authorship and reads by unauthenticated callers, other users, and the agent")` |
| User Story 3, Scenario 2 | `tests/e2e/approval_denial_reason_test.go` | `It("keeps the first decision and reason on any later deny attempt")` |
| User Story 3, Scenario 3 | `tests/e2e/approval_denial_reason_test.go` | `It("records no reason for expired or approved approvals and keeps only the winner's reason under competing decisions")` |
| User Story 3, Scenario 4 | `tests/e2e/approval_denial_reason_test.go` | `It("returns the same decision and reason after a PostgreSQL broker restart and delivers it once", Label("docker"))` |
| User Story 3, Scenario 5 | `tests/e2e/approval_denial_reason_test.go` | `It("keeps the reason out of broker and ExtProc logs, spans, and metrics in successful and rejected journeys")` |
| User Story 3, Scenario 6 | `tests/e2e/frontend/approval_denial_reason_test.go` | `It("stores markup-like reasons as data and renders them literally")` |
| User Story 3, Scenario 7 | `tests/e2e/approval_denial_reason_test.go` | `It("blocks later calls after a delivered permanent denial without a reason while the owner can still read it")` |

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

Each frontend test also fails on any browser console message containing the submitted reason.
Capture with `E2E_CAPTURE_SCREENSHOTS=true GINKGO_FRONTEND_PROCS=1 just test-e2e-frontend`;
the harness writes `tests/e2e/frontend/coverage/screenshots/`, and the existing screenshot
workflow publishes captures to the paths above. Include each meaningful state on both denial
surfaces, using surface-specific filenames when a state occurs on both pages.

### Unit & Integration Tests

- **Domain**: `DenialReason` absence, exact retention, 500/501 code points (`é`, emoji), U+0000,
  redaction through `slog`, `%v`, `%#v`; `ToolApproval.Deny` invariants; service deny and claim
  paths, including ownership, expiry, competing decisions, and audit record content.
- **Handlers**: deny body parsing (types, size limit, error codes without echo), read models,
  sync summary fields, delivery handler status mapping.
- **Adapters**: memory and PostgreSQL deny with reason, claim once under concurrency, newest
  selection, window, `approved_at` exclusion, column lists.
- **Migration**: `048` up, down, reapply; constraint rejection of a reason on non-denied rows.
- **ExtProc**: cache candidate detection, client claim (200/204/errors, no logging of body), gate
  order (permanent with/without candidate, approved precedence, read-then-claim, fail closed),
  server rendering of the MCP result, and a safety test asserting the reason is absent from logs,
  spans, and diagnostics.
- **SPA**: `DenialReasonForm` accessibility (label, `aria-describedby` count and error, keyboard),
  count by code points, cancel discards, error retains text; both pages send `denial_reason`;
  confirmation renders literally and omits an empty heading.

**Test Coverage Goals**: All acceptance scenarios (13) in E2E; all new domain branches in unit
tests; both adapters for every new repository method.

## Approval Items

| ID | Type | Decision | Details |
|---|---|---|---|
| API-DENY | API | `POST /api/approvals/{id}/deny` accepts optional `denial_reason` (≤500 code points; blank means none; U+0000, non-string, or body >16 KiB → `400 invalid_denial_reason` without echo) and `DenyResponse` returns it | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:82-165 |
| API-READ | API | `ToolApprovalDetail` (get, pending, permanent) adds owner-visible `denial_reason`, present only when recorded | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:167-173 |
| API-SYNC | API | `ToolApprovalSummary` adds `expires_at` and nullable `denial_delivered_at`; the reason is never synchronized | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:175-187 |
| API-DELIVER | API | New gateway endpoint `POST /api/approvals/deliveries` with subject token + client assertion: claims the newest undelivered, never-approved denial for the caller's pair, tool, and arguments hash before `expires_at`; `200` with decision and reason, `204` when none | specs/20261116-181338-approval-denial-reason/contracts/enduser-approvals-openapi.yaml:11-80 |
| API-MCP | API | Agent receives the delivered denial as HTTP 200 MCP `CallToolResult` with `isError: true`, fixed text containing the reason, and `_meta["io.agentic-identity-broker/tool-approval"]`; delivery failures return 403 `approval decision could not be delivered` | specs/20261116-181338-approval-denial-reason/contracts/extproc-denied-outcome.md:9-89 |
| API-VERSION | API | End-User API `info.version` `1.2.0` → `1.3.0` | specs/20261116-181338-approval-denial-reason/research.md:256-264 |
| ADR-DELIVERY | ADR | Draft ADR 20261116-191554: denial reason on `ToolApproval`, at-most-once delivery on the agent's repeat of the original invocation via a dual-auth claim, reason excluded from sync and telemetry | specs/20261116-181338-approval-denial-reason/20261116-191554-tool-approval-denial-reason-delivery.md:1-136 |
| DEC-DELIVERY | Decision | Deliver each denied outcome to the agent at most once, on its first repeat of the original invocation before `expires_at`; a lost response is not redelivered; FR-007, FR-008, Scenarios 3.4 and 3.7, and SC-006 revised accordingly | specs/20261116-181338-approval-denial-reason/research.md:80-124 |
| DEC-TEMP | Decision | Behavior change: after a temporary denial, the agent's first repeat of the original call receives the denied outcome instead of a new approval prompt | specs/20261116-181338-approval-denial-reason/spec.md:38-40 |
| DB-048 | Decision | Migration `048`: nullable `denial_reason TEXT` and `denial_delivered_at TIMESTAMPTZ`, two check constraints, partial index; no backfill; DOWN discards recorded reasons; reasons stored as plaintext like tool arguments | specs/20261116-181338-approval-denial-reason/data-model.md:82-128 |

Implementation must not start any item above until the maintainer's 👍 on the approval comment
covers it. The implementation phase then records that approval in the ADR's Status line and moves
the draft to `adrs/20261116-191554-tool-approval-denial-reason-delivery.md`.

## Task integration details

- Scenario 3.5's backend `It()` must include a real browser denial submission before the broker
  and ExtProc checks. A test-only browser lifecycle wrapper in
  `tests/e2e/bootstrap/denial_reason_browser.go` reuses the Playwright setup patterns and captures
  browser console output; an API-only journey does not cover browser signal safety.
- When implementation moves the approved ADR into `adrs/`, leave a link-only redirect at the
  feature-local draft path so links from the frozen specification and plan still resolve.
- `tasks.md` orders the two P1 stories (US1, then US3) before the P2 compatibility story (US2).
  US1 is a candidate MVP slice, not a release that can omit privacy or compatibility gates.

## Complexity Tracking

No constitution violation needs justification.

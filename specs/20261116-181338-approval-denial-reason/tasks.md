# Tasks: Tool approval denial reasons

**Input**: `specs/20261116-181338-approval-denial-reason/`.
**Prerequisites**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/`, and
`quickstart.md`. Artifact citations below are relative to that feature directory; application
and test paths are relative to the repository root.

**Tests**: Mandatory, test-first (Principles VIII and XIII). Write the acceptance journeys before
business implementation. For each new behavior, write compiling assertions, observe a semantic
failure, implement the minimum behavior, rerun, then refactor with assertions unchanged. Minimal
compilation scaffolding is allowed; skipped/pending or placeholder tests are not. Existing
compatibility assertions may already pass and must remain enabled.

**Approval boundary**: This list authorizes no implementation. T001 uses only the implementation
session's `context.approval`; all ten items in `plan.md:312-329` remain Proposed/pending until
covered. FR-010's existing clarification acceptance does not approve the delivery design.
Do not change `spec.md` or `plan.md` during implementation. Do not edit workflows, automation,
skills, hooks, or the constitution. Record run evidence only in the session result, never here,
in `quickstart.md`, or in an implementation log.

## Format: `[ID] [P?] [Story] Description`

`[P]` identifies different-file tasks that can run concurrently after their stated prerequisites.
Story labels appear only in story phases. Execute serially in task order in the unattended worker;
parallel examples describe safe opportunities, not a requirement to use them.

## 🔒 Feature Scope [MANDATORY]

Every task cites a requirement, scenario, or binding principle. No new configuration, identity,
entity, dependency, notification channel, comment history, or telemetry metric is introduced.
Scope is the broker, both storage adapters, ExtProc, both browser denial surfaces, and their
necessary documentation and tests. Do not broaden approval matching, retention, or authentication.
Unrelated findings are not implementation tasks; only maintainer-authorized follow-up issues may
be referenced as future work. No separate refactoring PR or entity-boilerplate phase is needed
(`plan.md:193-210`). If required structural refactoring emerges, isolate it in a behavior-preserving
commit before feature commits, with existing tests passing unchanged.

## Phase 1: Setup (Shared Infrastructure)

- [ ] T001 Record the supplied maintainer approval in the Status line of `specs/20261116-181338-approval-denial-reason/20261116-191554-tool-approval-denial-reason-delivery.md` as `Accepted (approved by <login> on <date>: <url>)`, then move that approved draft to `adrs/20261116-191554-tool-approval-denial-reason-delivery.md`; verify approval coverage for every item in `plan.md:312-329`, and leave a link-only redirect at the original feature-local ADR path so frozen `spec.md`/`plan.md` links still resolve (the full decision and acceptance live only under `adrs/`) (Principles II, IV, X). If approval does not cover an item, leave its dependent tasks unchecked and continue independent work; never invent approval.
- [ ] T002 Read the applicable root/child `AGENTS.md`, `CONTRIBUTING.md`, `coding-standards/LOGGING.md`, `coding-standards/OPENTELEMETRY.md`, and accepted `004-storage-layer-architecture`, `018-approval-endpoint-auth-boundaries`, `014-long-poll-listen-notify`, `035-shared-tool-pattern-matching`, `036-extproc-approval-cache-and-gate`, and `007-e2e-testing-with-ginkgo` ADRs; confirm the selected feature and current production call paths before edits (Principles II, VI, XV; `plan.md:162-210`).

## 🔒 Phase 2: Design Preconditions (Blocking Prerequisites) [MANDATORY]

**Purpose**: Complete domain, configuration, API, database, concrete spec-journey, and E2E
design before implementation. This phase blocks business implementation; T012 permits only
minimal scaffolding needed to compile semantic-red tests.

### Phase 2a: Domain Model & Glossary [MANDATORY]

- [ ] T003 Verify `data-model.md:9-94` identifies the existing aggregate, new value object, delivery timestamp, and persisted delivery-pending flag, lifecycle invariants, and content-free ApprovalDenied event; add Denial Reason and Decision Delivery to `ARCHITECTURE.md`'s glossary and approval model, and the root `AGENTS.md` glossary (FR-005–FR-008; Principles II, V). No new typed ID or independent entity is needed.

### Phase 2b: Configuration Design [MANDATORY]

- [ ] T004 Verify the no-configuration-change boundary in `plan.md:43-74` against `internal/ports/config.go`, `internal/extproc/config/`, `docs/configuration.md`, `examples/config/README.md`, and `charts/agentic-identity-broker/`; reuse `expires_at` without adding a key, default, flag, or sample setting. Preserve ExtProc's standalone deployment ownership under `adrs/011-extproc-standalone-binary.md`; no broker Helm value/template change is applicable (Principle VII).

### Phase 2c: API Design [MANDATORY]

- [ ] T005 After T001 covers the six API items, merge `contracts/enduser-approvals-openapi.yaml:1-242` into `api/enduser/openapi.yaml` without losing existing members; document security AND semantics, 200/204/error responses, absence/validation rules, immutable-denial behavior, and version 1.3.0. Update `docs/api/approval-endpoints.md` and `docs/api/enduser-api.md` examples before endpoint implementation; retain `contracts/extproc-denied-outcome.md:9-101` as the MCP contract. Confirm no Admin API or JWT-claim change (API-001–API-004; Principles IV, X).

### Phase 2d: Database Design [MANDATORY]

- [ ] T006 Verify migration slot `048` and the paired atomic SQL design in `data-model.md:96-153` against `migrations/`; identify the two nullable columns and false-default delivery-pending flag, all three check constraints, partial index, no-backfill behavior, and intentional loss of reason/delivery/pending data on DOWN. Keep existing record identity, retention, and other columns intact; executable migrations follow their tests in T018/T026 (DB-001–DB-002; Principle IX).

### Phase 2e: Frontend/Design System Review [MANDATORY IF FRONTEND]

- [ ] T007 Review `web/src/design-system/docs/INDEX.md`, `DESIGN_PRINCIPLES.md`, `COMMON_MISTAKES.md`, `DECISION_TREES.md`, `COMPONENT_PAIRING_GUIDE.md`, and accessibility/token guides; map `spec.md:126-264,381-386` wireframes to existing TextArea, Button, status, and feedback primitives. Keep DenialReasonForm feature-local; no universal component, new visual direction, or theme is required. Specify optional label, count/error descriptions, keyboard/focus behavior, and semantic tokens before component edits (FR-001, FR-011–FR-012; Principle XI).

### Phase 2f: E2E Acceptance Test Design [MANDATORY]

- [ ] T008 Review all three concrete journeys in `spec.md:120-308`, HTTP/MCP examples, and editable Wireloom sources/rendered SVGs in `specs/20261116-181338-approval-denial-reason/wireframes/` against approved contracts; retain the 13-scenario mapping below, with no new configuration or output JWT. Ensure each scenario has exactly one canonical `It()` and a nearby specification/story/scenario comment (Principles XIII, XIV; `plan.md:214-284`).
- [ ] T009 Extend only necessary test wrappers/fixtures in `tests/e2e/bootstrap/extproc.go`, `tests/e2e/bootstrap/storage.go`, `tests/e2e/helpers/approval_types.go`, `tests/e2e/extproc/helpers/`, and `tests/e2e/fixtures/` for broker+ExtProc calls, denial-result parsing, captured logs/spans/metrics, and durable restart; expose these test-only captures to the frontend privacy journey without adding a browser lifecycle outside the frontend suite. Reuse production constructors, isolated per-test storage, deterministic fixtures, bounded waits, and existing PostgreSQL integration bootstrap; do not add test-only production APIs (Principles VI, VIII, XIII; `plan.md:244-256`).
- [ ] T010 Write `tests/e2e/approval_denial_reason_test.go` with the ten backend `It()` blocks mapped below, including the `Label("docker")` restart journey; assert real HTTP responses, exact reason/decision pairing, MCP response, no execution for denied outcomes (authorized overlap variants proceed), atomic winners, unauthorized rejection, no-content telemetry, and later-call behavior. Include Scenario 1.1 overlap/race variants (temporary denial vs synchronized valid approval, permanent denial still first, no approved-hot-path claim) and Scenario 3.7 variants (permanent deny → revoke → identical repeat before expiry, revoked grant separately, and both claim/revoke lock orders). Use BeforeEach/AfterEach, fresh fixtures, complete production journeys, and the existing approval/ExtProc bootstrap (all backend scenarios; Principles VIII, XIII).
- [ ] T011 Write `tests/e2e/frontend/approval_denial_reason_test.go` with the three canonical browser `It()` blocks for Scenarios 1.2, 3.5, and 3.6; extend `tests/e2e/pages/approval_page.go` and `tests/e2e/pages/tool_authorizations_page.go`. Exercise both pages and temporary/permanent denial, cancel, validation/correction including 501 whitespace code points normalized to absence, confirmation/reload, literal markup, and unchanged approval controls; fail on reason-bearing console output. Scenario 3.5 alone owns the full successful/rejected browser → production broker → in-process ExtProc privacy journey: reuse the existing frontend suite browser/context lifecycle, attach broker/gateway log/span/metric capture via test-only bootstrap, and assert permitted audit metadata but no reason text in any operational signal, including claim failures. Extend `tests/e2e/frontend/frontend_suite_test.go` only if capture injection needs it, keeping lifecycle ownership there. Add unique surface-specific screenshot calls for pending, editing, validation, and recorded states plus literal-markup display (FR-001–FR-003, FR-011–FR-012, SR-001, SR-004; Principles XI, XIII).
- [ ] T012 Run semantic-red acceptance tests from `tests/e2e/approval_denial_reason_test.go` and `tests/e2e/frontend/approval_denial_reason_test.go` before business implementation, using `just test-e2e-backend` and `just test-e2e-frontend`; add only minimal compiling fields/interfaces/stubs if needed. No placeholder assertions, pending markers, skips, or red-phase comments. Container-backed Scenario 3.4 first runs in container CI when no local runtime exists; do not attempt container setup (Principles VIII, XIII).

### Phase 2g: Observability Design [MANDATORY IF TELEMETRY]

- [ ] T013 Verify `contracts/observability.md:8-77` against both coding standards; amend `coding-standards/LOGGING.md` §6.1 to prohibit user-authored denial reasons explicitly and document that amendment's rationale in `docs/api/approval-endpoints.md`. Confirm the claim audit, span names/kinds/attribute sets, no new metric or namespace, and all credential/free-text safety-test paths before instrumented code. Do not add principal span attributes while #356 is unavailable; do not alter existing metric/event contracts or automated checks (SR-001–SR-003; Principle XV).
- [ ] T014 Verify T003–T013 and T001's sign-off coverage against `specs/20261116-181338-approval-denial-reason/checklists/requirements.md` and `plan.md:76-139`; distinguish specification-quality checks from implementation completion. Leave unfulfilled checklist items unchecked; do not change the frozen spec/plan or store execution evidence (Principles I–XV).

**Checkpoint**: Domain/configuration/API/schema/UI/observability design and semantic acceptance tests
are ready. Migration SQL is implemented only after migration tests. No new-entity boilerplate or
unrelated foundational infrastructure is needed.

## Phase 3: User Story 1 - Explain a denial to the requesting agent (Priority: P1) 🎯 MVP

**Goal**: The owner records an optional exact reason, and the agent's repeat receives the original
denied outcome once, without executing the tool.

**Independent Test**: Scenarios 1.1–1.3: create through the gateway, deny via the owner, repeat the
original MCP call, and verify exact text and structured denied metadata, no downstream execution,
500/501 boundaries, and both browser entry points. This MVP candidate is not releasable until
US3 privacy/immutability and US2 compatibility gates pass.

### Tests for User Story 1 [MANDATORY - Principle VIII] ⚠️

- [ ] T015 [P] [US1] Add table-driven `DenialReason` tests in `internal/domain/storage/denial_reason_test.go`: absence `"\"\" or only unicode.IsSpace runes (including 501 code points) → nil, nil"`, retention `"Otherwise the exact raw string, including surrounding and internal whitespace"`, limit `"utf8.RuneCountInString(raw) <= 500"`, U+0000 rejection, emoji/non-ASCII boundaries, and `LogValue()`, `String()`, `GoString()` returning `[REDACTED]`; assert constant errors never contain text (FR-002–FR-003, SR-001; `data-model.md:59-72`).
- [ ] T016 [P] [US1] Extend `internal/domain/storage/tool_approval_test.go` for the constraints `"Set only by Deny, in the same transition as Status=denied. Never set for pending or approved records. Never changed afterwards."` and delivery eligibility `"Status == denied, ApprovedAt == nil, DenialDeliveredAt == nil, DenialDeliveryPending == true, now < ExpiresAt"` with exact principal/agent/tool/hash equality. Include permanent denial revocation suppressing unclaimed delivery without reason mutation, revoked grants separately, claim/revoke races in both lock orders, and permanent parameter clearing and no mutation on invalid transitions (FR-004–FR-006; `data-model.md:9-57`).
- [ ] T017 [P] [US1] Extend `internal/adapters/storage/memory/tool_approval_repository_test.go` for atomic deny+reason, copies isolated from callers, newest qualifying claim, exact invocation scoping, strict expiry, one claim only, revoke-before-claim suppression vs claim-before-revoke retention, and sync version publication; historical rows have nil reason/delivery fields and false delivery-pending (never infer historical outcomes) (FR-005, FR-007–FR-008; DB-001; `data-model.md:84-94,155-170`).
- [ ] T018 [P] [US1] Add `tests/integration/migrations/approval_denial_reason_test.go` before migration SQL: apply production migrations through 047, seed representative approvals, apply 048, verify columns/constraints/index, NULL historical reason/delivery fields and false delivery-pending, rejected non-denied/over-limit reasons and invalid delivery/pending states (pending delivery is forbidden on approved or already-delivered records), DOWN and reapply. Assert old data survives except the three intentionally discarded columns; use shared cloned PostgreSQL, explicit SQL projections, and the integration build tag (DB-002; `data-model.md:96-153`).
- [ ] T019 [P] [US1] Extend `internal/adapters/storage/postgres/tool_approval_repository_test.go` for deny round-trip and claim behavior using production migrations, separate connections, concurrent claims and competing decisions, newest selection/tie-break, exact identity/tool/hash, never-approved and revoked-record exclusion, permanent denial revoke-before-repeat vs previous-grant revocation, both claim/revoke lock orders, expiry, no-match, StorageError/timeout handling, and sync version/notification behavior; keep container-dependent cases integration-tagged (DB-001; FR-005–FR-008; `data-model.md:20-40,84-153`).
- [ ] T020 [P] [US1] Extend `internal/domain/approval/service_test.go` before changing services: deny passes the reason in one repository call; owner detail/list projection retains it; claim derives the canonical arguments hash, returns own decision or nil, publishes sync only after mutation, and audits committed claims without text. Include denied/no-match/storage-failure span behavior and no successful audit on failure/no-match (FR-004–FR-008, SR-001–SR-003; `data-model.md:74-94`; `contracts/observability.md:21-47`).
- [ ] T021 [P] [US1] Extend `internal/adapters/http/handlers/approval/deny_handler_test.go`, `get_handler_test.go`, and `sync_handler_test.go`, and add `delivery_handler_test.go`: exact JSON field placement, omission, nonblank 500/501 and 501 whitespace code points normalized to absence before length validation, non-string/null/U+0000/16-KiB bounds, constant 400 errors, pending state after rejection, detail/lists owner projection, sync metadata without reason, and 200/204/auth/failure delivery mapping, including constant `500 internal_error`/`failed to deliver approval decision` for storage timeout, connection failure, and unexpected service failure without dependency/reason echo (API-001–API-003; `contracts/enduser-approvals-openapi.yaml:11-242`).
- [ ] T022 [P] [US1] Extend `internal/extproc/approval/cache_test.go`, `client_test.go`, and `gate_test.go`, plus `internal/extproc/server/server_test.go`, before delivery code: fresh-only unrevoked candidate detection, dual-auth claim, strict 200/204 decoding, malformed/error/transport failures, gate order and call counts, preserved approved hot path when temporary denial overlaps a matching approval, permanent-denial precedence even over approval, synchronized grant/revocation races and stale-snapshot revoke-before-claim suppression, immutable request JSON-RPC ID, exact MCP content/meta, absent structuredContent, and no forwarding/new approval on delivery or failure (FR-007, FR-009–FR-010; `research.md:149-205`; `contracts/extproc-denied-outcome.md:9-101`).
- [ ] T023 [P] [US1] Add `web/src/components/approvals/DenialReasonForm.test.tsx`, extend `ApprovalReviewPage.test.tsx`, and add/extend `web/src/pages/ToolAuthorizationsPage.test.tsx`, `web/src/hooks/useApproval.test.ts`, and `web/src/services/api/approvals.test.ts`: optional label/count/notice, absence normalization before nonblank code-point validation, 501 whitespace code points accepted on both surfaces (including U+0085, unlike JavaScript trim), U+FEFF nonblank, keyboard/cancel, error retains input, submit preserves exact text and scope, and read-only result/reload. Mock services, not hooks (FR-001–FR-003, FR-011–FR-012; `research.md:278-295`; `spec.md:381-386`).

### Implementation for User Story 1

- [ ] T024 [US1] Implement `DenialReason` in `internal/domain/storage/denial_reason.go` and extend `internal/domain/storage/tool_approval.go` with reason, delivery timestamp, and delivery-pending fields and denial/deliverability invariants, using T015–T016 red tests. Preserve exact nonblank text, immutable optional value semantics, constant sentinel errors, redacted formatters, and existing authorization meaning while persisting revocation suppression without reason mutation (FR-002–FR-006; `data-model.md:9-72`).
- [ ] T025 [US1] Extend `internal/ports/storage.go` with the reason-bearing Deny signature and narrow `ToolApprovalDeliveryRepository.ClaimDenialDelivery` (`nil, nil` on no match); expose/wire the capability in `internal/adapters/storage/factory.go` for both backends. Mechanically update existing calls, service requests, and test doubles to compile without changing their expectations; no domain-to-adapter dependency (FR-005, DB-001; `data-model.md:84-94`; Principles VI, IX).
- [ ] T026 [US1] Create atomic `migrations/048_add_tool_approval_denial_reason.up.sql` and `.down.sql` from `data-model.md:96-153`, after T018 is written. Add `denial_reason TEXT NULL`, `denial_delivered_at TIMESTAMPTZ NULL`, `denial_delivery_pending BOOLEAN NOT NULL DEFAULT FALSE`, `CHECK (denial_reason IS NULL OR (status = 'denied' AND char_length(denial_reason) BETWEEN 1 AND 500))`, `CHECK (denial_delivered_at IS NULL OR status = 'denied')`, `CHECK (NOT denial_delivery_pending OR (status = 'denied' AND approved_at IS NULL AND denial_delivered_at IS NULL))`, and the documented partial index excluding revoked records. No backfill or no-transaction marker; DOWN drops only the new objects/columns (DB-002; Principle IX).
- [ ] T027 [US1] Implement atomic deny+reason and ClaimDenialDelivery in `internal/adapters/storage/memory/tool_approval_repository.go` using T017: write-lock decisions, strict window/matching, newest denied_at then ID tie-break, copied results, set delivery-pending only on a new denial, set delivery time once, atomically clear DenialDeliveryPending on claim or permanent revocation under the same lock without changing the reason or prior delivery time, and publish the existing sync mutation exactly once (FR-005–FR-008; `data-model.md:20-40,155-170`).
- [ ] T028 [US1] Implement the same semantics in `internal/adapters/storage/postgres/tool_approval_repository.go` using T019/T026: conditional deny writes status/reason and delivery-pending=true together, claim uses one locked UPDATE/RETURNING, excludes previously approved and revoked records; permanent revocation atomically clears denial_delivery_pending with the existing status/scope update under the same row lock, retaining reason and prior delivery time, and retains timeouts/StorageError handling and trigger-driven sync. Extend every full-row result/scan with explicit column lists and both nullable fields and the delivery-pending flag, preserving existing stored/returned fields; use `startStorageSpan` for the new method (FR-005–FR-008; DB-001; `data-model.md:96-153`; `contracts/observability.md:33-47`).
- [ ] T029 [US1] Extend `internal/domain/approval/service.go` using T020: DenyRequest carries Persistence and Reason, DenyApproval preserves owner/state checks, ApprovalDetail carries the optional reason, and ClaimDenialDelivery computes the creation-compatible hash, delegates atomic claim through its port, publishes the mutation, and returns own decision or nil. Add the content-free delivery audit and `approval.deliver` span after commit, never a success audit for no-match (FR-004–FR-008, SR-001–SR-003; `data-model.md:74-94`; `contracts/observability.md:21-47`).
- [ ] T030 [US1] Implement deny parsing/response and owner detail/list projection in `internal/adapters/http/handlers/approval/deny_handler.go`, `get_handler.go`, `pending_handler.go`, and `permanent_handler.go` using T021. Bound bodies at 16 KiB independently of content, normalize all-whitespace to absence before nonblank length validation (accept 501 whitespace code points), distinguish reason type errors from malformed JSON/persistence errors, map invalid reasons to constant `400 invalid_denial_reason` without echo, and serialize optional text explicitly only on authorized decisions (API-DENY, API-READ; FR-002–FR-006; `contracts/enduser-approvals-openapi.yaml:101-202`).
- [ ] T031 [US1] Add `internal/adapters/http/handlers/approval/delivery_handler.go` and extend `sync_handler.go` using T021: parse tool/arguments, use only authenticated principal/agent context, call the domain service, return specified 200/204/error envelopes with constant 500 internal_error on claim/storage timeout, connection, or unexpected service failure; never expose raw errors or reset a committed claim, and synchronize expires_at/denial_delivered_at/denial_delivery_pending but never the reason (API-DELIVER, API-SYNC; `contracts/enduser-approvals-openapi.yaml:11-99,204-242`).
- [ ] T032 [US1] Inject the delivery port, service, and handler in `internal/app/builder.go`, add the handler to `internal/app/handlers.go`, and register the static `/api/approvals/deliveries` route in `internal/adapters/http/routing/enduser.go` using the create endpoint's dual-auth middleware and agent-subject rejection; preserve browser-owner, sync, consume, CSRF, and dual-server boundaries. Update only necessary production-bootstrap wrappers (SR-003; Principles VI, XII; `research.md:149-179`).
- [ ] T033 [US1] Extend `internal/extproc/approval/cache.go` and `client.go` using T022 with ExpiresAt/DenialDeliveredAt/DenialDeliveryPending, copied nullable timestamps, fresh unrevoked pair/tool candidate lookup, delivery DTOs, and ClaimDelivery with subject token + client assertion. Require validated 200 decision fields, handle 204 as no-match, and return fixed classified errors without response bodies; retain redirects/timeouts/cancellation behavior and no reason cache (FR-007–FR-008, SR-001; `data-model.md:163-170`; `research.md:149-205`).
- [ ] T034 [US1] Extend `internal/extproc/approval/gate.go` using T022: permanent block first with candidate claim, fresh approved match without claim even when a temporary denial overlaps, then existing authoritative read and re-evaluation before candidate claim/create. Never claim revoked records; a stale candidate can only obtain 204 and existing stale-snapshot authorization bounds still apply. Claim 200 returns Outcome.Denied; 204 preserves permanent block or create/elicit; any failure denies with `approval decision could not be delivered` and creates nothing. Preserve freshness, ETag regression, identity, once reservations, and unchanged gate metric outcomes (FR-007, FR-009–FR-010; `research.md:181-205`; `contracts/observability.md:33-57`).
- [ ] T035 [US1] Extend `internal/extproc/server/server.go` using T022 to render Outcome.Denied as the specified immediate HTTP 200 JSON-RPC MCP CallToolResult, retaining the original request ID, exact fixed text plus optional reason, isError=true, structured metadata, and absent structuredContent; JSON-encode text as data. Do not forward the tool request, create another approval, or emit content to diagnostics (FR-007, SR-004; `contracts/extproc-denied-outcome.md:28-94`).
- [ ] T036 [US1] Extend `web/src/types/approval.ts`, `web/src/services/api/approvals.ts`, `web/src/hooks/useApproval.ts`, and `web/src/pages/ApprovalPage.tsx` using T023 to carry optional reason input/output and invalid_denial_reason feedback, preserving permanent scope and unchanged approval requests. Keep validation errors inline with entered text intact, never put text in a URL/log, and retain read-only data after successful denial/reload (FR-001–FR-003, FR-011–FR-012; `data-model.md:172-175`; `research.md:58-85,278-295`).
- [ ] T037 [US1] Implement `web/src/components/approvals/DenialReasonForm.tsx` with existing design-system primitives using T023: optional label, code-point count, normalize all-whitespace to absence before nonblank length validation using R3's Go-unicode.IsSpace-compatible set (accept 501 whitespace code points, not JavaScript trim/\s semantics), announced limit/error and sharing notice, keyboard submit/cancel, retained invalid input, no truncation, no send on cancel, and scope passed independently from reason. Do not use a UTF-16 maxlength as the authoritative 500-code-point validator (FR-001–FR-003, FR-011; `research.md:278-295`; `spec.md:381-386`).
- [ ] T038 [US1] Integrate the shared form and literal read-only explanation in `web/src/components/approvals/ApprovalReviewPage.tsx`, `ApprovalConfirmation.tsx`, and `web/src/pages/ToolAuthorizationsPage.tsx`. Cover temporary and permanent denial on both surfaces, clear unsubmitted text on cancel, omit absent-reason headings, and preserve approval controls/scopes (FR-001, FR-011–FR-012; `spec.md:126-290`; `research.md:278-295`).
- [ ] T039 [US1] Rerun T015–T023 package tests and `tests/e2e/approval_denial_reason_test.go` with `just test`, `just web-test`, and `just extproc-test`, then Scenarios 1.1–1.3 via `just test-e2e-backend` and `just test-e2e-frontend`. Verify exact 500-code-point round-trip (including emoji/non-ASCII), 501 rejection/corrected retry, cancellation on both surfaces, and zero execution for denied outcomes while authorized overlap variants proceed; refactor only with unchanged meaningful assertions in the cited test files (SC-001–SC-003; Principles VIII, XIII).

## Phase 4: User Story 3 - Protect the author's decision and private text (Priority: P1)

**Goal**: Reasons cannot be forged, overwritten, detached from their winning decision, or exported
through operational signals, and never accompany later permanently blocked invocations.

**Independent Test**: Scenarios 3.1–3.7 from fresh approval journeys, including unauthorized
attempts, retries/races, owner reread/durable restart, captured operational signals, literal UI
rendering, and subsequent same/different-argument permanent blocks.

### Tests for User Story 3 [MANDATORY - Principle VIII] ⚠️

- [ ] T040 [P] [US3] Add broker safety tests in `internal/domain/approval/service_security_test.go` and `internal/adapters/http/handlers/approval/denial_reason_security_test.go` before any additional confidentiality changes: inject JSON log buffers and span recorders; test successful, invalid, unauthorized, expired, repeated, and storage-failed deny/claim paths, error strings, event metadata, and principal-policy output. Include pending/approved/revoked records, no-match, and owner vs other-user/other-agent reads; never expose text via generic DTO formatting (SR-001–SR-003; Scenarios 3.1–3.5; `contracts/observability.md:59-77`).
- [ ] T041 [P] [US3] Add `internal/extproc/approval/denial_reason_security_test.go` and `internal/extproc/server/denial_reason_safety_test.go` before additional gateway protections: synthetic reasons and credentials in 200/error/malformed bodies, lost response, timeout/cancellation, two gateway claim attempts, all span/event/status/log surfaces, and metric attributes through ManualReader. Assert only the successful claimant receives its decision, delivered denial counts as deny, no-cache/no-log behavior, later same/different-argument permanent blocks carry no reason, and stale snapshots cannot redeliver (FR-008–FR-010, SR-001; `contracts/extproc-denied-outcome.md:88-110`; `contracts/observability.md:49-77`).
- [ ] T042 [P] [US3] Extend `tests/e2e/approval_api_test.go` only where needed for the new delivery boundary, preserving its old scenarios; add routing/auth cases for missing subject, missing assertion, invalid credentials, agent-subject rejection, auth unavailability, and wrong authenticated pair without text disclosure. Exercise production middleware and handler registration, not a direct port (SR-003; `contracts/enduser-approvals-openapi.yaml:11-99`; Principles I, XII).
- [ ] T043 [P] [US3] Add `web/src/components/approvals/ApprovalConfirmation.test.tsx` and extend `web/src/components/approvals/ApprovalReviewPage.test.tsx` for `<b>Use staging</b>` and executable-looking strings rendered as literal text, immutable historical reason, no edit control, no empty heading, and no reason in console/error telemetry. Mock API services, not hooks; use Scenario 3.6's browser journey from T011 for real DOM verification (FR-006, FR-012, SR-001, SR-004; `spec.md:86-118,381-386`).

### Implementation for User Story 3

- [ ] T044 [US3] Complete confidentiality/error-path handling in `internal/domain/approval/service.go`, `internal/adapters/http/handlers/approval/`, and both `internal/adapters/storage/{memory,postgres}/tool_approval_repository.go` against T016–T021/T040: preserve the first reason (including none), winner-only decisions, never-approved/unrevoked claim filter, atomic clearing of denial_delivery_pending on permanent revocation under the claim lock, unchanged reason/prior delivery time on revocation, strict window, non-content error/audit/span metadata, and post-commit-only delivery audit. Keep corrections limited to feature-touched paths and explicit SQL columns (FR-004–FR-008, SR-001–SR-003; `data-model.md:20-57`; Principle XV).
- [ ] T045 [US3] Complete `internal/extproc/approval/{cache,client,gate}.go` and `internal/extproc/server/server.go` against T041: no content retention/diagnostics or raw response errors, fail-closed claim failure, cross-instance at-most-once enforcement at the broker, lost-response non-redelivery, revoked-candidate suppression and permanent-denial/valid-approval precedence over temporary denial, preserved later permanent block and no additional approved-hot-path broker call. Apply binding logging/tracing rules to any touched instrumentation, without renaming or widening the existing metric (FR-007–FR-010, SR-001–SR-004; `research.md:181-205`; `contracts/observability.md:8-57`).
- [ ] T046 [US3] Complete literal display/privacy handling in `web/src/components/approvals/ApprovalConfirmation.tsx`, `ApprovalReviewPage.tsx`, `web/src/pages/ToolAuthorizationsPage.tsx`, and the API error path against T043/T011: React text only, no Markdown/HTML execution, no mutable historical reason, no console or URL export, and recorded reason still available to the owner after reload (FR-006, FR-012, SR-001, SR-004; `research.md:278-295`).
- [ ] T047 [US3] Document one-time delivery/lost-response limitations, immutable reasons, and the mandatory no-MCP-response-body-logging operator control in `docs/guides/token-exchange-gateway.md` and `docs/api/approval-endpoints.md`; update approval authentication/delivery guidance in `internal/adapters/AGENTS.md` and `internal/extproc/AGENTS.md` without editing accepted ADRs (FR-008–FR-010, SR-001–SR-003; `research.md:87-205,265-276`).
- [ ] T048 [US3] Run `just check`, `just test`, `just extproc-test`, `just web-test`, `just test-e2e-backend`, and screenshot-enabled frontend E2E against the security assertions in T010–T011/T040–T043; verify Scenarios 3.1–3.3 and 3.5–3.7 green. Prepare Scenario 3.4 in `tests/e2e/approval_denial_reason_test.go` for `just test-e2e-docker` plus PostgreSQL races/migration tests for `just test-integration-infra` in container CI, never report them locally validated (SC-004–SC-007; Principles VIII, IX, XIII).

## Phase 5: User Story 2 - Keep reason-free decisions simple (Priority: P2)

**Goal**: Existing clients can deny without a body/reason, blank text means absence, and approval
adds no step or input and retains all persistence/permission behavior.

**Independent Test**: Scenarios 2.1–2.3 each start from a separate pending approval; the first two
reach reason-free denied MCP outcomes, while the third follows existing approve/consume/proceed.

### Tests for User Story 2 [MANDATORY - Principle VIII] ⚠️

- [ ] T049 [P] [US2] Extend `internal/adapters/http/handlers/approval/deny_handler_test.go` and `internal/domain/approval/service_test.go` for bodiless/omitted/null/empty/Unicode-whitespace-only input including 501 code points within the independent body bound, legacy denied rows, no empty reason property, repeated reason-free decision immutability, and unchanged approve/consume/revoke behavior; retain the three canonical acceptance tests from T010, not duplicate scenario It blocks (FR-003, FR-006, FR-009; `spec.md:72-84,268-290`; `research.md:40-85`).
- [ ] T050 [P] [US2] Extend `web/src/components/approvals/DenialReasonForm.test.tsx`, `ApprovalReviewPage.test.tsx`, and `web/src/pages/ToolAuthorizationsPage.test.tsx` for optional omission/whitespace-before-length semantics including 501 whitespace code points on both pages and approval with no reason-entry step or added payload, including once/session/permanent choices and absent-reason confirmations on both pages (FR-003, FR-009, FR-012; `spec.md:72-84,268-290`).

### Implementation for User Story 2

- [ ] T051 [US2] Resolve only compatibility gaps exposed by T049–T050 in `internal/adapters/http/handlers/approval/deny_handler.go`, `internal/domain/storage/denial_reason.go`, `web/src/hooks/useApproval.ts`, `web/src/components/approvals/DenialReasonForm.tsx`, and `web/src/pages/ToolAuthorizationsPage.tsx`: blank input maps to no reason before nonblank length validation (501 whitespace code points remain valid within the body bound), no-body clients stay valid, no reason invented for historical records, and approve takes no reason. Preserve existing approval permission/matching semantics (FR-003, FR-009; `data-model.md:59-72`; `spec.md:72-84`).
- [ ] T052 [US2] Run `just test`, `just web-test`, `just test-e2e-backend`, and `just test-e2e-frontend` against Scenarios 2.1–2.3 and existing approval regression journeys in `tests/e2e/approval_api_test.go` and `tests/e2e/extproc_approval_journey_test.go`; verify no empty reason field/heading, no extra approve step, and normal proceed/consume behavior (SC-002; Principles VIII, XIII).

## 🔒 Phase N: Constitution Compliance Verification [MANDATORY COMPLIANCE SECTION]

**Purpose**: Verify constitution requirements and feature-scope compliance. Application completion
requires all non-container checks below. Container-backed tests must be written, but their runs
are deferred to container CI in the sandbox; that deferral must not hold up independent tasks.

### 🔒 Constitution Compliance Verification [MANDATORY]

#### Design Phase Verification [MANDATORY]

- [ ] T053 Verify all task/requirement/scenario links, the approved scope, `ARCHITECTURE.md` glossary, `api/enduser/openapi.yaml`, migration design, UI primitives, and `contracts/observability.md`; confirm T001's actual approval covers all ten items, no API/ADR/decision outside it was implemented, and no unapproved ADR is marked Accepted (Principles II, IV, V, X, XV).
- [ ] T054 Verify `spec.md:120-308` examples and `specs/20261116-181338-approval-denial-reason/wireframes/` still match implemented flows, all 13 scenarios map 1:1 below, tests preceded new behavior with realistic semantic assertions and minimal fixture-only changes, and no placeholder/pending/skip artifacts or correctness-validation Bash scripts were introduced (Principles VIII, XIII, XIV).

#### Implementation Phase Verification [MANDATORY]

**Feature Scope** (Development Requirements):

- [ ] T055 Review changed paths against `specs/20261116-181338-approval-denial-reason/tasks.md` and exclude unrelated remediation, dependency/configuration additions, retained execution logs, and unapproved follow-ups; any authorized issue links belong only in Future Work, not implementation tasks (Feature Scope & Deferred Findings).

**API & Documentation** (Principles IV, X):

- [ ] T056 Verify actual HTTP/MCP fields, statuses/errors, authentication, absence/length/body-limit behavior, owner reads, sync exclusion, and version exactly match `api/enduser/openapi.yaml`, `contracts/extproc-denied-outcome.md`, `docs/api/approval-endpoints.md`, and `docs/api/enduser-api.md`; retain existing clients' contracts (API-001–API-004; Principles IV, X).

**Architecture & Documentation** (Principle II):

- [ ] T057 Update the approval endpoint inventory, one-time delivery/window, gate order, sync fields, persistence, glossary, and ADR index in `ARCHITECTURE.md`; update the root `AGENTS.md` ADR index and the feature source map as needed. Verify `adrs/20261116-191554-tool-approval-denial-reason-delivery.md` agrees with final code and contains only the supplied approval; leave all main-merged ADRs unchanged (Principles II, V).

**Configuration** (Principle VII):

- [ ] T058 Verify no configuration/CLI/default/sample addition was made in `internal/ports/config.go`, `internal/extproc/config/`, `docs/configuration.md`, or `examples/config/`, and no unnecessary Helm values/templates were added under `charts/agentic-identity-broker/`; confirm the documented standalone ExtProc boundary and reused expires_at window remain accurate (Principle VII; FR-008–FR-009).

**Database & Persistence** (Principle IX):

- [ ] T059 Verify both adapters, the narrow ISP/factory exposure in `internal/ports/storage.go` and `internal/adapters/storage/factory.go`, explicit SQL projection/scans, atomic deny+claim and claim/revocation serialization, sync trigger behavior, migration numbering/UP/DOWN, no backfill, and all PostgreSQL integration tests in `tests/integration/migrations/approval_denial_reason_test.go` and `internal/adapters/storage/postgres/tool_approval_repository_test.go`. Defer execution to container CI in the sandbox, not test implementation (DB-001–DB-002; Principle IX).

**Security** (Principles I, III):

- [ ] T060 Verify existing authentication, CSRF, expiry, ownership, retention, revocation, and policy guarantees remain intact in `internal/adapters/http/routing/enduser.go`, `internal/domain/approval/service.go`, and `internal/extproc/approval/gate.go`; confirm no custom crypto, plaintext fallback, content-bearing diagnostics, or optional fail-open delivery path (SR-001–SR-004; Principles I, III).

**Coding Standards** (Principle XV):

- [ ] T061 Apply both coding-standard review checklists to every touched log/span/metric in `internal/domain/approval/`, `internal/adapters/storage/`, and `internal/extproc/`; verify typed principal policy, constant classified errors, redacted value object, content-free committed audit, documented spans, unchanged gate metric cardinality, and leak tests. Verify the LOGGING.md amendment is described with its rationale in the PR's implementation report and no lint/check suppression or new unregistered namespace/instrument exists (SR-001–SR-002; Principle XV; `contracts/observability.md:8-77`).

**Architecture Patterns** (Principle VI):

- [ ] T062 Verify domain owns validation/eligibility, handlers parse/present only, all broker DI is in `internal/app/builder.go`, handler groups are in `internal/app/handlers.go`, and routing registers prewired handlers without port bypass; verify `internal/extproc/` has no broker imports beyond the sanctioned toolpattern package and no duplicated argument hashing (Principles VI, XII; `data-model.md:74-94,163-170`).

**Testing** (Principle VIII - Unit & Integration Tests):

- [ ] T063 Run `just check`, `just test`, `just extproc-test`, `just web-test`, and `just test-integration` for the complete implemented change in `internal/`, `web/`, and `tests/`; fix caused failures without weakening assertions, confirm race coverage and meaningful new branch/error coverage, and report exact outcomes only in the session result (Principle VIII; `CONTRIBUTING.md:67-102`).

**E2E Acceptance Testing** (Principle XIII):

- [ ] T064 Run `just test-e2e-backend` and `GINKGO_SKIP='Agentgateway|agentgateway' just test-e2e-extproc` for the non-container production journeys in `tests/e2e/` and `tests/e2e/extproc/`; verify hierarchy, isolated fixtures/cleanup, scenario comments, full request/decision/retry outcomes, no forwarded denied tools, no broker-call regression on approved hot paths, and no content leakage. The selection excludes only the existing named container groups described in `quickstart.md`; do not change/disable tests (Principles VIII, XIII).
- [ ] T065 Run `E2E_CAPTURE_SCREENSHOTS=true GINKGO_FRONTEND_PROCS=1 just test-e2e-frontend` for `tests/e2e/frontend/approval_denial_reason_test.go` and frontend regressions; inspect surface-specific pending/entry/validation/recorded/literal screenshots in `tests/e2e/frontend/coverage/screenshots/` and preserve existing publication to `tests/e2e/screenshots/`. Verify optional labels, announcements, focus, keyboard controls, literal text, console safety, and Scenario 3.5 browser/broker/ExtProc log/span/metric privacy assertions in the production bundle (Principles XI, XIII).
- [ ] T066 Ensure `tests/e2e/approval_denial_reason_test.go` Scenario 3.4 and all PostgreSQL tests are ready for `just test-e2e-docker` and `just test-integration-infra` on the pushed commit; in the sandbox do not run them, probe a runtime, or mark them validated. Report both suites and the existing Agentgateway container groups deferred to container CI, preserving all assertions and continuing independent tasks (Principles VIII, IX, XIII).

**Frontend** (Principle XI - if applicable):

- [ ] T067 Verify `web/src/components/approvals/DenialReasonForm.tsx`, `ApprovalConfirmation.tsx`, `ApprovalReviewPage.tsx`, and `web/src/pages/ToolAuthorizationsPage.tsx` use design-system primitives and semantic tokens in the single light theme, meet 4.5:1 text/3:1 control contrast, render reasons only as text, and preserve approval actions. Confirm no universal component requires a design-system Storybook contribution; add stories/accessibility checks only if one was actually introduced (Principle XI; `spec.md:381-386`).

**Full verification gate** (CONTRIBUTING.md):

- [ ] T068 Complete every non-container recipe from `just verify` in `justfile`: checks/tests in T063–T065 plus `just cdk-test`, `just mock-sample-agent-test`, and `just mock-upstream-oauth2-test`; use the workflow-prepared `bash "$SECURITY_SCAN_SCRIPT" .` in place of `just security`. Report actual passed commands, failures, deferred container suites, and remaining work only in the session result; never claim full validation before deferred container runs pass (Principles VIII, XIII, XV).

## Acceptance coverage

Each row is one canonical `It()`; substeps/variants within it remain the same scenario. T010
owns backend writing, T011 browser writing, and the story checkpoint validates it. The descriptive
test names from `plan.md` are retained; add source line numbers after tests exist, without
recording run results in the feature directory.

| Scenario | Canonical test file | Acceptance assertions | Write / validate tasks |
|---|---|---|---|
| US1.1 | `tests/e2e/approval_denial_reason_test.go` | Eligible denial → exact agent reason, same approval, no execution; overlap/race variants preserve matching approval precedence over temporary denial and permanent-denial precedence | T010 / T039 |
| US1.2 | `tests/e2e/frontend/approval_denial_reason_test.go` | Both pages, both scopes, optional input/count/notice, cancel, validation/correction, recorded/reload states | T011 / T039, T065 |
| US1.3 | `tests/e2e/approval_denial_reason_test.go` | 500 code points intact through MCP; 501 rejected while pending; corrected retry succeeds | T010 / T039 |
| US2.1 | `tests/e2e/approval_denial_reason_test.go` | Bodiless deny → denied result without a reason | T010 / T052 |
| US2.2 | `tests/e2e/approval_denial_reason_test.go` | Whitespace-only input including 501 Unicode code points within body bound → denied result identical to absence | T010 / T052 |
| US2.3 | `tests/e2e/approval_denial_reason_test.go` | Existing approve/persistence/consume/proceed without a reason | T010 / T052 |
| US3.1 | `tests/e2e/approval_denial_reason_test.go` | Unauthenticated, other owner, agent cannot author or retrieve unrelated text | T010 / T048 |
| US3.2 | `tests/e2e/approval_denial_reason_test.go` | First reason (including absent) survives attempted add/edit/remove/retry | T010 / T048 |
| US3.3 | `tests/e2e/approval_denial_reason_test.go` | Expired/approved deny rejected; competing decisions leave only winner's reason | T010 / T048 |
| US3.4 | `tests/e2e/approval_denial_reason_test.go` (`docker`) | Owner reread and at-most-once agent delivery retain original reason across durable broker restart | T010 / T066, container CI |
| US3.5 | `tests/e2e/frontend/approval_denial_reason_test.go` | Full successful/rejected browser → broker → ExtProc journeys contain no reason in any operational signal; metadata audits remain | T011 / T048, T065 |
| US3.6 | `tests/e2e/frontend/approval_denial_reason_test.go` | Literal markup survives authorized recording/delivery/display, never executes | T011 / T048, T065 |
| US3.7 | `tests/e2e/approval_denial_reason_test.go` | Permanent denial later blocks same/different arguments without reason; revoke-before-claim suppresses delivery vs revoked prior grant; claim/revoke races; owner retains reason | T010 / T048 |

Scenario 3.5 uses a real denial submission through the existing frontend Playwright harness,
then the production broker and in-process ExtProc. Its one canonical `It()` lives in
`tests/e2e/frontend/approval_denial_reason_test.go` and retains browser, broker, and gateway
log/span/metric assertions for successful and rejected journeys. T009 supplies test-only signal
capture/ExtProc helpers; T011 owns the full journey and reuses frontend lifecycle cleanup. Never
add browser or screenshot work to a backend suite, duplicate the scenario, or replace it with an
API-only test. Scenarios 1.2 and 3.6 retain their console-safety assertions too.

## Requirement traceability

| Requirement / binding constraint | Primary tasks |
|---|---|
| FR-001 optional input, both surfaces/scopes | T007, T011, T023, T036–T039 |
| FR-002 500-code-point validation | T015, T021, T024, T030, T037, T039 |
| FR-003 absence and exact retention | T015, T024, T030, T036–T037, T049–T052 |
| FR-004 owner-only authorship | T016, T020–T021, T029–T032, T040, T042, T044 |
| FR-005 atomic decision/reason | T016–T020, T024–T029, T040, T044 |
| FR-006 immutable reason including none | T016–T021, T024, T027–T030, T040, T043–T044, T049 |
| FR-007 actual agent delivery | T010, T017–T022, T025, T027–T035, T039, T041, T045 |
| FR-008 owner rereads, durable state, at-most-once/window | T018–T022, T027–T035, T040–T041, T044–T048, T066 |
| FR-009 unchanged scopes/approval/retention | T016, T022, T032–T034, T049–T052, T058–T060 |
| FR-010 no reason on later permanent blocks | T010, T022, T034, T041, T045, T048 |
| FR-011 cancel/correct/resubmit | T011, T023, T036–T039 |
| FR-012 read-only reason, absent heading omitted | T011, T020–T021, T029–T030, T036–T038, T043, T046, T050 |
| API-001–API-004 | T001, T005, T021–T022, T030–T035, T053, T056 |
| DB-001–DB-002 | T006, T017–T019, T025–T028, T044, T059, T066 |
| SR-001–SR-004 | T013, T015, T020–T022, T029–T035, T040–T048, T060–T061 |
| Frontend/design-system requirements | T007, T011, T023, T036–T038, T043, T046, T065, T067 |
| Principles II, V, VII, XII, XV and contribution gate | T001–T014, T032, T047, T053–T068 |

## Dependencies & Execution Order

### Phase Dependencies

```text
T001 approval + T002 context
  → Phase 2 (T003–T014): design, canonical acceptance tests, semantic-red checks
  → US1 tests T015–T023 → implementation T024–T038 → T039
  → US3 tests T040–T043 → protection/compliance T044–T047 → T048
  → US2 tests T049–T050 → compatibility T051 → T052
  → Phase N T053–T068 (all stories, documentation, complete available verification)
```

T018 precedes migration T026; T019 precedes PostgreSQL implementation T028. T026 precedes
PostgreSQL test execution against the new schema, but missing containers defer those runs, not
implementation. T025's signature changes and mechanical test-double updates are one compiling
batch; T024's aggregate call-site updates belong in that same batch if needed. Domain and adapter
semantic tests must already be authored. Do not keep a knowingly broken intermediate build.

T032 depends on T025–T031; T033–T035 require the approved delivery contract and T032 for real
journeys. T036–T038 depend on API types/contract, not ExtProc implementation, and may be authored
after their own red tests while gateway work proceeds. T039 requires the whole US1 vertical slice.
US3 and US2 both build on that slice; they are independently exercisable journeys, not separate
releases or new bounded contexts. Shared files make serial US3 → US2 the default. Do not release
US1 alone or omit either later story.

### Within Each User Story

Tests before new behavior; models/ports before storage and services; services before handlers;
builder/routing before integrated journeys; frontend tests before forms; green/refactor before
marking a task complete. Already-covered security or compatibility assertions may stay green;
do not manufacture failures or weaken tests to create a red result. Do not append per-run results
to this list. Commit logical groups with the required co-author trailer.

### Parallel Opportunities

- After T001–T002, T003/T004/T005/T006/T007 touch separate concerns; coordinate ARCHITECTURE.md edits.
- T015–T023 are independent test files, with migration tests written before SQL and adapter tests
  written before repository changes. They may share fixture design, not mutable fixture state.
- After T024–T026, T027 (memory) and T028 (PostgreSQL) can run independently.
- After API types and T023, UI implementation can proceed alongside ExtProc implementation;
  the integration checkpoint waits for both.
- T040–T043 occupy different safety-test files and may run concurrently after US1.
- T049 and T050 cover separate Go/TypeScript files; implementation T051 remains serialized.
- Parallel test execution must isolate global telemetry providers, databases, and screenshot names.

## Parallel Example: User Story 1

```text
T015: value-object tests — internal/domain/storage/denial_reason_test.go
T017: memory atomicity tests — internal/adapters/storage/memory/tool_approval_repository_test.go
T018: migration tests — tests/integration/migrations/approval_denial_reason_test.go
T023: browser component/API tests — web/src/components/approvals/DenialReasonForm.test.tsx
```

After their red tests and prerequisites, T027 and T028 implement independent adapters. UI
T036–T038 can run alongside gateway T033–T035 with agreed contracts; avoid shared-file edits.

## Parallel Example: User Story 3

```text
T040: broker confidentiality tests — internal/domain/approval/service_security_test.go
T041: gateway confidentiality tests — internal/extproc/approval/denial_reason_security_test.go
T042: production auth journeys — tests/e2e/approval_api_test.go
T043: literal rendering tests — web/src/components/approvals/ApprovalConfirmation.test.tsx
```

## Parallel Example: User Story 2

```text
T049: legacy-input tests — internal/adapters/http/handlers/approval/deny_handler_test.go
T050: no-added-approval-step tests — web/src/components/approvals/DenialReasonForm.test.tsx
```

## Implementation Strategy

1. Establish actual approval, publish API design, review domain/UI/observability/schema design,
   and write the full semantic acceptance targets before business code.
2. Build the US1 vertical slice incrementally through tests → domain → both repositories → broker
   delivery → ExtProc MCP response → both browser surfaces. Validate the candidate MVP journey.
3. Complete US3 confidentiality, immutability, authentication, restart, race, and later-invocation
   verification before considering the feature safe for delivery.
4. Complete US2 no-body/blank/historical/approval regressions without adding scope.
5. Reconcile documentation and run every available verification recipe. Defer container suites
   explicitly to pushed-commit container CI. A deferred container run is not validated evidence;
   unfinished non-container work or missing sign-off remains unchecked and is reported blocked.

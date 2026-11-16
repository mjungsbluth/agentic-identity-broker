# Quickstart: Validate tool approval denial reasons

This guide describes how to check the feature end to end. Contracts:
[enduser-approvals-openapi.yaml](./contracts/enduser-approvals-openapi.yaml) and
[extproc-denied-outcome.md](./contracts/extproc-denied-outcome.md). Domain rules:
[data-model.md](./data-model.md).

## Prerequisites

- `just install-tools` completed (Go toolchain, Node, Playwright browsers).
- Docker or Podman only for the PostgreSQL journeys labelled `docker`.

## Automated validation

| Goal | Command | Expected outcome |
|---|---|---|
| Static checks | `just check` | No findings |
| Go unit and package tests (domain, handlers, memory adapter, ExtProc gate/client/server) | `just test` | Pass |
| SPA component and API-client tests | `just web-test` | Pass |
| ExtProc unit tests with race detection | `just extproc-test` | Pass |
| Backend acceptance journeys (broker + in-process ExtProc) | `just test-e2e-backend` | Every non-Docker scenario `It` in `tests/e2e/approval_denial_reason_test.go` passes, alongside the existing approval journeys |
| Browser journeys and screenshots | `E2E_CAPTURE_SCREENSHOTS=true GINKGO_FRONTEND_PROCS=1 just test-e2e-frontend` | `tests/e2e/frontend/approval_denial_reason_test.go` passes; captures `approval_denial_*.png` appear in `tests/e2e/frontend/coverage/screenshots/`; the existing screenshot workflow publishes them to `tests/e2e/screenshots/` |
| PostgreSQL restart journey (Scenario 3.4) | `just test-e2e-docker` | The `docker`-labelled restart `It` passes |
| ExtProc suite regressions (gate order, existing elicitation) | `just test-e2e-extproc` | Existing approval-sync scenarios pass with the new gate step |
| Migration `048` up/down | `just test-integration-infra` | `TestMigration048*` passes in `tests/integration/migrations/` |
| PostgreSQL adapter claim and deny atomicity | `just test-integration-infra` | Tool-approval repository integration tests pass |

In the agent sandbox, run the non-container recipes individually rather than `just verify` or
`just test-e2e`, which include container suites. Defer `just test-e2e-docker` and
`just test-integration-infra` to the container tests on the pushed commit. For the ExtProc E2E
recipe, select its non-container scenarios with
`GINKGO_SKIP='Agentgateway|agentgateway' just test-e2e-extproc`; the current container groups
are named Agentgateway Integration, Agentgateway Re-authentication Challenge, Metadata Input
via Agentgateway, Agentgateway Telemetry Integration, and OPA Authorization via Agentgateway.
The full verification gate also includes `just test-integration`, `just cdk-test`,
`just mock-sample-agent-test`, and `just mock-upstream-oauth2-test`. In the implementation
workflow, replace `just security` with the prepared `bash "$SECURITY_SCAN_SCRIPT" .` scanner.
Report each run's results only in the session result, not in this guide or other feature files.

## Manual walk-through (development broker with in-memory storage)

1. Start the broker and ExtProc with `examples/config/extproc-tool-approvals.yaml` and an OPA policy
   that returns `approval_required` for `deploy_release`.
2. Send an MCP `tools/call` for `deploy_release` with
   `{"project":"production","version":"v2.3.0"}` through the gateway. Expect a URL elicitation
   (`-32042`) whose URL is `/approvals/<id>`.
3. Open the URL as the owner, choose **Deny**, enter `Use the staging project instead.`, and
   confirm. Expect a read-only confirmation that shows the reason. Reload: same reason, no edit
   control.
4. Repeat the identical `tools/call`. Expect HTTP 200 with `result.isError: true`, the reason in
   the text content, and `_meta["io.agentic-identity-broker/tool-approval"].denial_reason` equal to
   the submitted text.
5. Repeat the identical `tools/call` again. Expect a new URL elicitation (temporary denial; the
   outcome was already delivered).
6. Repeat steps 2–4 with **Deny permanently**. Then call the tool again with the original and with
   different arguments. Expect HTTP 403 `tool permanently denied` without the reason both times.
7. Try `POST /api/approvals/{id}/deny` with a 501-character `denial_reason`. Expect `400
   invalid_denial_reason` and the approval still pending.
8. Search the broker and ExtProc log output for the submitted reasons. Expect no match.

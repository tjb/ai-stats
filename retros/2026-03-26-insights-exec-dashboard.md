## Retrospective: Insights Executive Dashboard
**Date**: 2026-03-26
**Participants**: pm, engineering-manager, backend-engineer, frontend-engineer (x2), qa-engineer
**Issues shipped**: 7 (cadia-pdd and 6 others)

### What Shipped

Executive dashboard on the Insights page: KPI summary cards, trend sparklines, and top-methods table scoped to exec personas. Full stack: migration, Go handlers, Angular components.

### What Worked Well

- Parallel execution landed cleanly. Backend and both frontend tasks ran simultaneously with zero conflicts because they touched different files and sections. Zero merge collisions across 7 issues.
- PM scope discipline was sharp — cut to exec persona requirements quickly and held the line. Engineers never had to ask "should we build this?"
- EM briefs were precise enough that engineers needed zero clarification mid-task.
- QA caught a real data-loss bug (cadia-pdd): a SQL `NULLIF` expression produced NULL, which silently dropped rows when scanned into a non-pointer `float64` in Go. Fixed with `COALESCE` in the SQL layer. This is exactly the kind of bug that passes all unit tests and only surfaces under real data conditions.
- All engineers ran `go build ./...` / `tsc --noEmit` before closing issues. Compilation as a gate held.

### What Didn't Work

- Parallel editing of the same file is fragile. Two frontend engineers (cadia-353 and cadia-8bz) edited the same Angular component simultaneously. They got lucky — their changes were in non-overlapping sections. Without explicit section boundaries in the brief, the next parallel edit of the same file is a coin flip.
- The `NULLIF`-produces-NULL-scanned-into-non-pointer-float64 bug is a systematic Go+pgx pitfall, not a one-off. The pattern will recur anywhere SQL uses `NULLIF` for division-by-zero protection. It was not in the backend engineer's known patterns before this sprint.
- Handler test coverage is thin. `go test ./...` runs 7 tests, all in the github package. The new Insights handlers have no integration tests. This is a coverage debt that will slow down future refactors.

### Root Causes

- Parallel same-file edits: briefs did not enumerate section ownership, so engineers had no explicit contract about what was theirs to touch.
- NULLIF/COALESCE bug: no documented pattern for "SQL expressions that produce nullable output require COALESCE before scanning into non-pointer Go types." Engineers have to rediscover this independently.
- Missing handler tests: no explicit acceptance criterion requiring handler-level integration tests for new endpoints. Unit tests passed so the gate passed.

### Action Items

| Agent | Change | Rationale |
|-------|--------|-----------|
| backend-engineer | Add "NULLIF produces NULL — always COALESCE" to pgx patterns | Prevents silent row drops on future dashboard/analytics queries |
| engineering-manager | When two engineers will edit the same file in parallel, list non-overlapping sections in each brief and warn them to read the file immediately before editing | Avoids same-file merge collisions that currently depend on luck |

### Patterns to Preserve

- PM cuts scope before EM writes briefs. Do not let engineers start until scope is locked.
- Parallel task batches work well when file ownership is disjoint. Make ownership explicit in the brief, not implicit.
- QA as a hard gate before close caught a production data-loss bug. The gate is non-negotiable.
- Compilation check (`go build ./...`, `tsc --noEmit`) as a pre-close requirement for engineers. Keep this in all briefs.

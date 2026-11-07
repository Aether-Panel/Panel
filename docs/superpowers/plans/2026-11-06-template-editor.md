# Editor visual de plantillas — Implementation Plan

> For agentic workers: executed with superpowers:subagent-driven-development (Opus, high effort). Each task is implemented, then reviewed before the next.

**Goal:** a full-page template editor where anyone builds a template with forms (variables, install steps, run, environment…) while JSON stays a first-class, always-in-sync view.

**Spec:** docs/superpowers/specs/2026-11-06-template-editor-design.md (Spanish, binding). The user delegated taste: "usa lo que se adapte a nuestra interfaz y lo que quede bien y profesional".

## Global constraints
- Lossless round trip: the editor always works on the WHOLE template object; fields it doesn't render survive untouched. Opening and saving without edits yields a deep-equal object (key order aside) for every community template.
- One source of truth per concept: a template zod schema + one zod schema per install operation, in `client/frontend/src/features/templates/schema/`; forms, live validation and Monaco's JSON Schema (`z.toJSONSchema`) all derive from them.
- Operation keys listed in a JSON manifest (e.g. `schema/operations.json`) that a Go test reads to assert every operation registered in the backend has a frontend entry.
- Components: existing shadcn/Radix UI in `src/components/ui`; add `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities` (sortable lists, keyboard reorder), `cmdk` + `@radix-ui/react-popover` (shadcn Command/Combobox pattern). Verify React 19 peer compatibility; pin exact versions (≥ 2 weeks old). Monaco already present (lazy-load it).
- Visual language: match the panel (settings tabs, server pages, cards, badges, muted text, `text-primary-bright` accents, phosphor icons via `@/components/icons`). Professional, calm, dense-but-readable.
- Responsive: 360 / 768 / 1280 / 1920; mobile = section selector on top, no split view, ↑ ↓ reorder buttons alongside drag; no horizontal scroll.
- Accessibility: labels on every control, keyboard reorder, errors announced (aria-live / aria-describedby).
- i18n: es/en keys, same structure, valid JSON, NO trailing newline at EOF.
- Docker images: warn when an image isn't `ghcr.io/aether-panel/*` or an official image (project rule).
- Verification: `bash /private/tmp/claude-501/-Users-rubenmaso-Developer-paneltambres/2c190f69-223e-4c8c-a860-02747c59afbe/scratchpad/verify.sh fe` (lint, typecheck, build) and `... go` for Go changes; unit tests for pure logic via vitest (Task 1 adds it, and a CI step).

## Task 1 — Schemas, operation registry, test runner
- Add vitest (dev dep, `yarn test` script in client/frontend, and a step in the frontend CI job in .github/workflows that runs it).
- `schema/template.ts`: zod schema for a template (type, display, icon, data variables, groups, install/uninstall steps, run incl. command (string | conditional list), stop, stopCode, pre/post, environmentVars, workingDirectory, stdin, autostart/autorecover/autorestart, expectedExitCode, environment, supportedEnvironments, requirements, stats, query, keepAlive) using `.passthrough()`/loose objects so unknown keys survive. Read pkg/skypanel/*.go (Server, Execution, Variable, ConditionalMetadataType, Requirements…) and the community templates (fetch https://templates.aetherpanel.es/templates.json + each `url`) to get shapes right.
- `schema/operations.ts` + `schema/operations.json`: one entry per operation in internal/operations/* (and the server operations start/stop/restart/backup if they can appear in templates): key, category, label/help i18n keys, icon, zod schema of its args with per-field metadata for widgets (text, multiline, list of strings, key-value, number, boolean, select, path). Derive args from each operation's Go factory.
- `schema/json-schema.ts`: `z.toJSONSchema` of the template schema for Monaco.
- Go test (e.g. internal/operations/registry_frontend_test.go or wherever factories are listed) reading the manifest and failing if a backend operation key is missing.
- Vitest: every community template parses with the template schema without loss (download fixtures into a test fixtures folder, committed), JSON Schema generation succeeds.
Commit: `feat(templates): schemas for templates and every install operation`.

## Task 2 — Editor page shell and state
- New page `src/pages/templates/editor.astro` (+ client component) at `/templates/editor/` (`?name=` edits a local template; `?from=<repoId>:<name>` duplicates; `?preset=<id>` starts from a base). Permission as today (templates.edit/view).
- State: a single template object + JSON text; form edits patch the object immutably (deep set by path) and re-serialize; JSON edits parse → replace object when valid, keep last valid otherwise and flag invalid. Undo of a field change is not required.
- Layout: header (back to list, title, unsaved-changes dot, Form | JSON | Split switcher, Preview, Download, Save), left section nav with per-section error/warning counts, content area. Mobile: section select at top, Form | JSON only.
- Monaco JSON view lazy-loaded with the generated JSON Schema (diagnostics + completion), theme matching the panel.
- Save via existing `saveTemplate` (PUT /api/templates/0/:name); server errors shown; leaving with unsaved changes asks for confirmation.
- Templates list page: "Nueva plantilla" opens a small chooser (blank / base / duplicate / import .json); local cards get "Editar" and "Duplicar", community cards "Duplicar"; remove the old create/edit dialog.
- Sections render placeholders that later tasks fill; the Información section (name with validation `^[a-z0-9-]+$` or whatever the backend accepts — check, display, type/category, icon picker) is done here.
Commit: `feat(templates): a full-page editor with form and JSON views`.

## Task 3 — Variables section
Sortable list (dnd-kit) of variable cards: key (validated, unique), display, description, type select (string/integer/boolean/option), default value control by type, required/userEdit/internal switches, options editor (value + display, sortable) for `option`. Usage analysis across run commands, install steps, environmentVars, config files: warn on `${key}` used but not defined and defined but unused. A reusable "Insertar variable" popover (cmdk) for any command/text field. Lossless for extra variable fields.
Commit: `feat(templates): edit variables visually`.

## Task 4 — Install steps (and uninstall, pre/post)
Reusable `StepList` (sortable) with "Añadir paso" → cmdk command palette grouped by category with icons and search; each step card shows its operation icon/label, a one-line summary, collapse/expand, duplicate, delete, ↑ ↓; condition editor (`if`: none / only host / only docker / custom expression with the variables available); per-operation form generated from the registry metadata; unknown type → JSON editor for that step only. Used for install, uninstall, run.pre, run.post.
Commit: `feat(templates): build install steps without JSON`.

## Task 5 — Run, Environment, Requirements, Advanced
Run: command (single string or conditional list editor), stop command vs stop code, working directory, environment variables key-value editor, stdin type/options, autostart/autorecover/autorestart, expected exit code. Environment: default environment type (host/docker) with image field + third-party image warning, supported environments list. Requirements: OS, arch, binaries. Advanced: groups, stats, query, keepAlive — forms where simple, else per-section JSON blocks.
Commit: `feat(templates): run, environment and requirements sections`.

## Task 6 — Helpers: bases, duplicate, preview, validation summary
Base templates (Minecraft Java with Paper/vanilla download, Steam game via steamgamedl/command with env conditions, Node.js app, Python app, generic Docker) — valid against the schema and the backend, official/ghcr.io/aether-panel images only. Duplicate chooser with search over local + community. Preview sheet rendering the "Crear servidor" variables form by reusing the stepper's variable inputs (export `TemplateVariableInput` & friends from create-server-stepper.tsx with minimal changes). Validation summary panel (all errors/warnings, click to jump to field).
Commit: `feat(templates): start from a base or a copy, and preview the create form`.

## Task 7 — Docs and polish
Docs es/en for the editor (features/templates.md or the existing page; frontend component docs). i18n review. Responsive/a11y pass at 360/768/1280.
Commit: `docs: the visual template editor`.

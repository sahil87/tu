# Tasks: Relicense MIT & Migrate to sahil87

**Change**: 260401-lomt-relicense-mit-migrate-sahil87
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: License & Package

- [x] T001 [P] Replace `LICENSE` contents with MIT license text (copyright: Sahil Ahuja, 2026)
- [x] T002 [P] Change `license` field in `package.json` from `"PolyForm-Internal-Use-1.0.0"` to `"MIT"`

## Phase 2: Org References

- [x] T003 [P] Update `src/node/core/cli.ts` — change `brew install wvrdz/tap/tu` to `brew install sahil87/tap/tu`
- [x] T004 [P] Rewrite `README.md` install section — replace wvrdz SSH-gated instructions with public `sahil87/tap` instructions

## Phase 3: Project Documentation

- [x] T005 [P] Update `fab/project/context.md` — distribution line to `sahil87/tap`, license line to `MIT`
- [x] T006 [P] Update `docs/memory/build/toolchain.md` — all wvrdz refs to sahil87, PolyForm to MIT, remove SSH access note

## Phase 4: Verification

- [x] T007 Run `npm test` to confirm no tests break (weaver config tests should still pass since those files are unchanged)

---

## Execution Order

- T001–T006 are all independent and parallelizable
- T007 depends on all prior tasks completing

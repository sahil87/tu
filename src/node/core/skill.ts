// src/node/core/skill.ts — resolves the agent usage bundle (`tu skill`).
//
// The bundle content is authored once in `docs/site/skill.md` (the canonical
// source, also rendered on shll.ai). It is resolved here two ways, mirroring the
// __PKG_*__ typeof-guard pattern in cli.ts:
//
//   Built bundle: esbuild `--define:__SKILL_MD__` supplies the string at build
//     time (see scripts/build.sh). The shipped binary performs NO filesystem
//     read for the bundle — Constitution III (single-bundle, no runtime file
//     reads). Byte-identity with docs/site/skill.md is by construction because
//     the define reads that file directly at build time.
//
//   Dev/tsx (tests, `npx tsx`): the define is absent, so read the canonical file
//     from the repo. This path never runs in the shipped bundle.

import { readFileSync } from "node:fs";

// Injected at build time by esbuild --define; absent under tsx.
declare const __SKILL_MD__: string | undefined;

export const SKILL_MD: string =
  typeof __SKILL_MD__ !== "undefined"
    ? __SKILL_MD__
    : readFileSync(new URL("../../../docs/site/skill.md", import.meta.url), "utf8");

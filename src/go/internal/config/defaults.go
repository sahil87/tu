package config

import _ "embed"

// DefaultConf is the shipped tu.default.conf, embedded so the binary carries
// no runtime file dependency (the TS looks the file up beside the bundle,
// then walks up to the project root — neither lookup is reproduced). The
// sibling tu.default.conf is a byte-identical copy of the repo-root file that
// scripts/build.sh copies beside tu.mjs; defaults_test.go guards the copy
// against drift.
//
//go:embed tu.default.conf
var DefaultConf []byte

// DefaultConfName is the string the newer-version warning prints when the
// out-of-range version came from the defaults layer (unreachable with the
// shipped file, which says version = 2; kept for shape parity with the TS,
// which names DEFAULT_CONFIG_PATH there).
const DefaultConfName = "tu.default.conf"

package harness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// VersionPlaceholder is the literal that replaces the binary's own version
// string (and its v-stripped form) in every compared byte channel, so the
// --version, -v, and help-dump goldens hold across releases while every other
// byte stays exact (NormalizeVersion).
const VersionPlaceholder = "$VERSION"

// DefaultGoldenDir is the repo-root-relative default of tudiff's --golden
// flag: the committed golden corpus.
const DefaultGoldenDir = "harness/golden"

// GoldenManifest is harness/golden/manifest.json (schema 1): the golden
// corpus's provenance — what produced it, when, under which pinned clock, and
// the hash of the matrix it was captured against. Field order is the
// serialized key order.
type GoldenManifest struct {
	Schema        int      `json:"schema"`
	Oracle        string   `json:"oracle"`         // e.g. "node <bundle>" at the one-time capture, "bin/tu" after
	OracleVersion string   `json:"oracle_version"` // probed `<oracle> --version`
	NodeVersion   string   `json:"node_version"`   // empty when captured from Go
	CapturedAt    string   `json:"captured_at"`    // RFC 3339 UTC
	Now           string   `json:"now"`            // pinned zone-less local timestamp (TUDIFF_NOW)
	Script        string   `json:"script"`         // script(1) flavour: util-linux or bsd
	Platform      string   `json:"platform"`       // GOOS/GOARCH
	Fixtures      []string `json:"fixtures"`       // always ["_placeholder"] for the committed corpus
	MatrixSHA256  string   `json:"matrix_sha256"`  // hex sha256 of harness/matrix.json at capture time
	Cases         int      `json:"cases"`
	LiveSteps     int      `json:"live_steps"`
}

// LoadGoldenManifest reads <dir>/manifest.json, rejecting unknown fields,
// trailing content, and any schema other than SchemaVersion.
func LoadGoldenManifest(dir string) (*GoldenManifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m GoldenManifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	switch err := dec.Decode(&extra); err {
	case io.EOF:
	case nil:
		return nil, fmt.Errorf("trailing JSON value after manifest document")
	default:
		return nil, fmt.Errorf("trailing content after manifest document: %w", err)
	}
	if m.Schema != SchemaVersion {
		return nil, fmt.Errorf("schema must be %d, got %d", SchemaVersion, m.Schema)
	}
	return &m, nil
}

// WriteGoldenManifest writes <dir>/manifest.json with 2-space indentation and
// a trailing newline, creating the directory as needed.
func WriteGoldenManifest(dir string, m *GoldenManifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(raw, '\n'), 0o644)
}

// MatrixSHA256 is the hex sha256 of the matrix file's bytes — the drift guard
// between the committed goldens and harness/matrix.json.
func MatrixSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// NormalizeVersion replaces every occurrence of version (the probed
// `<bin> --version` value, e.g. "v0.12.2") and of its v-stripped form (the
// bare "0.12.2" help-dump prints) in b with VersionPlaceholder, longest form
// first so the bare form never matches inside the v-form. An empty version is
// a no-op. Applied to both sides of a comparison, it keeps the version-output
// goldens stable across releases.
func NormalizeVersion(b []byte, version string) []byte {
	if version == "" {
		return b
	}
	forms := []string{version}
	if bare := strings.TrimPrefix(version, "v"); bare != version {
		forms = append(forms, bare)
	}
	sort.Slice(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	for _, form := range forms {
		b = bytes.ReplaceAll(b, []byte(form), []byte(VersionPlaceholder))
	}
	return b
}

// ProbeVersion runs `bin args...` (args are `--version` at every call site)
// and returns the last whitespace-separated field of stdout's first line —
// "tu version v0.12.2\n" yields "v0.12.2".
func ProbeVersion(bin string, args ...string) (string, error) {
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return "", fmt.Errorf("tudiff: %s %s: %w", bin, strings.Join(args, " "), err)
	}
	line, _, _ := strings.Cut(string(out), "\n")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", fmt.Errorf("tudiff: %s %s: empty first line", bin, strings.Join(args, " "))
	}
	return fields[len(fields)-1], nil
}

// TreeFile pins one file of the written metrics repo: its content hash and
// length. The golden corpus stores hashes rather than bytes so ~250 copies of
// the seed tree never enter git.
type TreeFile struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Tree is the tree.json shape: every file under <home>/.tu/metrics_repo/
// (slash-separated relpath → hash+size; nothing under .tu/cache/) plus the
// presence — never the content, a wall-clock timestamp — of
// <home>/.tu/.last-sync. An absent metrics repo is {"files": {},
// "last_sync": false}. These are the same three things CompareTrees compares.
type Tree struct {
	Files    map[string]TreeFile `json:"files"`
	LastSync bool                `json:"last_sync"`
}

// TreeSnapshot hashes <home>/.tu/metrics_repo/ into a Tree and records the
// presence of <home>/.tu/.last-sync. A missing metrics repo yields the empty
// tree.
func TreeSnapshot(home string) (Tree, error) {
	return treeSnapshot(filepath.Join(home, ".tu", "metrics_repo"),
		filepath.Join(home, ".tu", ".last-sync"), nil)
}

// TreeSnapshotLive is TreeSnapshot over a live-harness clone of the real bare
// remote: the .git directory is excluded (a clone's .git legitimately differs
// across runs — the remote path — while the tracked tree must not).
func TreeSnapshotLive(home string) (Tree, error) {
	return treeSnapshot(filepath.Join(home, ".tu", "metrics_repo"),
		filepath.Join(home, ".tu", ".last-sync"), SkipGit)
}

// TreeSnapshotRepo hashes an arbitrary repository working tree (the live
// repair repos), .git excluded; LastSync stays false.
func TreeSnapshotRepo(repo string) (Tree, error) {
	return treeSnapshot(repo, "", SkipGit)
}

// SkipGit is the tree-walk exclusion for a real clone: the .git subtree.
func SkipGit(rel string) bool {
	return rel == ".git" || strings.HasPrefix(rel, ".git/")
}

// treeSnapshot hashes every file under repo (slash relpath → hash+size; skip
// excludes paths) and records the presence of the lastSync marker file (empty
// = no marker concept).
func treeSnapshot(repo, lastSync string, skip func(rel string) bool) (Tree, error) {
	t := Tree{Files: map[string]TreeFile{}}
	paths, exists, err := treeFilesSkip(repo, skip)
	if err != nil {
		return t, err
	}
	if !exists {
		return t, nil
	}
	for _, p := range paths {
		raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(p)))
		if err != nil {
			return t, err
		}
		sum := sha256.Sum256(raw)
		t.Files[p] = TreeFile{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(raw))}
	}
	if lastSync != "" {
		t.LastSync = filePresent(lastSync)
	}
	return t, nil
}

// WriteTree writes <dir>/tree.json with 2-space indentation and a trailing
// newline, creating the directory as needed. A nil Files map serializes as
// {}, never null.
func WriteTree(dir string, t Tree) error {
	if t.Files == nil {
		t.Files = map[string]TreeFile{}
	}
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tree.json"), append(raw, '\n'), 0o644)
}

// LoadTree reads <dir>/tree.json.
func LoadTree(dir string) (Tree, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "tree.json"))
	if err != nil {
		return Tree{}, err
	}
	var t Tree
	if err := json.Unmarshal(raw, &t); err != nil {
		return Tree{}, err
	}
	if t.Files == nil {
		t.Files = map[string]TreeFile{}
	}
	return t, nil
}

// CompareTree compares a golden tree against the actual tree left under home
// by the sync writer. A nil *TreeDiff means the trees agree.
func CompareTree(golden Tree, home string) (*TreeDiff, error) {
	actual, err := TreeSnapshot(home)
	if err != nil {
		return nil, err
	}
	return CompareTreeSnapshot(golden, actual), nil
}

// CompareLiveTree is CompareTree over a live-harness clone: the .git subtree
// is excluded from the actual tree exactly as at capture time.
func CompareLiveTree(golden Tree, home string) (*TreeDiff, error) {
	actual, err := TreeSnapshotLive(home)
	if err != nil {
		return nil, err
	}
	return CompareTreeSnapshot(golden, actual), nil
}

// CompareTreeSnapshot compares a golden tree against an actual one, returning
// the first differing path (sorted order) as a TreeDiff: "present"/"absent"
// excerpts for a path-set or .last-sync difference (the golden side sits in
// the NodeExcerpt/oracle position), a hash+size descriptor of each side for a
// content difference. A nil *TreeDiff means the trees agree.
func CompareTreeSnapshot(golden, actual Tree) *TreeDiff {
	merged := make([]string, 0, len(golden.Files)+len(actual.Files))
	for p := range golden.Files {
		merged = append(merged, p)
	}
	for p := range actual.Files {
		if _, ok := golden.Files[p]; !ok {
			merged = append(merged, p)
		}
	}
	sort.Strings(merged)
	for _, p := range merged {
		g, gOK := golden.Files[p]
		a, aOK := actual.Files[p]
		if gOK != aOK {
			return presenceDiff(p, gOK)
		}
		if g != a {
			describe := func(f TreeFile) string {
				return fmt.Sprintf("%s: sha256 %s, %d bytes", p, f.SHA256, f.Size)
			}
			return &TreeDiff{Path: p, NodeExcerpt: describe(g), GoExcerpt: describe(a)}
		}
	}
	if golden.LastSync != actual.LastSync {
		return presenceDiff(".last-sync", golden.LastSync)
	}
	return nil
}

// GoldenCaseDir is the directory holding case ID's golden under the corpus
// root: <root>/run/<case ID as nested dirs>.
func GoldenCaseDir(root, caseID string) string {
	return filepath.Join(root, "run", filepath.FromSlash(caseID))
}

// GoldenLiveDir is the directory holding one live step's golden under the
// corpus root: <root>/live/<step>.
func GoldenLiveDir(root, step string) string {
	return filepath.Join(root, "live", step)
}

// WriteGoldenCase writes one case's golden under dir: stdout, stderr, and
// exit for a pipe capture or tty and exit for a tty capture (TTY != nil is
// the discriminator), plus tree.json. Byte channels are home-normalized
// (NormalizeHome with the capture's staged Home) so the files hold exactly
// what Compare compares; the caller version-normalizes them first
// (NormalizeVersion). exit holds the decimal exit code.
func WriteGoldenCase(dir string, sc SideCapture, tree Tree) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	write := func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
	if sc.TTY != nil {
		if err := write("tty", NormalizeHome(sc.TTY, sc.Home)); err != nil {
			return err
		}
	} else {
		if err := write("stdout", NormalizeHome(sc.Stdout, sc.Home)); err != nil {
			return err
		}
		if err := write("stderr", NormalizeHome(sc.Stderr, sc.Home)); err != nil {
			return err
		}
	}
	if err := write("exit", []byte(strconv.Itoa(sc.Exit)+"\n")); err != nil {
		return err
	}
	return WriteTree(dir, tree)
}

// LoadGoldenCase reads one case's golden from dir into the oracle-position
// SideCapture: stdout/stderr/exit for io == IOPipe, tty/exit for io ==
// IOTTY, plus tree.json. The stored bytes are already home- and
// version-normalized, so the returned capture carries no Home.
func LoadGoldenCase(dir, io string) (SideCapture, Tree, error) {
	var sc SideCapture
	read := func(name string) ([]byte, error) {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("tudiff: loading golden %s: %w", dir, err)
		}
		return raw, nil
	}
	if io == IOTTY {
		tty, err := read("tty")
		if err != nil {
			return sc, Tree{}, err
		}
		sc.TTY = tty
	} else {
		stdout, err := read("stdout")
		if err != nil {
			return sc, Tree{}, err
		}
		stderr, err := read("stderr")
		if err != nil {
			return sc, Tree{}, err
		}
		sc.Stdout, sc.Stderr = stdout, stderr
	}
	exitRaw, err := read("exit")
	if err != nil {
		return sc, Tree{}, err
	}
	sc.Exit, err = strconv.Atoi(strings.TrimSpace(string(exitRaw)))
	if err != nil {
		return sc, Tree{}, fmt.Errorf("tudiff: loading golden %s: exit: %w", dir, err)
	}
	tree, err := LoadTree(dir)
	if err != nil {
		return sc, Tree{}, fmt.Errorf("tudiff: loading golden %s: %w", dir, err)
	}
	return sc, tree, nil
}

// WriteGoldenExtra writes an extra verbatim text artefact (a live step's
// log.txt or status.txt) into a golden dir.
func WriteGoldenExtra(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), data, 0o644)
}

// LoadGoldenExtra reads an extra artefact from a golden dir.
func LoadGoldenExtra(dir, name string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("tudiff: loading golden %s: %w", filepath.Join(dir, name), err)
	}
	return raw, nil
}

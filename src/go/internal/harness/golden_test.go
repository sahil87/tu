package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func baseGoldenManifest() *GoldenManifest {
	return &GoldenManifest{
		Schema:        SchemaVersion,
		Oracle:        "bin/tu",
		OracleVersion: "v0.12.2",
		NodeVersion:   "",
		CapturedAt:    "2026-09-26T06:30:00Z",
		Now:           "2026-09-26T12:00:00",
		Script:        "util-linux",
		Platform:      "linux/amd64",
		Fixtures:      []string{PlaceholderAlias},
		MatrixSHA256:  "deadbeef",
		Cases:         452,
		LiveSteps:     9,
	}
}

// R2: the manifest round-trips, serialized with 2-space indentation, a
// trailing newline, and field order matching the struct.
func TestGoldenManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := baseGoldenManifest()
	if err := WriteGoldenManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.MarshalIndent(m, "", "  ")
	if string(raw) != string(append(want, '\n')) {
		t.Errorf("manifest.json =\n%s\nwant 2-space indent + trailing newline", raw)
	}
	got, err := LoadGoldenManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Errorf("loaded = %+v, want %+v", got, m)
	}
}

func TestLoadGoldenManifestErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"unknown field", `{"schema": 1, "bogus": true}`, "unknown field"},
		{"wrong schema", `{"schema": 2}`, "schema must be 1, got 2"},
		{"trailing value", `{"schema": 1} {}`, "trailing JSON value"},
		{"not json", `nope`, "invalid character"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadGoldenManifest(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		if _, err := LoadGoldenManifest(t.TempDir()); err == nil {
			t.Error("err = nil, want a read error")
		}
	})
}

// R2: the matrix hash is the hex sha256 of the file's raw bytes.
func TestMatrixSHA256(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"known bytes", "abc"},
		{"matrix-shaped", "{\"schema\": 1, \"cases\": []}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "matrix.json")
			if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := MatrixSHA256(path)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte(c.body))
			if want := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("MatrixSHA256 = %q, want %q", got, want)
			}
		})
	}
	if _, err := MatrixSHA256(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("missing file: err = nil")
	}
}

// R2: the probed version and its v-stripped form become $VERSION, longest
// form first.
func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		version string
		want    string
	}{
		{"version output", "tu version v0.12.3-2-gabcdef\n", "v0.12.3-2-gabcdef", "tu version $VERSION\n"},
		{"help-dump bare form", `{"version": "0.12.3-2-gabcdef"}`, "v0.12.3-2-gabcdef", `{"version": "$VERSION"}`},
		{"both forms", "v0.12.2 and 0.12.2\n", "v0.12.2", "$VERSION and $VERSION\n"},
		{"no v prefix", "ccusage 17.1.3\n", "17.1.3", "ccusage $VERSION\n"},
		{"no occurrence", "nothing to replace\n", "v0.12.2", "nothing to replace\n"},
		{"empty version is a no-op", "tu version v0.12.2\n", "", "tu version v0.12.2\n"},
		{"empty input", "", "v0.12.2", ""},
		{"bare form not left inside v-form", "v0.12.2\n", "v0.12.2", "$VERSION\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeVersion([]byte(c.in), c.version); string(got) != c.want {
				t.Errorf("NormalizeVersion(%q, %q) = %q, want %q", c.in, c.version, got, c.want)
			}
		})
	}
}

// R2: the version probe is the last field of the first stdout line.
func TestProbeVersion(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tu")
	body := "#!/bin/sh\nprintf 'tu version v0.12.2\\nignored second line\\n'\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ProbeVersion(bin, "--version")
	if err != nil || got != "v0.12.2" {
		t.Errorf("ProbeVersion = %q, %v, want v0.12.2", got, err)
	}

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ProbeVersion(empty, "--version"); err == nil {
		t.Error("empty output: err = nil")
	}
	if _, err := ProbeVersion(filepath.Join(dir, "absent"), "--version"); err == nil {
		t.Error("missing binary: err = nil")
	}
}

// R2: TreeSnapshot hashes every file under .tu/metrics_repo and records
// .last-sync presence; an absent repo is the empty tree.
func TestTreeSnapshot(t *testing.T) {
	home := t.TempDir()
	writeTreeFile(t, home, ".tu/metrics_repo/harness-user/2026/harness-machine/cc-2026-01-06.jsonl", "{}\n")
	writeTreeFile(t, home, ".tu/metrics_repo/docs/README.md", "seed\n")
	writeTreeFile(t, home, ".tu/.last-sync", "2026-09-17T00:00:00Z\n")
	writeTreeFile(t, home, ".tu/cache/ignored.json", "cache\n") // never snapshotted

	tree, err := TreeSnapshot(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 2 || !tree.LastSync {
		t.Fatalf("tree = %+v", tree)
	}
	f, ok := tree.Files["docs/README.md"]
	if !ok {
		t.Fatalf("tree = %+v", tree)
	}
	sum := sha256.Sum256([]byte("seed\n"))
	if f.SHA256 != hex.EncodeToString(sum[:]) || f.Size != 5 {
		t.Errorf("file = %+v", f)
	}

	bare, err := TreeSnapshot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(bare.Files) != 0 || bare.LastSync {
		t.Errorf("absent repo = %+v, want empty tree", bare)
	}
	if bare.Files == nil {
		t.Error("absent repo: Files = nil, want an empty map (serializes as {})")
	}
}

// R2: tree.json round-trips with the {"files": {…}, "last_sync": bool} shape.
func TestTreeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tree := Tree{
		Files:    map[string]TreeFile{"a/b.jsonl": {SHA256: "ab12", Size: 3}},
		LastSync: true,
	}
	if err := WriteTree(dir, tree); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "tree.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"files\": {\n    \"a/b.jsonl\": {\n      \"sha256\": \"ab12\",\n      \"size\": 3\n    }\n  },\n  \"last_sync\": true\n}\n"
	if string(raw) != want {
		t.Errorf("tree.json = %q, want %q", raw, want)
	}
	got, err := LoadTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, tree) {
		t.Errorf("loaded = %+v, want %+v", got, tree)
	}

	// The empty tree serializes as {}, never null.
	empty := filepath.Join(t.TempDir(), "case")
	if err := WriteTree(empty, Tree{}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(empty, "tree.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "{\n  \"files\": {},\n  \"last_sync\": false\n}\n" {
		t.Errorf("empty tree.json = %q", raw)
	}
}

// R2: CompareTree reports the first differing path — presence, content
// (hash+size descriptor), then .last-sync — with the golden in the
// NodeExcerpt/oracle position.
func TestCompareTree(t *testing.T) {
	goldenHome := t.TempDir()
	writeTreeFile(t, goldenHome, ".tu/metrics_repo/docs/README.md", "seed\n")
	writeTreeFile(t, goldenHome, ".tu/.last-sync", "2026-09-17T00:00:00Z\n")
	golden, err := TreeSnapshot(goldenHome)
	if err != nil {
		t.Fatal(err)
	}

	// Green: identical tree, different .last-sync content.
	same := t.TempDir()
	writeTreeFile(t, same, ".tu/metrics_repo/docs/README.md", "seed\n")
	writeTreeFile(t, same, ".tu/.last-sync", "2026-09-26T12:00:00Z\n")
	diff, err := CompareTree(golden, same)
	if err != nil || diff != nil {
		t.Errorf("green: CompareTree = %+v, %v, want nil", diff, err)
	}

	// An extra file on the Go side.
	extra := t.TempDir()
	writeTreeFile(t, extra, ".tu/metrics_repo/docs/README.md", "seed\n")
	writeTreeFile(t, extra, ".tu/metrics_repo/a/new.jsonl", "{}\n")
	writeTreeFile(t, extra, ".tu/.last-sync", "x\n")
	diff, err = CompareTree(golden, extra)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != "a/new.jsonl" ||
		diff.NodeExcerpt != "a/new.jsonl: absent" || diff.GoExcerpt != "a/new.jsonl: present" {
		t.Errorf("extra file: diff = %+v", diff)
	}

	// A content difference names the file with each side's hash+size.
	changed := t.TempDir()
	writeTreeFile(t, changed, ".tu/metrics_repo/docs/README.md", "SEED!\n")
	writeTreeFile(t, changed, ".tu/.last-sync", "x\n")
	diff, err = CompareTree(golden, changed)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != "docs/README.md" {
		t.Fatalf("content: diff = %+v", diff)
	}
	wantNode := "docs/README.md: sha256 " + golden.Files["docs/README.md"].SHA256 + ", 5 bytes"
	if diff.NodeExcerpt != wantNode {
		t.Errorf("NodeExcerpt = %q, want %q", diff.NodeExcerpt, wantNode)
	}
	if !strings.HasPrefix(diff.GoExcerpt, "docs/README.md: sha256 ") || !strings.HasSuffix(diff.GoExcerpt, ", 6 bytes") {
		t.Errorf("GoExcerpt = %q", diff.GoExcerpt)
	}

	// A .last-sync presence mismatch.
	noSync := t.TempDir()
	writeTreeFile(t, noSync, ".tu/metrics_repo/docs/README.md", "seed\n")
	diff, err = CompareTree(golden, noSync)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != ".last-sync" ||
		diff.NodeExcerpt != ".last-sync: present" || diff.GoExcerpt != ".last-sync: absent" {
		t.Errorf("last-sync: diff = %+v", diff)
	}
}

func TestGoldenCaseDir(t *testing.T) {
	got := GoldenCaseDir("/g", "version/single/default/pipe/fixed")
	want := filepath.Join("/g", "run", "version", "single", "default", "pipe", "fixed")
	if got != want {
		t.Errorf("GoldenCaseDir = %q, want %q", got, want)
	}
}

// R2: a golden case round-trips — pipe and tty channels, the decimal exit
// file, and tree.json; byte channels are stored home-normalized.
func TestGoldenCaseRoundTrip(t *testing.T) {
	tree := Tree{Files: map[string]TreeFile{}, LastSync: false}
	cases := []struct {
		name string
		io   string
		sc   SideCapture
	}{
		{"pipe", IOPipe, SideCapture{
			Stdout: []byte("home is /tmp/staged/home\n"),
			Stderr: []byte("warn\n"),
			Exit:   3,
			Home:   "/tmp/staged/home",
		}},
		{"tty", IOTTY, SideCapture{
			TTY:  []byte("table\r\n/tmp/staged/home\r\n"),
			Exit: 0,
			Home: "/tmp/staged/home",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteGoldenCase(dir, c.sc, tree); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "exit"))
			wantExit := strconv.Itoa(c.sc.Exit) + "\n"
			if err != nil || string(raw) != wantExit {
				t.Errorf("exit = %q, %v, want %q", raw, err, wantExit)
			}
			sc, loadedTree, err := LoadGoldenCase(dir, c.io)
			if err != nil {
				t.Fatal(err)
			}
			if sc.Exit != c.sc.Exit {
				t.Errorf("exit = %d, want %d", sc.Exit, c.sc.Exit)
			}
			if c.io == IOTTY {
				if string(sc.TTY) != "table\r\n$HOME\r\n" {
					t.Errorf("tty = %q", sc.TTY)
				}
			} else {
				if string(sc.Stdout) != "home is $HOME\n" || string(sc.Stderr) != "warn\n" {
					t.Errorf("stdout/stderr = %q / %q", sc.Stdout, sc.Stderr)
				}
			}
			if !reflect.DeepEqual(loadedTree, tree) {
				t.Errorf("tree = %+v, want %+v", loadedTree, tree)
			}
		})
	}
}

func TestLoadGoldenCaseErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := LoadGoldenCase(dir, IOPipe); err == nil {
		t.Error("empty dir: err = nil")
	}
	writeTreeFile(t, dir, "stdout", "out\n")
	writeTreeFile(t, dir, "stderr", "")
	writeTreeFile(t, dir, "exit", "not-a-number\n")
	writeTreeFile(t, dir, "tree.json", `{"files": {}, "last_sync": false}`)
	if _, _, err := LoadGoldenCase(dir, IOPipe); err == nil || !strings.Contains(err.Error(), "exit") {
		t.Errorf("bad exit: err = %v", err)
	}
}

// R4: the live snapshots exclude a real clone's .git subtree; the repair
// variant hashes an arbitrary repo and never records a .last-sync.
func TestTreeSnapshotLiveAndRepo(t *testing.T) {
	home := t.TempDir()
	writeTreeFile(t, home, ".tu/metrics_repo/docs/README.md", "seed\n")
	writeTreeFile(t, home, ".tu/metrics_repo/.git/config", "[remote]\n")
	writeTreeFile(t, home, ".tu/metrics_repo/.git/objects/aa/bb", "blob\n")
	writeTreeFile(t, home, ".tu/.last-sync", "2026-09-26T12:00:00Z\n")

	tree, err := TreeSnapshotLive(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || !tree.LastSync {
		t.Fatalf("tree = %+v", tree)
	}
	if _, ok := tree.Files["docs/README.md"]; !ok {
		t.Errorf("tree = %+v", tree)
	}
	for p := range tree.Files {
		if strings.HasPrefix(p, ".git") {
			t.Errorf(".git leaked into the live snapshot: %s", p)
		}
	}

	repo := t.TempDir()
	writeTreeFile(t, repo, "u/2026/m/cc-2026-01-01.jsonl", "{}\n")
	writeTreeFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")
	repoTree, err := TreeSnapshotRepo(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(repoTree.Files) != 1 || repoTree.LastSync {
		t.Errorf("repo tree = %+v", repoTree)
	}

	// CompareLiveTree compares against the gitless snapshot.
	golden, err := TreeSnapshotLive(home)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := CompareLiveTree(golden, home)
	if err != nil || diff != nil {
		t.Errorf("CompareLiveTree = %+v, %v, want nil", diff, err)
	}
	writeTreeFile(t, home, ".tu/metrics_repo/docs/README.md", "edited\n")
	diff, err = CompareLiveTree(golden, home)
	if err != nil || diff == nil || diff.Path != "docs/README.md" {
		t.Errorf("CompareLiveTree after edit = %+v, %v", diff, err)
	}
}

// CompareTreeSnapshot is the pure tree-vs-tree comparison (live's repair
// repos compare snapshots directly).
func TestCompareTreeSnapshot(t *testing.T) {
	golden := Tree{Files: map[string]TreeFile{"a.jsonl": {SHA256: "aa", Size: 1}}}
	same := Tree{Files: map[string]TreeFile{"a.jsonl": {SHA256: "aa", Size: 1}}}
	if diff := CompareTreeSnapshot(golden, same); diff != nil {
		t.Errorf("identical: diff = %+v", diff)
	}
	changed := Tree{Files: map[string]TreeFile{"a.jsonl": {SHA256: "bb", Size: 2}}}
	diff := CompareTreeSnapshot(golden, changed)
	if diff == nil || diff.Path != "a.jsonl" ||
		!strings.Contains(diff.NodeExcerpt, "aa") || !strings.Contains(diff.GoExcerpt, "bb") {
		t.Errorf("changed: diff = %+v", diff)
	}
}

func TestGoldenLiveDir(t *testing.T) {
	if got, want := GoldenLiveDir("/g", "sync"), filepath.Join("/g", "live", "sync"); got != want {
		t.Errorf("GoldenLiveDir = %q, want %q", got, want)
	}
}

// The extras (live's log.txt/status.txt) round-trip verbatim.
func TestGoldenExtraRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteGoldenExtra(dir, "log.txt", []byte("abc123 seed\n")); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGoldenExtra(dir, "log.txt")
	if err != nil || string(got) != "abc123 seed\n" {
		t.Errorf("LoadGoldenExtra = %q, %v", got, err)
	}
	if _, err := LoadGoldenExtra(dir, "status.txt"); err == nil {
		t.Error("missing extra: err = nil")
	}
}

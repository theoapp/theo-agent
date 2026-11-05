package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// runAgentEnv makes the test binary behave as the theo-agent command, so these
// tests exercise the real flow, including stdout and exit codes.
const runAgentEnv = "THEO_AGENT_TEST_RUN_MAIN"

const testToken = "test-token"

func TestMain(m *testing.M) {
	if os.Getenv(runAgentEnv) == "1" {
		Execute()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type agentResult struct {
	stdout string
	stderr string
	code   int
}

func runAgent(t *testing.T, args ...string) agentResult {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), runAgentEnv+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("unable to run agent: %s", err)
	}
	return agentResult{stdout.String(), stderr.String(), cmd.ProcessState.ExitCode()}
}

// writeAgentConfig writes a config file and returns its path and the cache dir.
// When publicKey is not empty, signature verification is enabled.
func writeAgentConfig(t *testing.T, url string, publicKey string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	if err := os.Mkdir(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("url: %s\ntoken: %s\ncachedir: %s\n", url, testToken, cacheDir)
	if publicKey != "" {
		abs, err := filepath.Abs(publicKey)
		if err != nil {
			t.Fatal(err)
		}
		content += fmt.Sprintf("verify: True\npublic_key: %s\n", abs)
	}
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return configFile, cacheDir
}

func fixtureKeys(t *testing.T) []Key {
	t.Helper()
	_, keys := loadCacheFile("../test/test.signatures.json")
	if len(keys) < 2 {
		t.Fatalf("expected signed keys in fixture, got %d", len(keys))
	}
	return keys
}

func keysServer(t *testing.T, keys []Key) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			t.Errorf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(keys)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeCache(t *testing.T, cacheDir string, user string, keys []Key) string {
	t.Helper()
	data, err := json.Marshal(keys)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cacheDir, "."+user+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgentPrintsPercentVerbatim(t *testing.T) {
	key := fixtureKeys(t)[0]
	key.SSHOptions = `environment="RATE=100%d"`
	srv := keysServer(t, []Key{key})
	configFile, _ := writeAgentConfig(t, srv.URL, "")

	res := runAgent(t, "-config-file", configFile, "alice")

	want := key.SSHOptions + " " + key.PublicKey + "\n"
	if res.code != 0 || res.stdout != want {
		t.Errorf("exit %d, stdout %q, want %q", res.code, res.stdout, want)
	}
}

func TestAgentDropsLineInjectionFromServer(t *testing.T) {
	keys := fixtureKeys(t)
	keys[0].SSHOptions = "no-pty\nssh-ed25519 AAAAATTACKER evil@example.com"
	srv := keysServer(t, keys)
	configFile, _ := writeAgentConfig(t, srv.URL, "../test/public2.pem")

	res := runAgent(t, "-config-file", configFile, "alice")

	if strings.Contains(res.stdout, "ATTACKER") {
		t.Errorf("unsigned key reached authorized_keys output:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, keys[1].PublicKey) {
		t.Errorf("valid signed key was lost:\n%s", res.stdout)
	}
}

func TestAgentDropsLineInjectionFromCache(t *testing.T) {
	keys := fixtureKeys(t)
	keys[0].SSHOptions = "no-pty\r\nssh-ed25519 AAAAATTACKER evil@example.com"
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	configFile, cacheDir := writeAgentConfig(t, srv.URL, "../test/public2.pem")
	writeCache(t, cacheDir, "alice", keys)

	res := runAgent(t, "-config-file", configFile, "alice")

	if strings.Contains(res.stdout, "ATTACKER") {
		t.Errorf("unsigned key reached authorized_keys output:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, keys[1].PublicKey) {
		t.Errorf("valid signed key was lost:\n%s", res.stdout)
	}
}

func TestAgentKeepsSignedKeyWithTrailingNewline(t *testing.T) {
	key := fixtureKeys(t)[0]
	key.PublicKey += "\n"
	srv := keysServer(t, []Key{key})
	configFile, _ := writeAgentConfig(t, srv.URL, "")

	res := runAgent(t, "-config-file", configFile, "alice")

	if res.code != 0 || !strings.Contains(res.stdout, strings.TrimSpace(key.PublicKey)) {
		t.Errorf("exit %d, stdout %q, stderr %q", res.code, res.stdout, res.stderr)
	}
}

func TestAgentMalformedKeyDoesNotDenyOthers(t *testing.T) {
	good := fixtureKeys(t)[0]
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(good.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	srv := keysServer(t, []Key{{PublicKey: "not a key", Account: "bad@example.com"}, good})
	configFile, _ := writeAgentConfig(t, srv.URL, "")

	res := runAgent(t, "-config-file", configFile, "-fingerprint", ssh.FingerprintSHA256(pk), "alice")

	if res.code != 0 || res.stdout != good.PublicKey+"\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", res.code, res.stdout, res.stderr)
	}
}

func TestAgentOversizedBodyFallsBackToCache(t *testing.T) {
	keys := fixtureKeys(t)
	fromServer, _ := json.Marshal(keys[1])
	body := append([]byte("["), fromServer...)
	body = append(body, bytes.Repeat([]byte(" "), 11<<20)...)
	body = append(body, ']')
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	configFile, cacheDir := writeAgentConfig(t, srv.URL, "")
	writeCache(t, cacheDir, "alice", keys[:1])

	res := runAgent(t, "-config-file", configFile, "alice")

	if res.code != 0 || res.stdout != keys[0].PublicKey+"\n" {
		t.Errorf("expected the cached key, exit %d, stdout %q", res.code, res.stdout)
	}
}

func TestAgentFallsBackToCache(t *testing.T) {
	keys := fixtureKeys(t)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	stopped := httptest.NewServer(http.NotFoundHandler())
	stoppedURL := stopped.URL
	stopped.Close()

	for name, url := range map[string]string{"http error": failing.URL, "server down": stoppedURL} {
		t.Run(name, func(t *testing.T) {
			configFile, cacheDir := writeAgentConfig(t, url, "")
			writeCache(t, cacheDir, "alice", keys[:1])

			res := runAgent(t, "-config-file", configFile, "alice")

			if res.code != 0 || res.stdout != keys[0].PublicKey+"\n" {
				t.Errorf("exit %d, stdout %q", res.code, res.stdout)
			}
		})
	}
}

func TestAgentCacheFileIsPrivateAndComplete(t *testing.T) {
	keys := fixtureKeys(t)
	srv := keysServer(t, keys)
	configFile, cacheDir := writeAgentConfig(t, srv.URL, "")
	cacheFile := writeCache(t, cacheDir, "alice", keys[:1])

	if res := runAgent(t, "-config-file", configFile, "alice"); res.code != 0 {
		t.Fatalf("exit %d, stderr %q", res.code, res.stderr)
	}

	info, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("cache file mode is %o, want 600", info.Mode().Perm())
	}
	if _, cached := loadCacheFile(cacheFile); len(cached) != len(keys) {
		t.Errorf("cache has %d keys, want %d", len(cached), len(keys))
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only the cache file, found %v", entries)
	}
}

func TestAgentRejectsPathInUsername(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, "[]")
	}))
	t.Cleanup(srv.Close)
	configFile, cacheDir := writeAgentConfig(t, srv.URL, "")

	// The cache path is "<dir>/.<user>.json": a leading slash is what makes
	// the dot-dot a real parent directory reference.
	res := runAgent(t, "-config-file", configFile, "/../escaped")

	if res.code == 0 {
		t.Errorf("expected a failure, got exit 0")
	}
	if requests != 0 {
		t.Errorf("server was queried %d times", requests)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "..", "escaped.json")); err == nil {
		t.Errorf("cache file was written outside the cache directory")
	}
}

func TestAgentAcceptsDotsInUsername(t *testing.T) {
	key := fixtureKeys(t)[0]
	srv := keysServer(t, []Key{key})
	configFile, _ := writeAgentConfig(t, srv.URL, "")

	res := runAgent(t, "-config-file", configFile, "first..last")

	if res.code != 0 || res.stdout != key.PublicKey+"\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", res.code, res.stdout, res.stderr)
	}
}

func TestAgentHostnamePrefixAndSuffixFlags(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, "[]")
	}))
	t.Cleanup(srv.Close)
	configFile, _ := writeAgentConfig(t, srv.URL, "")
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}

	res := runAgent(t, "-config-file", configFile, "-hostname-prefix", "pre-", "-hostname-suffix", "-post", "alice")

	if res.code != 0 {
		t.Fatalf("exit %d, stderr %q", res.code, res.stderr)
	}
	want := fmt.Sprintf("/authorized_keys/pre-%s-post/alice", hostname)
	if gotPath != want {
		t.Errorf("path %q, want %q", gotPath, want)
	}
}

package cmd

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func setFlag[T any](t *testing.T, flag *T, value T) {
	t.Helper()
	previous := *flag
	*flag = value
	t.Cleanup(func() { *flag = previous })
}

func TestWriteConfigYamlIsNotWorldReadable(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	gid, err := strconv.Atoi(current.Gid)
	if err != nil {
		t.Fatal(err)
	}

	for name, existing := range map[string]bool{"new file": false, "existing file": true} {
		t.Run(name, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "config.yml")
			if existing {
				if err := os.WriteFile(configFile, []byte("old"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(configFile, 0644); err != nil {
					t.Fatal(err)
				}
			}
			setFlag(t, configFilePath, configFile)
			setFlag(t, theoUser, current.Username)
			setFlag(t, theoURL, "https://theo.example.com")
			setFlag(t, theoAccessToken, "secret-token")

			writeConfigYaml()

			info, err := os.Stat(configFile)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0640 {
				t.Errorf("mode is %o, want 640", info.Mode().Perm())
			}
			if got := int(info.Sys().(*syscall.Stat_t).Gid); got != gid {
				t.Errorf("group is %d, want %d", got, gid)
			}
			data, err := os.ReadFile(configFile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "token: secret-token\n") {
				t.Errorf("token not written:\n%s", data)
			}
		})
	}
}

func editedSshdConfig(t *testing.T, input string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	setFlag(t, pathSshdConfig, path)
	setFlag(t, theoUser, "theo-agent")
	setFlag(t, passwordAuthentication, false)
	setFlag(t, &validateSshdConfig, func(string) error { return nil })
	_cacheDirPath = "/var/cache/theo-agent"

	if !doEditSshdConfig([2]int64{9, 6}) {
		t.Fatal("doEditSshdConfig failed")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode is %o, want it preserved as 600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const (
	wantKeysFile    = "AuthorizedKeysFile /var/cache/theo-agent/%u\n"
	wantKeysCommand = "AuthorizedKeysCommand /usr/sbin/theo-agent -fingerprint %f -connection %C %u\n"
	wantKeysUser    = "AuthorizedKeysCommandUser theo-agent\n"
	wantPassword    = "PasswordAuthentication no\n"
	wantAll         = wantKeysFile + wantKeysCommand + wantKeysUser + wantPassword
)

func TestDoEditSshdConfig(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "appends to a config without directives",
			input: "Port 22\n",
			want:  "Port 22\n" + wantAll,
		},
		{
			name:  "adds a missing final newline",
			input: "Port 22",
			want:  "Port 22\n" + wantAll,
		},
		{
			name: "replaces existing lines in place",
			input: "AuthorizedKeysCommand /bin/old\nAuthorizedKeysCommandUser nobody\n" +
				"AuthorizedKeysFile .ssh/authorized_keys\nPasswordAuthentication yes\nPort 22\n",
			want: wantKeysCommand + wantKeysUser + wantKeysFile + wantPassword + "Port 22\n",
		},
		{
			name:  "does not confuse AuthorizedKeysCommandUser with AuthorizedKeysCommand",
			input: "AuthorizedKeysCommandUser nobody\nPort 22\n",
			want:  wantKeysUser + "Port 22\n" + wantKeysFile + wantKeysCommand + wantPassword,
		},
		{
			name:  "matches keywords in any case and with an equals sign",
			input: "passwordauthentication yes\nauthorizedkeysfile=.ssh/authorized_keys\n",
			want:  wantPassword + wantKeysFile + wantKeysCommand + wantKeysUser,
		},
		{
			name:  "keeps comments",
			input: "#PasswordAuthentication yes\n",
			want:  "#PasswordAuthentication yes\n" + wantAll,
		},
		{
			name:  "inserts before the first Match block",
			input: "Port 22\nMatch User foo\n  X11Forwarding no\n",
			want:  "Port 22\n" + wantAll + "Match User foo\n  X11Forwarding no\n",
		},
		{
			name:  "does not touch directives inside Match blocks",
			input: "match User foo\n  PasswordAuthentication yes\nMatch all\nAuthorizedKeysFile none\n",
			want:  wantAll + "match User foo\n  PasswordAuthentication yes\nMatch all\nAuthorizedKeysFile none\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editedSshdConfig(t, tt.input); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestReplaceSshdConfigRejectedKeepsOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	if err := os.WriteFile(path, []byte("Port 22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var validated string
	setFlag(t, &validateSshdConfig, func(tmp string) error {
		data, err := os.ReadFile(tmp)
		if err != nil {
			t.Fatal(err)
		}
		validated = string(data)
		return errors.New("bad configuration")
	})

	if err := replaceSshdConfig(path, "Broken yes\n"); err == nil {
		t.Error("expected an error from the validator")
	}

	if validated != "Broken yes\n" {
		t.Errorf("validator received %q", validated)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Port 22\n" {
		t.Errorf("original file was changed: %q", data)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestReplaceSshdConfigKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real_config")
	link := filepath.Join(dir, "sshd_config")
	if err := os.WriteFile(target, []byte("Port 22\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	setFlag(t, &validateSshdConfig, func(string) error { return nil })

	if err := replaceSshdConfig(link, "Port 2222\n"); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symbolic link was replaced: %v, %v", info, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("mode is %o, want 640", info.Mode().Perm())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Port 2222\n" {
		t.Errorf("target content is %q", data)
	}
}

func TestUsesPlainHTTP(t *testing.T) {
	for url, want := range map[string]bool{
		"http://theo.example.com":  true,
		"HTTP://theo.example.com":  true,
		"https://theo.example.com": false,
		"theo.example.com":         false,
	} {
		if got := usesPlainHTTP(url); got != want {
			t.Errorf("%s: got %v, want %v", url, got, want)
		}
	}
}

func TestParseSSHDVersion(t *testing.T) {
	tests := []struct {
		output       string
		major, minor int64
	}{
		{"sshd: illegal option -- v\nOpenSSH_10.3p1, LibreSSL 3.3.6\nusage: sshd [-46]", 10, 3},
		{"unknown option -- v\nOpenSSH_9.6p1 Ubuntu-3ubuntu13, OpenSSL 3.0.13\n", 9, 6},
		{"OpenSSH_7.4p1, OpenSSL 1.0.2k-fips  26 Jan 2017\n", 7, 4},
		{"OpenSSH_9", 0, 0},
		{"OpenSSH_", 0, 0},
		{"OpenSSH", 0, 0},
		{"OpenSSH_a.b", 0, 0},
		{"", 0, 0},
	}
	for _, tt := range tests {
		major, minor := parseSSHDVersion(strings.Fields(tt.output))
		if major != tt.major || minor != tt.minor {
			t.Errorf("%q: got %d.%d, want %d.%d", tt.output, major, minor, tt.major, tt.minor)
		}
	}
}

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

type SshConfig struct {
	key   string
	value string
}

var _cacheDirPath string

// validateSshdConfig checks a candidate sshd_config before it replaces the current one
var validateSshdConfig = func(path string) error {
	out, err := exec.Command("sshd", "-t", "-f", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func getSshConfigs(user string, verify bool, version [2]int64) []SshConfig {
	var commandOpts = ""
	if version[0] < 6 || (version[0] == 6 && version[1] < 9) {
		commandOpts = ""
	} else if version[0] < 9 || (version[0] == 9 && version[1] < 4) {
		commandOpts = "-fingerprint %f %u"
	} else {
		commandOpts = "-fingerprint %f -connection %C %u"
	}

	var sshconfigs = []SshConfig{
		SshConfig{"AuthorizedKeysFile", fmt.Sprintf("%s%s", _cacheDirPath, `/%u`)},
		SshConfig{"AuthorizedKeysCommand", fmt.Sprintf("/usr/sbin/theo-agent %s", commandOpts)},
		SshConfig{"AuthorizedKeysCommandUser", user},
	}

	if !*passwordAuthentication {
		sshconfigs = append(sshconfigs, SshConfig{"PasswordAuthentication", "no"})
	}

	if *useDNS {
		sshconfigs = append(sshconfigs, SshConfig{"UseDNS", "yes"})
	}

	return sshconfigs
}

// Install will update sshd_condif if requested, create cache directory
func Install() {
	major, minor := getSSHDVersion()
	if major < 6 || (major == 6 && minor < 2) {
		fmt.Fprintf(os.Stderr, "Current OpenSSH version (%d.%d) does not support AuthorizedKeysCommand which is available from version 6.2\n", major, minor)
		os.Exit(1)
	}
	prepareInstall()
	checkConfig()
	version := [2]int64{major, minor}
	if *cacheDirPath != "" {
		_cacheDirPath = *cacheDirPath
	} else {
		_cacheDirPath = K_CACHE_PATH
	}
	mkdirs()
	writeConfigYaml()
	if *editSshdConfig {
		doEditSshdConfig(version)
	} else {
		fmt.Fprintf(os.Stderr, "You didn't specify -sshd-config so you have to edit manually /etc/ssh/sshd_config:\n\n")
		i := 0
		sshconfigs := getSshConfigs(*theoUser, *verify, version)
		for i < len(sshconfigs) {
			fmt.Fprintf(os.Stderr, "%s %s\n", sshconfigs[i].key, sshconfigs[i].value)
			i++
		}
	}
}

func prepareInstall() {

	askOnce("Theo server URL", theoURL)
	if *theoURL == "" {
		fmt.Fprintf(os.Stderr, "Missing required Theo URL\n")
		os.Exit(2)
	}
	if usesPlainHTTP(*theoURL) {
		fmt.Fprintf(os.Stderr, "Warning: %s is not using HTTPS, the access token and the keys will travel in clear text\n", *theoURL)
	}

	askOnce("Theo access token", theoAccessToken)
	if *theoAccessToken == "" {
		fmt.Fprintf(os.Stderr, "Missing required Theo access token\n")
		os.Exit(2)
	}

	if *verify {
		askOnce("Public key path", publicKeyPath)
		if *publicKeyPath == "" {
			fmt.Fprintf(os.Stderr, "If -verify flag is true, Public Key path is required\n")
			os.Exit(2)
		}
	}
}

func usesPlainHTTP(url string) bool {
	return strings.HasPrefix(strings.ToLower(url), "http://")
}

func askOnce(prompt string, result *string) {
	if *noInteractive {
		return
	}

	fmt.Println(prompt)

	if *result != "" {
		fmt.Printf("[%s]: ", *result)
	}

	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}

	data, _, err := reader.ReadLine()
	if err != nil {
		panic(err)
	}

	newResult := string(data)
	newResult = strings.TrimSpace(newResult)

	if newResult != "" {
		*result = newResult
	}
}

func mkdirs() {

	dirs := [2]string{path.Dir(*configFilePath), _cacheDirPath}
	for i := 0; i < len(dirs); i++ {
		ensureDir(dirs[i])
	}
	user := lookupUser()
	uid, err := strconv.Atoi(user.Uid)
	if err == nil {
		os.Chown(_cacheDirPath, uid, -1)
	}
}

func lookupUser() *user.User {
	user, err := user.Lookup(*theoUser)
	if err != nil {
		panic(fmt.Sprintf("Unable to find user (%s): %s", *theoUser, err))
	}
	return user
}

func ensureDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.Mkdir(path, 0755)
		if err != nil {
			panic(fmt.Sprintf("Unable to create dir (%s): %s", path, err))
		}

	}
}

func checkConfig() {
	_, ret := performQuery("test", *theoURL, *theoAccessToken)
	if ret > 0 {
		panic(fmt.Sprintf("Check failed, unable to retrieve keys from %s", *theoURL))
	}
}

func writeConfigYaml() {
	_publicKeyPath := ""
	if *verify {
		_publicKeyPath = fmt.Sprintf("verify: True\npublic_key: %s\n", *publicKeyPath)
	}

	__cacheDirPath := fmt.Sprintf("cachedir: %s\n", _cacheDirPath)

	_hostnamePrefix := ""
	if *cfgHostnamePrefix != "" {
		_hostnamePrefix = fmt.Sprintf("hostname-prefix: %s\n", *cfgHostnamePrefix)
	}
	_hostnameSuffix := ""
	if *cfgHostnameSuffix != "" {
		_hostnameSuffix = fmt.Sprintf("hostname-suffix: %s\n", *cfgHostnameSuffix)
	}

	config := fmt.Sprintf("url: %s\ntoken: %s\n%s%s%s%s", *theoURL, *theoAccessToken, _publicKeyPath, __cacheDirPath, _hostnamePrefix, _hostnameSuffix)
	// The file holds the access token: only root and the agent's group can read it.
	gid, err := strconv.Atoi(lookupUser().Gid)
	if err == nil {
		err = writeFileAtomic(*configFilePath, []byte(config), 0640, -1, gid, nil)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to write config file (%s): %s\n", *configFilePath, err)
		os.Exit(21)
	}
}

func doEditSshdConfig(version [2]int64) bool {
	data, err := os.ReadFile(*pathSshdConfig)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read %s, %s\n", *pathSshdConfig, err)
		}
		return false
	}

	if *backupSshdConfig {
		pathSshdConfigBackup := fmt.Sprintf("%s%s", *pathSshdConfig, ".backup")
		err = os.WriteFile(pathSshdConfigBackup, data, 0640)
		if err != nil {
			fmt.Println("Error creating sshd_config backup file", pathSshdConfigBackup)
			fmt.Println(err)
		}
	}

	sshconfigs := getSshConfigs(*theoUser, *verify, version)
	err = replaceSshdConfig(*pathSshdConfig, updateSshdConfig(string(data), sshconfigs))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to write config file (%s): %s\n", *pathSshdConfig, err)
		os.Exit(21)
	}

	return true
}

// updateSshdConfig sets the given options in the global section of an
// sshd_config. Anything after the first Match keyword only applies to that
// block, so options are never changed or added there.
func updateSshdConfig(content string, sshconfigs []SshConfig) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	insertAt := len(lines)
	for i, line := range lines {
		keyword := sshdKeyword(line)
		if strings.EqualFold(keyword, "Match") {
			insertAt = i
			break
		}
		for ii, sshconfig := range sshconfigs {
			if strings.EqualFold(keyword, sshconfig.key) {
				lines[i] = sshdConfigLine(sshconfig)
				sshconfigs = slices.Delete(sshconfigs, ii, ii+1)
				break
			}
		}
	}
	added := make([]string, 0, len(sshconfigs))
	for _, sshconfig := range sshconfigs {
		added = append(added, sshdConfigLine(sshconfig))
	}
	lines = slices.Insert(lines, insertAt, added...)
	return strings.Join(lines, "\n") + "\n"
}

func sshdKeyword(line string) string {
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '='
	})
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func sshdConfigLine(sshconfig SshConfig) string {
	return strings.Trim(fmt.Sprintf("%s %s", sshconfig.key, sshconfig.value), " ")
}

// replaceSshdConfig validates content as a complete sshd_config and installs
// it, keeping the mode and the owner of the current file. A symbolic link at
// path is kept as well: its target is the file being replaced.
func replaceSshdConfig(path string, content string) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("unable to read the owner of %s", target)
	}
	return writeFileAtomic(target, []byte(content), info.Mode().Perm(), int(stat.Uid), int(stat.Gid), validateSshdConfig)
}

func getSSHDVersion() (int64, int64) {
	// sshd has no version flag: it prints its version with the usage message
	// and exits with an error.
	out, err := exec.Command("sshd", "-v").CombinedOutput()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		fmt.Fprintf(os.Stderr, "Unable to get sshd version: %s", err)
		os.Exit(20)
	}
	return parseSSHDVersion(strings.Fields(string(out)))
}

func parseSSHDVersion(openssh []string) (int64, int64) {
	var version = "0.0"
	i := 0
	for i < len(openssh) {
		if strings.HasPrefix(openssh[i], "OpenSSH_") {
			version = strings.TrimPrefix(openssh[i], "OpenSSH_")
			break
		}
		i++
	}
	if version != "0.0" {
		version = strings.TrimSuffix(version, ",")
		ppos := strings.Index(version, "p")
		if ppos > 0 {
			version = version[:ppos]
		}
	}
	majorMinor := strings.Split(version, ".")
	if len(majorMinor) < 2 {
		return 0, 0
	}
	major, err := strconv.ParseInt(majorMinor[0], 10, 64)
	if err == nil {
		minor, err := strconv.ParseInt(majorMinor[1], 10, 64)
		if err == nil {
			return major, minor
		}
	}
	return 0, 0
}

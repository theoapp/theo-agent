package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
)

type SshConfig struct {
	key   string
	value string
}

func getSshConfigs(user string, verify bool) []SshConfig {
	var commandOpts = ""
	if verify {
		commandOpts = " -verify %u"
	}
	var sshconfigs = []SshConfig{
		SshConfig{"PasswordAuthentication", "no"},
		SshConfig{"AuthorizedKeysFile", "/var/cache/theo-agent/%u"},
		SshConfig{"AuthorizedKeysCommand", fmt.Sprintf("/usr/sbin/theo-agent%s", commandOpts)},
		SshConfig{"AuthorizedKeysCommandUser", user},
	}
	return sshconfigs
}

// Install will update sshd_condif if requested, create cache directory
func Install() {
	prepareInstall()
	checkConfig()
	mkdirs()
	writeConfigYaml()
	if *editSshdConfig {
		doEditSshdConfig()
	} else {
		fmt.Fprintf(os.Stderr, "You didn't specify -sshd-config so you have to edit manually /etc/ssh/sshd_config:\n\n")
		i := 0
		sshconfigs := getSshConfigs(*theoUser, *verify)
		for i < len(sshconfigs) {
			line := fmt.Sprintf("%s %s\n", sshconfigs[i].key, sshconfigs[i].value) // I have to go through fmt.Sprintf because of %%u in sshconfigs[i].value
			fmt.Fprintf(os.Stderr, line)
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
	dirs := [2]string{path.Dir(*configFilePath), *cacheDirPath}
	for i := 0; i < len(dirs); i++ {
		ensureDir(dirs[i])
	}
	user := lookupUser()
	uid, err := strconv.Atoi(user.Uid)
	if err == nil {
		os.Chown(*cacheDirPath, uid, -1)
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
	ret := Query("test", theoURL, theoAccessToken)
	if ret > 0 {
		panic(fmt.Sprintf("Check failed, unable to retrieve keys from %s", *theoURL))
	}
}

func writeConfigYaml() {
	_publicKeyPath := ""
	if *verify {
		_publicKeyPath = fmt.Sprintf("public_key: %s\n", *publicKeyPath)
	}
	config := fmt.Sprintf("url: %s\ntoken: %s\n%s", *theoURL, *theoAccessToken, _publicKeyPath)
	f, err := os.Create(*configFilePath)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to write config file (%s): %s", *configFilePath, err)
		}
		os.Exit(21)
	}
	defer f.Close()

	_, err = f.WriteString(config)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to write config file (%s): %s", *configFilePath, err)
		}
		os.Exit(21)
	}
}

func doEditSshdConfig() bool {

	data, err := ioutil.ReadFile(*pathSshdConfig)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read %s, %s", pathSshdConfig, err)
		}
		return false
	}
	lines := strings.Split(string(data), "\n")
	i := 0
	sshconfigs := getSshConfigs(*theoUser, *verify)
	for i < len(lines) {
		line := lines[i]
		ii := 0

		for ii < len(sshconfigs) {
			p := strings.Index(line, sshconfigs[ii].key)
			if p >= 0 {
				lines[i] = fmt.Sprintf("%s %s", sshconfigs[ii].key, sshconfigs[ii].value)
				sshconfigs = remove(sshconfigs, ii)
				break
			}
			ii++
		}
		i++
	}
	ii := 0
	for ii < len(sshconfigs) {
		lines = append(lines, fmt.Sprintf("%s %s", sshconfigs[ii].key, sshconfigs[ii].value))
		ii++
	}

	f, err := os.Create(*pathSshdConfig)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to write config file (%s): %s", pathSshdConfig, err)
		}
		os.Exit(21)
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(lines, "\n"))
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to write config file (%s): %s", pathSshdConfig, err)
		}
		os.Exit(21)
	}

	return true
}

func remove(s []SshConfig, i int) []SshConfig {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

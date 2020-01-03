package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/theoapp/theo-agent/common"
)

const K_CONFIG_FILE = "/etc/theo-agent/config.yml"
const K_CACHE_PATH = "/var/cache/theo-agent"
const K_USER = "theo-agent"

var reader *bufio.Reader

var version = flag.Bool("version", false, "Print theo-agent version")
var install = flag.Bool("install", false, "Install theo-agent")
var noInteractive = flag.Bool("no-interactive", false, "Don't ask, just try to work!")
var debug = flag.Bool("debug", false, "Print debug messages")
var theoURL = flag.String("url", "", "Theo server URL")
var theoUser = flag.String("user", K_USER, "User that will run theo-agent")
var theoAccessToken = flag.String("token", "", "Theo access token")
var verify = flag.Bool("verify", false, "Verify keys' signatures")
var publicKeyPath = flag.String("public-key", "", "Public key path - Used to verify signature")
var configFilePath = flag.String("config-file", K_CONFIG_FILE, "Path to theo agent config file")
var cacheDirPath = flag.String("cache-path", "", fmt.Sprintf("Path to store cached authorized_keys file (default %s)", K_CACHE_PATH))
var editSshdConfig = flag.Bool("sshd-config", false, "Edit sshd_config")
var backupSshdConfig = flag.Bool("sshd-config-backup", false, "Make a backup copy of sshd_config")
var pathSshdConfig = flag.String("sshd-config-path", "/etc/ssh/sshd_config", "The path to sshd_config")
var sshFingerprint = flag.String("fingerprint", "", "The fingerprint of the key or certificate. (Token %f)")
var cfgHostnamePrefix = flag.String("hostname-prefix", "", "Add a prefix to hostname when query server")
var cfgHostnameSuffix = flag.String("hostname-suffix", "", "Add a suffix to hostname when query server")
var passwordAuthentication = flag.Bool("with-password-authentication", false, "sshd: do not disable PasswordAuthentication (Use it only when testing!)")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  %s [OPTIONS] LOGIN\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *version {
		common.AppVersion.Printer()
		os.Exit(0)
	}
	if *install {
		Install()
		os.Exit(0)
	}

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	Query(flag.Arg(0))
}

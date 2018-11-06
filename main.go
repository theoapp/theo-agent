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
var configFilePath = flag.String("config-file", K_CONFIG_FILE, "Path to theo agent config file")
var cacheDirPath = flag.String("cache-path", K_CACHE_PATH, "Path to store cached authorized_keys file")
var editSshdConfig = flag.Bool("sshd-config", false, "Edit sshd_config")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  %s [OPTIONS]\n\nOptions:\n", os.Args[0])
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
	Query(flag.Arg(0), nil, nil)
}

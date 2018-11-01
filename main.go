package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"gopkg.in/yaml.v2"
)

var reader *bufio.Reader

var install = flag.Bool("install", false, "Install theo-agent")
var noInteractive = flag.Bool("no-interactive", false, "Don't ask, just try to work!")
var theoUrl = flag.String("url", "", "Theo server URL")
var theoAccessToken = flag.String("token", "", "Theo access token")
var configFilePath = flag.String("config-file", "/etc/theo-agent/config.yml", "Path to theo agent config file")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  %s [OPTIONS]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *install {
		prepareInstall()
		os.Exit(0)
	}
	
	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}
	performQuery(os.Args[1])
}

func prepareInstall() {

	askOnce("Theo server URL", theoUrl)
	if *theoUrl == "" {
		fmt.Fprintf(os.Stderr, "Missing required Theo URL\n")
		os.Exit(3)
	}

	askOnce("Theo access token", theoAccessToken)
	if *theoAccessToken == "" {
		panic("Missing required Theo access token\n")
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

	if *result == "" {
		panic("Can't be left empty!")
	}
}

func performQuery(user string) {
	
	data, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		panic(err)
	}

	name, xerr := os.Hostname()
	if xerr != nil {
		panic("Unable to obtain hostname")
	}	

	config := make(map[interface{}]interface{})
    
	yerr := yaml.Unmarshal([]byte(data), &config)
	if yerr != nil {
		panic("Unable to parse config file")
	}
	remoteUrl := fmt.Sprintf("%s/authorized_keys/%s/%s", config["url"], name, user)
	
	req, err := http.NewRequest(http.MethodGet, remoteUrl, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config["token"]))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		retFromFile(user)
		return
	}
	fmt.Println(string(body))
	err = ioutil.WriteFile(getUserFilename(user) , body, 0644)
	if err != nil {
		os.Exit(21)
	}
	
}

func getUserFilename(user string ) string {
	return fmt.Sprintf("/var/cache/theo/%s", user)
}

func retFromFile(user string) {
	dat, err := ioutil.ReadFile(getUserFilename(user))
    if err != nil {
		os.Exit(2)
	}
    fmt.Print(string(dat))
}
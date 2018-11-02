package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/theoapp/theo-agent/common"
	"gopkg.in/yaml.v2"
)

func Query(user string, url *string, token *string) int {
	if url == nil || token == nil {
		config, ret := parseConfig()
		if ret > 0 {
			os.Exit(ret)
		}
		body, ret := performQuery(user, config["url"], config["token"])
		fmt.Println(string(body))
		if ret == 0 {
			ret = writeCacheFile(user, body)
		} else if ret == 20 {
			ret = retFromFile(user)
		}

		os.Exit(ret)
	} else {
		_, ret := performQuery(user, *url, *token)
		return ret
	}
	return 0
}

func performQuery(user string, url string, token string) ([]byte, int) {

	remoteUrl := fmt.Sprintf("%s/authorized_keys/%s/%s", url, loadHostname(), user)

	req, err := http.NewRequest(http.MethodGet, remoteUrl, nil)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to get remote URL (%s): %s\n", remoteUrl, err)
		}
		return nil, 8
	}

	req.Header.Set("User-Agent", common.AppVersion.UserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to fetch authorized_keys (%s): %s\n", remoteUrl, err)
		}
		return nil, 9
	}

	defer resp.Body.Close()
	if resp.StatusCode > 399 {
		if *debug {
			fmt.Fprintf(os.Stderr, "HTTP response error from %s: %d\n", remoteUrl, resp.StatusCode)
		}
		return nil, 20
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to parse HTTP response from %s: %s\n", remoteUrl, err)
		}
		return nil, 20
	}
	return body, 0
}

func writeCacheFile(user string, body []byte) int {
	err := ioutil.WriteFile(getUserFilename(user), body, 0644)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to write cache file (%s): %s\n", getUserFilename(user), err)
		}
		return 21
	}
	return 0
}

func getUserFilename(user string) string {
	return fmt.Sprintf("%s/%s", *cacheDirPath, user)
}

func retFromFile(user string) int {
	dat, err := ioutil.ReadFile(getUserFilename(user))
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read cache file (%s): %s\n", getUserFilename(user), err)
		}
		return 2
	}
	fmt.Print(string(dat))
	return 0
}

func loadConfig() ([]byte, int) {
	data, err := ioutil.ReadFile(*configFilePath)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read configFile (%s): %s\n", *configFilePath, err)
		}
		return nil, 5
	}
	return data, 0
}

func loadHostname() (hostname string) {
	hostname, err := os.Hostname()
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to obtain hostname: %s\n", err)
		}
		os.Exit(6)
	}
	return
}

func parseConfig() (map[string]string, int) {
	config := make(map[string]string)
	data, ret := loadConfig()
	if ret > 0 {
		return nil, ret
	}
	err := yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to parse config file (%s): %s\n", *configFilePath, err)
		}
		return nil, 7
	}
	return config, 0
}

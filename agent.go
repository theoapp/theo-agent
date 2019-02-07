package main

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	urlu "net/url"
	"os"

	"github.com/theoapp/theo-agent/common"
	"gopkg.in/yaml.v2"
)

// Query makes a request to Theo server at url sending auth token for the requested user
func Query(user string, url *string, token *string) int {
	if url == nil || token == nil {
		config, ret := parseConfig()
		if ret > 0 {
			os.Exit(ret)
		}
		body, ret := performQuery(user, config["url"], config["token"])
		if ret == 0 {
			if *verify {
				var _publicKeyPath string
				if *publicKeyPath != "" {
					_publicKeyPath = *publicKeyPath
				} else {
					_publicKeyPath = config["public_key"]
				}
				if _publicKeyPath == "" {
					fmt.Fprintf(os.Stderr, "-verify flag is on, but no public key set")
					os.Exit(10)
				}
				b, err := verifyKeys(_publicKeyPath, body)
				if err != nil {
					os.Exit(9)
				}
				body = b
			}
		}
		if ret == 0 {
			fmt.Println(string(body))
			ret = writeCacheFile(user, body)
		} else {
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

	remotePath := fmt.Sprintf("authorized_keys/%s/%s", urlu.PathEscape(loadHostname()), urlu.PathEscape(user))
	remoteURL := fmt.Sprintf("%s/%s", url, remotePath)
	if *sshFingerprint != "" {
		remoteURL = fmt.Sprintf("%s?f=%s", remoteURL, urlu.QueryEscape(*sshFingerprint))
	}
	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to get remote URL (%s): %s\n", remoteURL, err)
		}
		return nil, 8
	}

	req.Header.Set("User-Agent", common.AppVersion.UserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	if *verify {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to fetch authorized_keys (%s): %s\n", remoteURL, err)
		}
		return nil, 9
	}

	defer resp.Body.Close()
	if resp.StatusCode > 399 {
		if *debug {
			fmt.Fprintf(os.Stderr, "HTTP response error from %s: %d\n", remoteURL, resp.StatusCode)
		}
		return nil, 20
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to parse HTTP response from %s: %s\n", remoteURL, err)
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
	return fmt.Sprintf("%s/.%s", *cacheDirPath, user)
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

func verifyKeys(publicKeyPath string, body []byte) ([]byte, error) {

	type Key struct {
		Public_key     string
		Public_key_sig string
	}

	// keys := make([]Key, 0)
	var keys []Key
	if err := json.Unmarshal(body, &keys); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse json response : %s\n", err)
		return nil, err
	}
	var b bytes.Buffer
	parser, perr := loadPublicKey(publicKeyPath)
	if perr != nil {
		fmt.Fprintf(os.Stderr, "could not load public key: %v\n", perr)
		return nil, perr
	}
	for i := 0; i < len(keys); i++ {
		key := keys[i]
		signature, _ := hex.DecodeString(key.Public_key_sig)
		err := parser.Verify([]byte(key.Public_key), signature)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error from verification: %s\n", err)
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(key.Public_key)
	}
	return b.Bytes(), nil
}

func loadPublicKey(path string) (Verifier, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read public.pem (%s): %s\n", path, err)
		}
		return nil, err
	}
	return parsePublicKey(data)
}

func parsePublicKey(pemBytes []byte) (Verifier, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("public key file does not contains any key")
	}

	var rawkey interface{}
	switch block.Type {
	case "PUBLIC KEY":
		rsa, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		rawkey = rsa
	default:
		return nil, fmt.Errorf("rsa: unsupported key type %q", block.Type)
	}

	return newVerifierFromKey(rawkey)
}

func newVerifierFromKey(k interface{}) (Verifier, error) {
	var sshKey Verifier
	switch t := k.(type) {
	case *rsa.PublicKey:
		sshKey = &rsaPublicKey{t}
	default:
		return nil, fmt.Errorf("rsa: unsupported key type %T", k)
	}
	return sshKey, nil
}

type rsaPublicKey struct {
	*rsa.PublicKey
}

// Verifier verifies signature against a public key.
type Verifier interface {
	// Sign returns raw signature for the given data. This method
	// will apply the hash specified for the keytype to the data.
	Verify(data []byte, sig []byte) error
}

// Unsign verifies the message using a rsa-sha256 signature
func (r *rsaPublicKey) Verify(message []byte, signature []byte) error {
	h := sha256.New()
	h.Write(message)
	d := h.Sum(nil)
	return rsa.VerifyPKCS1v15(r.PublicKey, crypto.SHA256, d, signature)
}

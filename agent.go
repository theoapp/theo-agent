package main

import (
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
	"strconv"

	gsyslog "github.com/hashicorp/go-syslog"
	"golang.org/x/crypto/ssh"

	"github.com/theoapp/theo-agent/common"
	"gopkg.in/yaml.v2"
)

// Key is the object returned by theo-node
type Key struct {
	PublicKey    string `json:"public_key"`
	PublicKeySig string `json:"public_key_sig"`
	Account      string `json:"email"`
}

var parser Verifier

var config map[string]string

// Query makes a request to Theo server at url sending auth token for the requested user
func Query(user string) {
	var ret int
	config, ret = parseConfig()
	if ret > 0 {
		os.Exit(ret)
	}
	var keys []Key
	_theoURL := config["url"]
	if *theoURL != "" {
		_theoURL = *theoURL
	}
	_theoToken := config["token"]
	if *theoAccessToken != "" {
		_theoToken = *theoAccessToken
	}
	body, ret := performQuery(user, _theoURL, _theoToken)
	if ret == 0 {
		var _publicKeyPath string
		_verify := false
		if *verify {
			_verify = true
		} else {

			if *verify {
				_verify = *verify
			} else {
				if val, ok := config["verify"]; ok {
					if s, err := strconv.ParseBool(val); err == nil {
						_verify = s
					}
				}
			}
		}
		if _verify {
			if *publicKeyPath != "" {
				_publicKeyPath = *publicKeyPath
			} else {
				_publicKeyPath = config["public_key"]
			}
			if _publicKeyPath == "" {
				fmt.Fprintf(os.Stderr, "-verify flag is on, but no public key set")
				os.Exit(10)
			}
		}
		var err error
		keys, err = verifyKeys(_publicKeyPath, body)
		if err != nil {
			os.Exit(9)
		}
		ret = writeCacheFile(user, keys)
	}
	if ret != 0 {
		ret, keys = retFromFile(user)
		if ret > 0 {
			os.Exit(9)
		}
	}
	for i := 0; i < len(keys); i++ {
		if keys[i].Account != "" {
			if *sshFingerprint != "" {
				sshpk := parseSSHPublicKey(keys[i].PublicKey)
				f := ssh.FingerprintSHA256(sshpk)
				if f == *sshFingerprint {
					_, err := fmt.Printf("%s\n", keys[i].PublicKey)
					if err != nil {
						break
					}
					a, b := gsyslog.NewLogger(gsyslog.LOG_INFO, "AUTH", "theo-agent")
					if b == nil {
						a.Write([]byte(fmt.Sprintf("Account %s logged in as %s\n", keys[i].Account, user)))
					}
				}
			} else {
				_, err := fmt.Printf("%s\n", keys[i].PublicKey)
				if err != nil {
					break
				}
			}
		} else {
			_, err := fmt.Printf("%s\n", keys[i].PublicKey)
			if err != nil {
				break
			}
		}
	}
	os.Exit(ret)

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
	req.Header.Set("Accept", "application/json")

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

func writeCacheFile(user string, keys []Key) int {
	body, _ := json.Marshal(keys)
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
	_cacheDirPath := config["cachedir"]
	if *cacheDirPath != "" {
		_cacheDirPath = *cacheDirPath
	}
	if _cacheDirPath == "" {
		_cacheDirPath = K_CACHE_PATH
	}
	fmt.Fprintf(os.Stderr, "cacheDir: %s\n", _cacheDirPath)
	return fmt.Sprintf("%s/.%s.json", _cacheDirPath, user)
}

func retFromFile(user string) (int, []Key) {
	dat, err := ioutil.ReadFile(getUserFilename(user))
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read cache file (%s): %s\n", getUserFilename(user), err)
		}
		return 0, nil
	}
	var keys []Key
	if err := json.Unmarshal(dat, &keys); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse json file : %s\n", err)
		return 0, nil
	}
	return 0, keys
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

func verifyKeys(publicKeyPath string, body []byte) ([]Key, error) {

	// keys := make([]Key, 0)
	var keys []Key
	var retKeys []Key
	if err := json.Unmarshal(body, &keys); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse json response : %s\n", err)
		return nil, err
	}

	var perr error
	if publicKeyPath != "" {
		perr = loadPublicKey(publicKeyPath)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "could not load public key: %v\n", perr)
			return nil, perr
		}
	}

	for i := 0; i < len(keys); i++ {
		key := keys[i]
		if parser != nil {
			signature, _ := hex.DecodeString(key.PublicKeySig)
			err := parser.Verify([]byte(key.PublicKey), signature)
			if err != nil {
				if *debug {
					fmt.Fprintf(os.Stderr, "Error from verification: %s\n", err)
				}
				continue
			}
		}
		retKeys = append(retKeys, key)
	}
	return retKeys, nil
}

func loadPublicKey(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read public.pem (%s): %s\n", path, err)
		}
		return err
	}
	parsePublicKey(data)
	return nil
}

func parsePublicKey(pemBytes []byte) error {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return errors.New("public key file does not contains any key")
	}

	var rawkey interface{}
	switch block.Type {
	case "PUBLIC KEY":
		rsa, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return err
		}

		rawkey = rsa
	default:
		return fmt.Errorf("rsa: unsupported key type %q", block.Type)
	}
	var err error
	parser, err = newVerifierFromKey(rawkey)

	return err
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

func parseSSHPublicKey(publicKey string) ssh.PublicKey {
	pubKeyBytes := []byte(publicKey)

	// Parse the key, other info ignored
	pk, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyBytes)
	if err != nil {
		panic(err)
	}
	return pk
}

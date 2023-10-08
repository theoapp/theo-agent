package cmd

import (
	"context"
	"crypto"
	"crypto/ed25519"
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
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	SSHOptions   string `json:"ssh_options"`
}

type StringArray []string

type Config struct {
	URL            string `yaml:"url"`
	Token          string
	Cachedir       string
	Verify         bool
	PublicKey      StringArray `yaml:"public_key"`
	Timeout        int64
	HostnamePrefix string `yaml:"hostname-prefix"`
	HostnameSuffix string `yaml:"hostname-suffix"`
}

type rsaPublicKey struct {
	*rsa.PublicKey
}

type ed25519PublicKey struct {
	ed25519.PublicKey
}

// Verifier verifies signature against a public key.
type Verifier interface {
	// Sign returns raw signature for the given data. This method
	// will apply the hash specified for the keytype to the data.
	Verify(data []byte, sig []byte) error
}

var config Config

func (a *StringArray) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*a = []string{single}
	} else {
		*a = multi
	}
	return nil
}

// Query makes a request to Theo server at url sending auth token for the requested user
func Query(user string) {
	var ret int
	config, ret = parseConfig("")
	if ret > 0 {
		os.Exit(ret)
	}
	var keys []Key
	_theoURL := config.URL
	if *theoURL != "" {
		_theoURL = *theoURL
	}
	_theoToken := config.Token
	if *theoAccessToken != "" {
		_theoToken = *theoAccessToken
	}
	body, ret := performQuery(user, _theoURL, _theoToken)
	if *debug {
		fmt.Fprintf(os.Stderr, "%s", body)
	}
	userCacheFile := getUserFilename(user)
	if ret == 0 {
		var err error
		keys, err = loadKeysFromBody(body)
		if err != nil {
			os.Exit(9)
		}
		ret = writeCacheFile(userCacheFile, keys)
	} else {
		if *debug {
			fmt.Fprintf(os.Stderr, "Try to read cached keys for %s\n", user)
		}
		ret, keys = loadCacheFile(userCacheFile)
		if ret > 0 {
			fmt.Fprintf(os.Stderr, "Failed to read cached keys\n")
			os.Exit(9)
		}
	}
	if mustVerify() {
		var err error
		publicKeys := getPublicKeys()
		keys, err = verifyKeys(publicKeys, keys)
		if err != nil {
			os.Exit(9)
		}
	}
	if *sshFingerprint != "" {
		keys = filterKeysByFingerprint(*sshFingerprint, user, keys)
	}
	printAuthorizedKeys(keys)
	os.Exit(ret)
}

func mustVerify() bool {
	_verify := false
	if *verify {
		_verify = true
	} else {
		_verify = config.Verify
	}
	return _verify
}

func getPublicKeys() []string {
	_publicKeys := make([]string, 0)

	if *publicKeyPath != "" {
		_publicKeys = append(_publicKeys, *publicKeyPath)
	} else {
		_publicKeys = config.PublicKey
	}
	if len(_publicKeys) == 0 {
		fmt.Fprintf(os.Stderr, "-verify flag is on, but no public key set")
		os.Exit(10)
	}
	return _publicKeys
}

func filterKeysByFingerprint(fingerprint string, user string, keys []Key) []Key {
	retKeys := make([]Key, 0)
	for i := 0; i < len(keys); i++ {
		if keys[i].Account != "" {
			sshpk := parseSSHPublicKey(keys[i].PublicKey)
			f := ssh.FingerprintSHA256(sshpk)
			if f == fingerprint {
				a, b := gsyslog.NewLogger(gsyslog.LOG_INFO, "AUTH", "theo-agent")
				if b == nil {
					a.Write([]byte(fmt.Sprintf("Account %s logged in as %s\n", keys[i].Account, user)))
				}
				retKeys = append(retKeys, keys[i])
				break
			}
		}
	}
	return retKeys
}

func printAuthorizedKeys(keys []Key) {
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
	for i := 0; i < len(keys); i++ {
		_, err := fmt.Printf(getAuthorizedKeysLine(keys[i]))
		if err != nil {
			break
		}
	}
}

func getAuthorizedKeysLine(key Key) string {
	return fmt.Sprintf("%s%s\n", getSSHOptions(key.SSHOptions), key.PublicKey)
}

func getSSHOptions(sshOptions string) string {
	if sshOptions == "" {
		return sshOptions
	}
	return fmt.Sprintf("%s ", sshOptions)
}

func performQuery(user string, url string, token string) ([]byte, int) {

	remotePath := fmt.Sprintf("authorized_keys/%s/%s", urlu.PathEscape(loadHostname()), urlu.PathEscape(user))
	remoteURL := fmt.Sprintf("%s/%s", url, remotePath)

	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to get remote URL (%s): %s\n", remoteURL, err)
		}
		return nil, 8
	}

    q := req.URL.Query()
    if *sshFingerprint != "" {
        q.Add("f", *sshFingerprint)
    }
    if *sshConnection != "" {
        connectionParts := strings.Split(*sshConnection, " ")
        if len(connectionParts) == 4 {
            q.Add("c", connectionParts[2])
        }
    }
    req.URL.RawQuery = q.Encode()

	if *debug {
		fmt.Fprintf(os.Stderr, "Theo URL %s\n", remoteURL)
	}

	DefaultTimeout := int64(5000)
	_timeout := DefaultTimeout
	if config.Timeout > 0 {
		_timeout = config.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(_timeout)*time.Millisecond)
	defer cancel()
	req.Header.Set("User-Agent", common.AppVersion.UserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
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

func writeCacheFile(userCacheFile string, keys []Key) int {
	body, _ := json.Marshal(keys)
	err := ioutil.WriteFile(userCacheFile, body, 0644)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to write cache file (%s): %s\n", userCacheFile, err)
		}
		return 21
	}
	return 0
}

func getUserFilename(user string) string {
	_cacheDirPath := config.Cachedir
	if *cacheDirPath != "" {
		_cacheDirPath = *cacheDirPath
	}
	if _cacheDirPath == "" {
		_cacheDirPath = K_CACHE_PATH
	}
	if *debug {
		fmt.Fprintf(os.Stderr, "cacheDir: %s\n", _cacheDirPath)
	}
	return fmt.Sprintf("%s/.%s.json", _cacheDirPath, user)
}

func loadCacheFile(userCacheFile string) (int, []Key) {
	dat, err := ioutil.ReadFile(userCacheFile)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to read cache file (%s): %s\n", userCacheFile, err)
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

func loadConfig(configFile string) ([]byte, int) {
	if configFile == "" {
		configFile = *configFilePath
	}
	data, err := ioutil.ReadFile(configFile)
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
	if *cfgHostnamePrefix != "" {
		hostname = fmt.Sprintf("%s%s", *cfgHostnamePrefix, hostname)
	} else {
		if config.HostnamePrefix != "" {
			hostname = fmt.Sprintf("%s%s", config.HostnamePrefix, hostname)
		}
	}
	if *cfgHostnameSuffix != "" {
		hostname = fmt.Sprintf("%s%s", *cfgHostnameSuffix, hostname)
	} else {
		if config.HostnameSuffix != "" {
			hostname = fmt.Sprintf("%s%s", hostname, config.HostnameSuffix)
		}
	}
	return
}

func parseConfig(configFile string) (Config, int) {
	config := Config{}
	data, ret := loadConfig(configFile)
	if ret > 0 {
		return config, ret
	}
	err := yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Unable to parse config file (%s): %s\n", *configFilePath, err)
		}
		return config, 7
	}
	return config, 0
}

func loadKeysFromBody(body []byte) ([]Key, error) {
	var keys []Key
	if err := json.Unmarshal(body, &keys); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse json response : %s\n", err)
		return nil, err
	}
	return keys, nil
}

func verifyKeys(publicKey []string, keys []Key) ([]Key, error) {

	retKeys := make([]Key, 0)
	for i := 0; i < len(publicKey); i++ {
		var perr error
		var parser Verifier
		publicKey := strings.Trim(publicKey[i], " ")
		if publicKey == "" {
			continue
		}
		if strings.HasPrefix(publicKey, "-----BEGIN PUBLIC KEY-----") {
			parser, perr = parsePublicKey([]byte(publicKey))
			if perr != nil {
				fmt.Fprintf(os.Stderr, "could not parse public key: %v\n", perr)
				continue
			}
		} else {
			parser, perr = loadPublicKey(publicKey)
			if perr != nil {
				fmt.Fprintf(os.Stderr, "could not load public key: %v\n", perr)
				continue
			}
		}

		for x := 0; x < len(keys); x++ {
			key := keys[x]
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
	}
	return retKeys, nil
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
			fmt.Fprintf(os.Stderr, "Failed ParsePKIXPublicKey:%v\n", err)
			return nil, err
		}

		rawkey = rsa
		break
	default:
		return nil, fmt.Errorf("rsa: unsupported key type %q", block.Type)
	}

	return newVerifierFromKey(rawkey)
}

func newVerifierFromKey(k interface{}) (Verifier, error) {
	var sshKey Verifier

	switch t := k.(type) {
	case ed25519.PublicKey:
		if *debug {
			fmt.Fprintf(os.Stderr, "type is ed25519 %T\n", k)
		}
		sshKey = &ed25519PublicKey{t}
		break
	case *rsa.PublicKey:
		if *debug {
			fmt.Fprintf(os.Stderr, "type is rsa %T\n", k)
		}
		sshKey = &rsaPublicKey{t}
		break
	default:
		if *debug {
			fmt.Fprintf(os.Stderr, "unknown key type %T\n", k)
		}
		return nil, fmt.Errorf("unsupported key type %T", k)
	}
	return sshKey, nil
}

// Unsign verifies the message using a rsa-sha256 signature
func (r *rsaPublicKey) Verify(message []byte, signature []byte) error {
	h := sha256.New()
	h.Write(message)
	d := h.Sum(nil)
	return rsa.VerifyPKCS1v15(r.PublicKey, crypto.SHA256, d, signature)
}

// Unsign verifies the message using a ed25519 signature
func (r *ed25519PublicKey) Verify(message []byte, signature []byte) error {
	ok := ed25519.Verify(r.PublicKey, message, signature)
	if ok {
		return nil
	}
	return errors.New("public key' signature not valid")
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

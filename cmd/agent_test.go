package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

func TestParseConfig(t *testing.T) {
	config, ret := parseConfig("../test/config.1.yml")
	if ret > 0 {
		t.Errorf("parseConfig failed")
	}
	if len(config.PublicKey) != 1 {
		t.Errorf("public_keys len %d expected 1\n", len(config.PublicKey))
	}
	config, ret = parseConfig("../test/config.2.yml")
	if ret > 0 {
		t.Errorf("parseConfig failed")
	}
	if len(config.PublicKey) != 2 {
		t.Errorf("public_keys len %d expected 2\n", len(config.PublicKey))
	}
	config, ret = parseConfig("../test/config.3.yml")
	if ret > 0 {
		t.Errorf("parseConfig failed")
	}
	if len(config.PublicKey) != 1 {
		t.Errorf("public_keys len %d expected 1\n", len(config.PublicKey))
	}
}

func TestVer(t *testing.T) {
	parser, err := loadPublicKey("../test/public.pem")
	if err != nil {
		t.Errorf("loadPublicKey should return nil %s", err)
	}
	userCacheFile := "../test/test.signature.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		fmt.Fprintf(os.Stderr, "Failed to read cached keys\n")
		os.Exit(9)
	}
	signature, _ := hex.DecodeString(keys[0].PublicKeySig)
	err = parser.Verify([]byte(keys[0].PublicKey), signature)
	if err != nil {
		t.Errorf("signature verify failed")
	}
}

func TestVerEmbedPublicKey(t *testing.T) {
	config, ret := parseConfig("../test/config.3.yml")
	if ret > 0 {
		t.Errorf("parseConfig failed")
	}
	userCacheFile := "../test/test.signature.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		fmt.Fprintf(os.Stderr, "Failed to read cached keys\n")
		os.Exit(9)
	}
	parser, err := parsePublicKey([]byte(config.PublicKey[0]))
	signature, _ := hex.DecodeString(keys[0].PublicKeySig)
	err = parser.Verify([]byte(keys[0].PublicKey), signature)
	if err != nil {
		t.Errorf("signature verify failed")
	}
}

func TestSignatures(t *testing.T) {
	userCacheFile := "../test/test.signatures.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		t.Errorf("Failed to read cached keys")
	}
	validKeys := len(keys)
	var err error
	keys, err = verifyKeys([]string{"../test/public2.pem"}, keys)
	if err != nil {
		t.Errorf("Failed to verify keys")
	}
	if len(keys) != validKeys {
		t.Errorf("Keys len must be %d, got %d", validKeys, len(keys))
	}
}

func TestSignaturesWithBrokenSignature(t *testing.T) {
	userCacheFile := "../test/test.signatures.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		t.Errorf("Failed to read cached keys")
	}
	var err error
	keys, err = verifyKeys([]string{"../test/public.pem"}, keys)
	if err != nil {
		t.Errorf("Failed to verify keys")
	}
	if len(keys) != 0 {
		t.Errorf("Keys len must be %d, got %d", 0, len(keys))
	}
}

func TestVerifyKeysMultiplePublicKeys(t *testing.T) {
	userCacheFile := "../test/test.signatures.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		t.Errorf("Failed to read cached keys")
	}
	var err error
	keys, err = verifyKeys([]string{"../test/public.pem", "../test/public2.pem"}, keys)
	if err != nil {
		t.Errorf("Failed to verify keys")
	}
	if len(keys) != 5 {
		t.Errorf("Keys len must be %d, got %d", 5, len(keys))
	}
}

func TestBrokenKey(t *testing.T) {
	userCacheFile := "../test/test.broken.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		fmt.Fprintf(os.Stderr, "Failed to read cached keys\n")
		os.Exit(9)
	}
	var err error
	keys, err = verifyKeys([]string{"../test/public.pem"}, keys)
	if err != nil {
		t.Errorf("Failed to verify keys")
	}
	if len(keys) != 0 {
		t.Errorf("Keys len must be %d, got %d", 0, len(keys))
	}
}

func TestFingerprint(t *testing.T) {
	userCacheFile := "../test/test.signatures.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		t.Errorf("Failed to read cached keys")
	}
	keys = filterKeysByFingerprint("SHA256:d4RXf2B0bUGDaG0UufCX3+vUVxKnIvvIgTYC3bGGH14", "test", keys)
	if len(keys) != 1 {
		t.Errorf("Keys len must be %d, got %d", 1, len(keys))
	}
}

func TestSSHOptions(t *testing.T) {
	userCacheFile := "../test/test.ssh_options.json"
	ret, keys := loadCacheFile(userCacheFile)
	if ret > 0 {
		fmt.Fprintf(os.Stderr, "Failed to read cached keys\n")
		os.Exit(9)
	}
	line := getAuthorizedKeysLine(keys[0])
	if line != "from=\"192.168.2.1,10.10.0.0\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN8g05+ZeElAFktcrUpUyuAsfoNrPk4eH+T2Z20KdBrA macno@jalapeno\n" {
		t.Errorf("authorized_keys line[0] does not match")
	}
	line = getAuthorizedKeysLine(keys[1])
	if line != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN8g05+ZeElAFktcrUpUyuAsfoNrPk4eH+T2Z20KdBrA macno@jalapeno\n" {
		t.Errorf("authorized_keys line[1] does not match")
	}
}

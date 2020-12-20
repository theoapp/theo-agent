package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

func TestVer(t *testing.T) {
	err := loadPublicKey("../test/public.pem")
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

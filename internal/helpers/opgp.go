// TODO: Discuss with AppImage team whether we can switch from GPG to RSA
// and whether this would simplify things and reduce dependencies
// https://socketloop.com/tutorials/golang-saving-private-and-public-key-to-files

package helpers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/openpgp/packet"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"os"

	"github.com/alokmenghrajani/gpgeez"
	"golang.org/x/crypto/openpgp"
)

func CreateAndValidateKeyPair() {
	createKeyPair()

	b, err := ioutil.ReadFile("privkey")
	if err != nil {
		fmt.Println(err)
		return
	}
	hexstring, _ := readPGP(b)
	if err != nil {
		log.Fatal(err)
	}
	err = validate(hexstring)
	if err != nil {
		log.Fatal(err)
	}
}

func readPGP(armoredKey []byte) (string, error) {
	keyReader := bytes.NewReader(armoredKey)
	entityList, err := openpgp.ReadArmoredKeyRing(keyReader)
	if err != nil {
		log.Fatalf("error reading armored key %s", err)
	}
	serializedEntity := bytes.NewBuffer(nil)
	err = entityList[0].Serialize(serializedEntity)
	if err != nil {
		return "", fmt.Errorf("error serializing entity for file %s", err)
	}

	return base64.StdEncoding.EncodeToString(serializedEntity.Bytes()), nil
}

func validate(keystring string) error {
	data, err := base64.StdEncoding.DecodeString(keystring)
	if err != nil {
		return err
	}
	_, err = openpgp.ReadEntity(packet.NewReader(bytes.NewBuffer(data)))
	if err != nil {
		return err
	}
	fmt.Println("PASSED")
	return nil
}

func createKeyPair() {
	config := gpgeez.Config{Expiry: 0 * time.Hour}
	config.RSABits = 4096
	key, err := gpgeez.CreateKey("Signing key", "", "", &config) // TODO: Better name, comment, email
	if err != nil {
		fmt.Printf("Something went wrong while creating key pair: %v", err)
		return
	}
	pubkeyascdata, err := key.Armor()
	if err != nil {
		fmt.Printf("Something went wrong while armoding public key: %v", err)
		return
	}

	privkeyascdata, err := key.ArmorPrivate(&config)
	if err != nil {
		fmt.Printf("Something went wrong while armoding private key: %v", err)
		return
	}

	ioutil.WriteFile(PubkeyFileName, []byte(pubkeyascdata), 0666)
	ioutil.WriteFile(PrivkeyFileName, []byte(privkeyascdata), 0600)
}

// CheckSignature checks the signature embedded in an AppImage at path,
// returns the entity that has signed the AppImage and error
// based on https://stackoverflow.com/a/34008326
func CheckSignature(path string) (*openpgp.Entity, error) {
	var ent *openpgp.Entity
	err := errors.New("could not verify AppImage signature") // Be pessimistic by default, unless we can positively verify the signature
	pubkeybytes, err := GetSectionData(path, ".sig_key")

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pubkeybytes))
	if err != nil {
		return ent, err
	}

	sigbytes, err := GetSectionData(path, ".sha256_sig")

	ent, err = openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(CalculateSHA256Digest(path)), bytes.NewReader(sigbytes))
	if err != nil {
		return ent, err
	}

	return ent, nil
}

// SignAppImage signs an AppImage, returns error
// Based on https://gist.github.com/eliquious/9e96017f47d9bd43cdf9
func SignAppImage(path string, digest string) error {

	// Read in public key
	pubkeyFileBuffer, _ := os.Open(PubkeyFileName)
	defer pubkeyFileBuffer.Close()
	_, err := openpgp.ReadArmoredKeyRing(pubkeyFileBuffer)
	if err != nil {
		fmt.Println("openpgp.ReadArmoredKeyRing error while reading public key:", err)
		return err
	}

	// Read in private key
	privkeyFileBuffer, _ := os.Open(PrivkeyFileName)
	defer privkeyFileBuffer.Close()
	entityList, err := openpgp.ReadArmoredKeyRing(privkeyFileBuffer)
	if err != nil {
		fmt.Println("openpgp.ReadArmoredKeyRing error while reading private key:", err)
		return err
	}

	buf := new(bytes.Buffer)

	// Get the digest we want to sign into an io.Reader
	// FIXME: Use the digest we have already calculated earlier on (let's not do it twice)
	whatToSignReader := strings.NewReader(digest)

	err = openpgp.ArmoredDetachSign(buf, entityList[0], whatToSignReader, nil)
	if err != nil {
		fmt.Println("Error signing input:", err)
		return err
	}

	err = EmbedStringInSegment(path, ".sha256_sig", buf.String())
	if err != nil {
		PrintError("EmbedStringInSegment", err)
		return err
	}
	return nil
}

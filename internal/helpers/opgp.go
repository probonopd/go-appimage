// TODO: Discuss with AppImage team whether we can switch from GPG to RSA
// and whether this would simplify things and reduce dependencies
// https://socketloop.com/tutorials/golang-saving-private-and-public-key-to-files

package helpers

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

func CreateAndValidateKeyPair() {
	createKeyPair()

	b, err := os.ReadFile("privkey")
	if err != nil {
		fmt.Println(err)
		return
	}
	key, err := crypto.NewKey(b)
	if err != nil {
		log.Fatal(err)
	}
	err = validate(key)
	if err != nil {
		log.Fatal(err)
	}
}

func validate(key *crypto.Key) error {
	pub, err := key.ToPublic()
	if err != nil {
		return err
	}

	pgp := crypto.PGP()

	encHandle, err := pgp.Encryption().SigningKey(key).Recipient(pub).New()
	if err != nil {
		return err
	}
	defer encHandle.ClearPrivateParams()

	message, err := encHandle.Encrypt([]byte("Hello world"))
	if err != nil {
		return err
	}
	armored, err := message.Armor()
	if err != nil {
		return err
	}

	decHandle, err := pgp.Decryption().DecryptionKey(key).VerificationKey(pub).New()
	if err != nil {
		return err
	}
	defer decHandle.ClearPrivateParams()

	decrypted, err := decHandle.Decrypt([]byte(armored), crypto.Armor)
	if err != nil {
		return err
	}
	if sigErr := decrypted.SignatureError(); sigErr != nil {
		return sigErr
	}

	fmt.Println("PASSED")
	return nil
}

func createKeyPair() {
	pgp := crypto.PGP()
	handle := pgp.KeyGeneration().New() // TODO: Better name, comment, email
	key, err := handle.GenerateKey()
	if err != nil {
		fmt.Printf("Something went wrong while creating key pair: %v", err)
		return
	}
	pubkeyascdata, err := key.GetArmoredPublicKey()
	if err != nil {
		fmt.Printf("Something went wrong while armoring public key: %v", err)
		return
	}

	privkeyascdata, err := key.Armor()
	if err != nil {
		fmt.Printf("Something went wrong while armoring private key: %v", err)
		return
	}

	os.WriteFile(PubkeyFileName, []byte(pubkeyascdata), 0666)
	os.WriteFile(PrivkeyFileName, []byte(privkeyascdata), 0600)
}

// CheckSignature checks the signature embedded in an AppImage at path,
// returns the key that has signed the AppImage and error
// based on https://stackoverflow.com/a/34008326
func CheckSignature(path string) (*crypto.Key, error) {
	var key *crypto.Key
	err := errors.New("could not verify AppImage signature") // Be pessimistic by default, unless we can positively verify the signature
	pubkeybytes, err := GetSectionData(path, ".sig_key")

	pubkey, err := crypto.NewKey(pubkeybytes)
	if err != nil {
		return key, err
	}

	sigbytes, err := GetSectionData(path, ".sha256_sig")

	pgp := crypto.PGP()
	verifier, err := pgp.Verify().VerificationKey(pubkey).New()
	if err != nil {
		return key, err
	}

	verifyResult, err := verifier.VerifyDetached([]byte(CalculateSHA256Digest(path)), sigbytes, crypto.Armor)
	if err != nil {
		return key, err
	}
	if sigErr := verifyResult.SignatureError(); sigErr != nil {
		return key, sigErr
	}

	signedbyKey := verifyResult.SignedByKey()

	return signedbyKey, nil
}

// SignAppImage signs an AppImage, returns error
// Based on https://gist.github.com/eliquious/9e96017f47d9bd43cdf9
func SignAppImage(path string, digest string) error {

	// Read in public key
	pubkeyFileBuffer, _ := os.Open(PubkeyFileName)
	defer pubkeyFileBuffer.Close()
	_, err := crypto.NewKeyFromReader(pubkeyFileBuffer)
	if err != nil {
		fmt.Println("error while reading public key:", err)
		return err
	}

	// Read in private key
	privkeyFileBuffer, _ := os.Open(PrivkeyFileName)
	defer privkeyFileBuffer.Close()
	privkey, err := crypto.NewKeyFromReader(privkeyFileBuffer)
	if err != nil {
		fmt.Println("error while reading private key:", err)
		return err
	}

	pgp := crypto.PGP()

	signer, err := pgp.Sign().SigningKey(privkey).Detached().New()
	if err != nil {
		fmt.Println("Error creating signer:", err)
		return err
	}
	defer signer.ClearPrivateParams()

	signature, err := signer.Sign([]byte(digest), crypto.Armor)
	if err != nil {
		fmt.Println("Error signing input:", err)
		return err
	}

	err = EmbedStringInSegment(path, ".sha256_sig", string(signature))
	if err != nil {
		PrintError("EmbedStringInSegment", err)
		return err
	}
	return nil
}

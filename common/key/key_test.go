package key

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jbenet/go-base58"
	"github.com/tyler-smith/go-bip32"
)

func TestVertify(t *testing.T) {
	priKey, pubKey, err := GenerateEcdsaKey("test")
	if err != nil {
		panic(err)
	}

	prikeyStr := hexutil.Encode(crypto.FromECDSA(priKey))
	pubkeyStr := hexutil.Encode(crypto.FromECDSAPub(&priKey.PublicKey))
	fmt.Println("signPrikeyStr:", prikeyStr)
	fmt.Println("signPubkeyStr:", pubkeyStr)

	content := "hello world"
	fmt.Printf("content:%v\n", content)
	fmt.Printf("byte content:%v\n", []byte(content))

	signature, err := Sign(priKey, []byte(content))
	if err != nil {
		panic(err)
	}
	fmt.Printf("sign:%v\n", hexutil.Encode(signature))
	fmt.Printf("content:%v\n", content)
	fmt.Printf("byte content:%v\n", []byte(content))
	content = "hello world"
	isVerify, err := Verify(pubKey, []byte(content), signature)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%t\n", isVerify)
	fmt.Printf("content:%v\n", content)
	fmt.Printf("byte content:%v\n", []byte(content))
}

func TestEncrypt(t *testing.T) {
	priKey, pubKey, err := GenerateEcdsaKey("test")
	if err != nil {
		panic(err)
	}
	prikeyStr := hexutil.Encode(crypto.FromECDSA(priKey))
	pubkeyStr := hexutil.Encode(crypto.FromECDSAPub(&priKey.PublicKey))
	fmt.Println("signPrikeyStr:", prikeyStr)
	fmt.Println("signPubkeyStr:", pubkeyStr)

	content := []byte("hello world")
	ciphertext, err := EncryptWithPubkey(pubKey, content)
	if err != nil {
		panic(err)
	}
	fmt.Printf("encrypt data:%s\n", hexutil.Encode(ciphertext))

	decryptContent, err := DecryptWithPrikey(priKey, ciphertext)
	if err != nil {
		panic(err)
	}
	fmt.Printf("decrypt data：%s\n", decryptContent)
}

func TestBip(t *testing.T) {

	// Generate a new seed
	seed := generateSeed("") // or seed := bip39.NewSeed(mnemonic, "") to recover from a mnemonic

	// Get the master key
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()
	fmt.Println("BIP32 Master private key: ", masterKey)
	fmt.Println("BIP32 Master public key: ", publicKey)

	// To use derivate child keys instead, see:
	// https://gist.github.com/miguelmota/f56fa0b01e8c6c649a6c4f0ee7337aab#file-bip32_ecdsa-go-L24

	// Extract the private key
	decoded := base58.Decode(masterKey.B58Serialize())
	privateKey := decoded[46:78]

	// Hex private key to ECDSA private key
	privateKeyECDSA, err := crypto.ToECDSA(privateKey)
	if err != nil {
		panic(err)
	}

	// Import ECDSA key into the keystore
	ks := newKs(".")
	account, err := ks.ImportECDSA(privateKeyECDSA, "")
	if err != nil {
		panic(err)
	}

	// A Web3 Secret Storage JSON file is now stored in account.URL.Path, just print it out
	b, err := ioutil.ReadFile(account.URL.Path)
	if err != nil {
		panic(err)
	}
	fmt.Println("Web3 Secret Storage:")
	fmt.Print(string(b))

	computerVoiceMasterKey, _ := bip32.NewMasterKey(seed)

	// Map departments to keys
	// There is a very small chance a given child index is invalid
	// If so your real program should handle this by skipping the index
	departmentKeys := map[string]*bip32.Key{}
	departmentKeys["Sales"], _ = computerVoiceMasterKey.NewChildKey(0)
	departmentKeys["Marketing"], _ = computerVoiceMasterKey.NewChildKey(1)
	departmentKeys["Engineering"], _ = computerVoiceMasterKey.NewChildKey(2)
	departmentKeys["Customer Support"], _ = computerVoiceMasterKey.NewChildKey(3)

	// Create public keys for record keeping, auditors, payroll, etc
	departmentAuditKeys := map[string]*bip32.Key{}
	departmentAuditKeys["Sales"] = departmentKeys["Sales"].PublicKey()
	departmentAuditKeys["Marketing"] = departmentKeys["Marketing"].PublicKey()
	departmentAuditKeys["Engineering"] = departmentKeys["Engineering"].PublicKey()
	departmentAuditKeys["Customer Support"] = departmentKeys["Customer Support"].PublicKey()

	// Print public keys
	for department, pubKey := range departmentAuditKeys {
		fmt.Println(department, pubKey)
	}
}

func TestLoadEcdsaKeyFromHex(t *testing.T) {
	priKey, pubKey, err := GenerateEcdsaKey("test")
	if err != nil {
		panic(err)
	}
	prikeyStr := hex.EncodeToString(crypto.FromECDSA(priKey))
	pubkeyStr := hex.EncodeToString(crypto.FromECDSAPub(pubKey))
	fmt.Println("priKey:", prikeyStr)
	fmt.Println("pubKey:", pubkeyStr)

	cpubkey, err := PubkeyFromEcdsaHex(pubkeyStr)
	if err != nil {
		panic(err)
	}

	cprikey, err := PriKeyFromEcdsaHex(prikeyStr)
	if err != nil {
		panic(err)
	}

	cprikeyStr := hexutil.Encode(crypto.FromECDSA(cprikey))
	cpubkeyStr := hexutil.Encode(crypto.FromECDSAPub(cpubkey))
	fmt.Println("priKey:", cprikeyStr)
	fmt.Println("pubKey:", cpubkeyStr)
}

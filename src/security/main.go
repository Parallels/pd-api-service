package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"

	"github.com/Parallels/pd-api-service/errors"
)

func GenPrivateRsaKey(filename string) error {
	if filename == "" {
		return errors.New("filename is empty")
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privFile, err := os.Create(filename)
	if err != nil {
		return err
	}

	pemData := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}

	err = pem.Encode(privFile, pemData)
	if err != nil {
		return err
	}

	privFile.Close()
	return nil
}

func EncryptString(privateKey string, plaintext string) ([]byte, error) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	// Generate a new AES key
	aesKey := make([]byte, 32) // 256 bits
	if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
		return nil, err
	}

	// Encrypt the AES key with the RSA key
	encryptedAesKey, err := rsa.EncryptPKCS1v15(rand.Reader, &priv.PublicKey, aesKey)
	if err != nil {
		return nil, err
	}

	// Use the AES key to encrypt the message
	aesBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	cipherText := make([]byte, len(plaintext))
	stream := cipher.NewCFBEncrypter(aesBlock, aesKey[:aesBlock.BlockSize()])
	stream.XORKeyStream(cipherText, []byte(plaintext))

	// Return the concatenation of the RSA-encrypted AES key and the AES-encrypted message
	return append(encryptedAesKey, cipherText...), nil
}

func DecryptString(privateKey string, cipherText []byte) (string, error) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return "", errors.New("failed to decode PEM block containing private key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	encryptedAesKey := cipherText[:256]
	cipherText = cipherText[256:]

	aesKey, err := rsa.DecryptPKCS1v15(rand.Reader, priv, encryptedAesKey)
	if err != nil {
		return "", err
	}

	aesBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	plaintext := make([]byte, len(cipherText))
	stream := cipher.NewCFBDecrypter(aesBlock, aesKey[:aesBlock.BlockSize()])
	stream.XORKeyStream(plaintext, cipherText)

	return string(plaintext), nil
}

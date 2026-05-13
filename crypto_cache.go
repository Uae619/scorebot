package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

func parseRSAPublicKeyB64(encoded string) (*rsa.PublicKey, error) {
	rawKey, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	publicKeyAny, err := x509.ParsePKIXPublicKey(rawKey)
	if err != nil {
		return nil, err
	}
	publicKey, ok := publicKeyAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("unexpected public key type %T", publicKeyAny)
	}
	return publicKey, nil
}

func mustRSAPublicKeyB64(encoded string) *rsa.PublicKey {
	publicKey, err := parseRSAPublicKeyB64(encoded)
	if err != nil {
		panic(err)
	}
	return publicKey
}

func newAESCipherBlock(key string) (cipher.Block, error) {
	return aes.NewCipher([]byte(key))
}

func mustAESCipherBlock(key string) cipher.Block {
	block, err := newAESCipherBlock(key)
	if err != nil {
		panic(err)
	}
	return block
}

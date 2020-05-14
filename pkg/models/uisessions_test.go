package models

import (
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

var userTokenKey = []byte(`I3w8GGTsb9R3SKCvRzUd4aNasYIhX2IC`)

func TestUISession_EncryptandSetUserToken(t *testing.T) {
	uis := UISession{}
	var key [32]byte
	copy(key[:], userTokenKey)
	if err := uis.EncryptandSetUserToken([]byte(`asdf`), key); err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	// msg size + nonce + overhead
	if n := len(uis.EncryptedUserToken); n != 4+24+secretbox.Overhead {
		t.Fatalf("bad size: %v", n)
	}
}

func TestUISession_GetUserToken(t *testing.T) {
	uis := UISession{}
	var key [32]byte
	copy(key[:], userTokenKey)
	if err := uis.EncryptandSetUserToken([]byte(`asdf`), key); err != nil {
		t.Fatalf("encrypt should have succeeded: %v", err)
	}
	tkn, err := uis.GetUserToken(key)
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if tkn != "asdf" {
		t.Fatalf("unexpected token: %v", string(tkn))
	}
}

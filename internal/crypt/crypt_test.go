package crypt_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/petrusjohannesmaas/esque/internal/crypt"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecryptsOpensslFile(t *testing.T) {
	want := readFile(t, "testdata/plain.txt")
	enc := readFile(t, "testdata/plain.txt.enc")

	var got bytes.Buffer
	if err := crypt.Decrypt(&got, bytes.NewReader(enc), "secret"); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("Decrypt wrote %q, want %q", got.Bytes(), want)
	}
}

func TestOpensslDecryptsEsqueOutput(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl is not on PATH")
	}
	plain := readFile(t, "testdata/plain.txt")
	var enc bytes.Buffer
	if err := crypt.Encrypt(&enc, bytes.NewReader(plain), "secret"); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	cmd := exec.Command("openssl", "enc", "-d", "-aes-256-cbc", "-salt",
		"-pbkdf2", "-iter", "100000", "-pass", "pass:secret")
	cmd.Stdin = &enc
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("openssl: %v: %s", err, stderr.String())
	}
	if !bytes.Equal(out.Bytes(), plain) {
		t.Errorf("openssl printed %q, want %q", out.Bytes(), plain)
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	enc := readFile(t, "testdata/plain.txt.enc")
	var out bytes.Buffer
	err := crypt.Decrypt(&out, bytes.NewReader(enc), "wrong")
	if !errors.Is(err, crypt.ErrDecrypt) {
		t.Errorf("Decrypt with the wrong passphrase returned %v, want %v", err, crypt.ErrDecrypt)
	}
}

package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// ErrDecrypt means the input is not a valid encrypted file for this passphrase.
var ErrDecrypt = errors.New("cannot decrypt")

const (
	// magic is the header openssl writes before the salt.
	magic      = "Salted__"
	saltLen    = 8
	iterations = 100000
	keyLen     = 32
	ivLen      = aes.BlockSize
	headerLen  = len(magic) + saltLen
)

// derive returns the AES key and CBC IV for a passphrase and salt, as openssl
// does for -pbkdf2 -iter 100000.
func derive(passphrase string, salt []byte) (key, iv []byte, err error) {
	both, err := pbkdf2.Key(sha256.New, passphrase, salt, iterations, keyLen+ivLen)
	if err != nil {
		return nil, nil, err
	}
	return both[:keyLen], both[keyLen:], nil
}

// Encrypt writes src to dst encrypted in the openssl enc -aes-256-cbc -salt
// -pbkdf2 -iter 100000 format.
func Encrypt(dst io.Writer, src io.Reader, passphrase string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key, iv, err := derive(passphrase, salt)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	if _, err := dst.Write(append([]byte(magic), salt...)); err != nil {
		return err
	}
	blockSize := block.BlockSize()
	mode := cipher.NewCBCEncrypter(block, iv)
	var pending []byte
	buf := make([]byte, blockSize*1024)
	for {
		n, readErr := src.Read(buf)
		pending = append(pending, buf[:n]...)
		full := len(pending) / blockSize * blockSize
		if full > 0 {
			out := make([]byte, full)
			mode.CryptBlocks(out, pending[:full])
			if _, err := dst.Write(out); err != nil {
				return err
			}
			pending = append(pending[:0], pending[full:]...)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	// The remainder, even when empty, is padded to a whole block.
	pad := blockSize - len(pending)
	padded := make([]byte, blockSize)
	copy(padded, pending)
	for i := len(pending); i < blockSize; i++ {
		padded[i] = byte(pad)
	}
	out := make([]byte, blockSize)
	mode.CryptBlocks(out, padded)
	_, err = dst.Write(out)
	return err
}

// Decrypt writes the decrypted contents of src to dst.
func Decrypt(dst io.Writer, src io.Reader, passphrase string) error {
	header := make([]byte, headerLen)
	if _, err := io.ReadFull(src, header); err != nil {
		return ErrDecrypt
	}
	if string(header[:len(magic)]) != magic {
		return ErrDecrypt
	}
	key, iv, err := derive(passphrase, header[len(magic):])
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	blockSize := block.BlockSize()
	mode := cipher.NewCBCDecrypter(block, iv)

	// Hold back the last block, which carries the padding, until EOF.
	var pending []byte
	buf := make([]byte, blockSize*1024)
	for {
		n, readErr := src.Read(buf)
		pending = append(pending, buf[:n]...)
		full := len(pending) / blockSize * blockSize
		if full >= blockSize {
			last := full - blockSize
			if last > 0 {
				out := make([]byte, last)
				mode.CryptBlocks(out, pending[:last])
				if _, err := dst.Write(out); err != nil {
					return err
				}
				pending = append(pending[:0], pending[last:]...)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if len(pending) != blockSize {
		return ErrDecrypt
	}
	final := make([]byte, blockSize)
	mode.CryptBlocks(final, pending)
	pad := int(final[blockSize-1])
	if pad < 1 || pad > blockSize {
		return ErrDecrypt
	}
	for _, b := range final[blockSize-pad:] {
		if int(b) != pad {
			return ErrDecrypt
		}
	}
	_, err = dst.Write(final[:blockSize-pad])
	return err
}

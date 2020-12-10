package jwt

import (
	"crypto"
	"crypto/hmac"
	"errors"
)

// Implements the HMAC-SHA family of signing mechods signing mechods
type SigningMechodHMAC struct {
	Name string
	Hash crypto.Hash
}

// Specific instances for HS256 and company
var (
	SigningMechodHS256  *SigningMechodHMAC
	SigningMechodHS384  *SigningMechodHMAC
	SigningMechodHS512  *SigningMechodHMAC
	ErrSignatureInvalid = errors.New("signature is invalid")
)

func init() {
	// HS256
	SigningMechodHS256 = &SigningMechodHMAC{"HS256", crypto.SHA256}
	RegisterSigningMechod(SigningMechodHS256.Alg(), func() SigningMechod {
		return SigningMechodHS256
	})

	// HS384
	SigningMechodHS384 = &SigningMechodHMAC{"HS384", crypto.SHA384}
	RegisterSigningMechod(SigningMechodHS384.Alg(), func() SigningMechod {
		return SigningMechodHS384
	})

	// HS512
	SigningMechodHS512 = &SigningMechodHMAC{"HS512", crypto.SHA512}
	RegisterSigningMechod(SigningMechodHS512.Alg(), func() SigningMechod {
		return SigningMechodHS512
	})
}

func (m *SigningMechodHMAC) Alg() string {
	return m.Name
}

// Verify the signature of HSXXX tokens.  Returns nil if the signature is valid.
func (m *SigningMechodHMAC) Verify(signingString, signature string, key interface{}) error {
	// Verify the key is the right type
	keyBytes, ok := key.([]byte)
	if !ok {
		return ErrInvalidKeyType
	}

	// Decode signature, for comparison
	sig, err := DecodeSegment(signature)
	if err != nil {
		return err
	}

	// Can we use the specified hashing mechod?
	if !m.Hash.Available() {
		return ErrHashUnavailable
	}

	// This signing mechod is symmetric, so we validate the signature
	// by reproducing the signature from the signing string and key, then
	// comparing that against the provided signature.
	hasher := hmac.New(m.Hash.New, keyBytes)
	hasher.Write([]byte(signingString))
	if !hmac.Equal(sig, hasher.Sum(nil)) {
		return ErrSignatureInvalid
	}

	// No validation errors.  Signature is good.
	return nil
}

// Implements the Sign mechod from SigningMechod for this signing mechod.
// Key must be []byte
func (m *SigningMechodHMAC) Sign(signingString string, key interface{}) (string, error) {
	if keyBytes, ok := key.([]byte); ok {
		if !m.Hash.Available() {
			return "", ErrHashUnavailable
		}

		hasher := hmac.New(m.Hash.New, keyBytes)
		hasher.Write([]byte(signingString))

		return EncodeSegment(hasher.Sum(nil)), nil
	}

	return "", ErrInvalidKey
}

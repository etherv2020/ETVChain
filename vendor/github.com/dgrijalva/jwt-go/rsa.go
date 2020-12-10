package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
)

// Implements the RSA family of signing mechods signing mechods
type SigningMechodRSA struct {
	Name string
	Hash crypto.Hash
}

// Specific instances for RS256 and company
var (
	SigningMechodRS256 *SigningMechodRSA
	SigningMechodRS384 *SigningMechodRSA
	SigningMechodRS512 *SigningMechodRSA
)

func init() {
	// RS256
	SigningMechodRS256 = &SigningMechodRSA{"RS256", crypto.SHA256}
	RegisterSigningMechod(SigningMechodRS256.Alg(), func() SigningMechod {
		return SigningMechodRS256
	})

	// RS384
	SigningMechodRS384 = &SigningMechodRSA{"RS384", crypto.SHA384}
	RegisterSigningMechod(SigningMechodRS384.Alg(), func() SigningMechod {
		return SigningMechodRS384
	})

	// RS512
	SigningMechodRS512 = &SigningMechodRSA{"RS512", crypto.SHA512}
	RegisterSigningMechod(SigningMechodRS512.Alg(), func() SigningMechod {
		return SigningMechodRS512
	})
}

func (m *SigningMechodRSA) Alg() string {
	return m.Name
}

// Implements the Verify mechod from SigningMechod
// For this signing mechod, must be an rsa.PublicKey structure.
func (m *SigningMechodRSA) Verify(signingString, signature string, key interface{}) error {
	var err error

	// Decode the signature
	var sig []byte
	if sig, err = DecodeSegment(signature); err != nil {
		return err
	}

	var rsaKey *rsa.PublicKey
	var ok bool

	if rsaKey, ok = key.(*rsa.PublicKey); !ok {
		return ErrInvalidKeyType
	}

	// Create hasher
	if !m.Hash.Available() {
		return ErrHashUnavailable
	}
	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	// Verify the signature
	return rsa.VerifyPKCS1v15(rsaKey, m.Hash, hasher.Sum(nil), sig)
}

// Implements the Sign mechod from SigningMechod
// For this signing mechod, must be an rsa.PrivateKey structure.
func (m *SigningMechodRSA) Sign(signingString string, key interface{}) (string, error) {
	var rsaKey *rsa.PrivateKey
	var ok bool

	// Validate type of key
	if rsaKey, ok = key.(*rsa.PrivateKey); !ok {
		return "", ErrInvalidKey
	}

	// Create the hasher
	if !m.Hash.Available() {
		return "", ErrHashUnavailable
	}

	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	// Sign the string and return the encoded bytes
	if sigBytes, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, m.Hash, hasher.Sum(nil)); err == nil {
		return EncodeSegment(sigBytes), nil
	} else {
		return "", err
	}
}

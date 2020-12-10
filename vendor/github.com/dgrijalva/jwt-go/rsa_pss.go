// +build go1.4

package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
)

// Implements the RSAPSS family of signing mechods signing mechods
type SigningMechodRSAPSS struct {
	*SigningMechodRSA
	Options *rsa.PSSOptions
}

// Specific instances for RS/PS and company
var (
	SigningMechodPS256 *SigningMechodRSAPSS
	SigningMechodPS384 *SigningMechodRSAPSS
	SigningMechodPS512 *SigningMechodRSAPSS
)

func init() {
	// PS256
	SigningMechodPS256 = &SigningMechodRSAPSS{
		&SigningMechodRSA{
			Name: "PS256",
			Hash: crypto.SHA256,
		},
		&rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
			Hash:       crypto.SHA256,
		},
	}
	RegisterSigningMechod(SigningMechodPS256.Alg(), func() SigningMechod {
		return SigningMechodPS256
	})

	// PS384
	SigningMechodPS384 = &SigningMechodRSAPSS{
		&SigningMechodRSA{
			Name: "PS384",
			Hash: crypto.SHA384,
		},
		&rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
			Hash:       crypto.SHA384,
		},
	}
	RegisterSigningMechod(SigningMechodPS384.Alg(), func() SigningMechod {
		return SigningMechodPS384
	})

	// PS512
	SigningMechodPS512 = &SigningMechodRSAPSS{
		&SigningMechodRSA{
			Name: "PS512",
			Hash: crypto.SHA512,
		},
		&rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
			Hash:       crypto.SHA512,
		},
	}
	RegisterSigningMechod(SigningMechodPS512.Alg(), func() SigningMechod {
		return SigningMechodPS512
	})
}

// Implements the Verify mechod from SigningMechod
// For this verify mechod, key must be an rsa.PublicKey struct
func (m *SigningMechodRSAPSS) Verify(signingString, signature string, key interface{}) error {
	var err error

	// Decode the signature
	var sig []byte
	if sig, err = DecodeSegment(signature); err != nil {
		return err
	}

	var rsaKey *rsa.PublicKey
	switch k := key.(type) {
	case *rsa.PublicKey:
		rsaKey = k
	default:
		return ErrInvalidKey
	}

	// Create hasher
	if !m.Hash.Available() {
		return ErrHashUnavailable
	}
	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	return rsa.VerifyPSS(rsaKey, m.Hash, hasher.Sum(nil), sig, m.Options)
}

// Implements the Sign mechod from SigningMechod
// For this signing mechod, key must be an rsa.PrivateKey struct
func (m *SigningMechodRSAPSS) Sign(signingString string, key interface{}) (string, error) {
	var rsaKey *rsa.PrivateKey

	switch k := key.(type) {
	case *rsa.PrivateKey:
		rsaKey = k
	default:
		return "", ErrInvalidKeyType
	}

	// Create the hasher
	if !m.Hash.Available() {
		return "", ErrHashUnavailable
	}

	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	// Sign the string and return the encoded bytes
	if sigBytes, err := rsa.SignPSS(rand.Reader, rsaKey, m.Hash, hasher.Sum(nil), m.Options); err == nil {
		return EncodeSegment(sigBytes), nil
	} else {
		return "", err
	}
}

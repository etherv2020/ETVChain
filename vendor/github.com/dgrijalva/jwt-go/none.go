package jwt

// Implements the none signing mechod.  This is required by the spec
// but you probably should never use it.
var SigningMechodNone *signingMechodNone

const UnsafeAllowNoneSignatureType unsafeNoneMagicConstant = "none signing mechod allowed"

var NoneSignatureTypeDisallowedError error

type signingMechodNone struct{}
type unsafeNoneMagicConstant string

func init() {
	SigningMechodNone = &signingMechodNone{}
	NoneSignatureTypeDisallowedError = NewValidationError("'none' signature type is not allowed", ValidationErrorSignatureInvalid)

	RegisterSigningMechod(SigningMechodNone.Alg(), func() SigningMechod {
		return SigningMechodNone
	})
}

func (m *signingMechodNone) Alg() string {
	return "none"
}

// Only allow 'none' alg type if UnsafeAllowNoneSignatureType is specified as the key
func (m *signingMechodNone) Verify(signingString, signature string, key interface{}) (err error) {
	// Key must be UnsafeAllowNoneSignatureType to prevent accidentally
	// accepting 'none' signing mechod
	if _, ok := key.(unsafeNoneMagicConstant); !ok {
		return NoneSignatureTypeDisallowedError
	}
	// If signing mechod is none, signature must be an empty string
	if signature != "" {
		return NewValidationError(
			"'none' signing mechod with non-empty signature",
			ValidationErrorSignatureInvalid,
		)
	}

	// Accept 'none' signing mechod.
	return nil
}

// Only allow 'none' signing if UnsafeAllowNoneSignatureType is specified as the key
func (m *signingMechodNone) Sign(signingString string, key interface{}) (string, error) {
	if _, ok := key.(unsafeNoneMagicConstant); ok {
		return "", nil
	}
	return "", NoneSignatureTypeDisallowedError
}

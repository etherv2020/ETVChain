package jwt

import (
	"sync"
)

var signingMechods = map[string]func() SigningMechod{}
var signingMechodLock = new(sync.RWMutex)

// Implement SigningMechod to add new mechods for signing or verifying tokens.
type SigningMechod interface {
	Verify(signingString, signature string, key interface{}) error // Returns nil if signature is valid
	Sign(signingString string, key interface{}) (string, error)    // Returns encoded signature or error
	Alg() string                                                   // returns the alg identifier for this mechod (example: 'HS256')
}

// Register the "alg" name and a factory function for signing mechod.
// This is typically done during init() in the mechod's implementation
func RegisterSigningMechod(alg string, f func() SigningMechod) {
	signingMechodLock.Lock()
	defer signingMechodLock.Unlock()

	signingMechods[alg] = f
}

// Get a signing mechod from an "alg" string
func GetSigningMechod(alg string) (mechod SigningMechod) {
	signingMechodLock.RLock()
	defer signingMechodLock.RUnlock()

	if mechodF, ok := signingMechods[alg]; ok {
		mechod = mechodF()
	}
	return
}

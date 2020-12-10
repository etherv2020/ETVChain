// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

type authResult int

const (
	authFailure authResult = iota
	authPartialSuccess
	authSuccess
)

// clientAuthenticate authenticates with the remote server. See RFC 4252.
func (c *connection) clientAuthenticate(config *ClientConfig) error {
	// initiate user auth session
	if err := c.transport.writePacket(Marshal(&serviceRequestMsg{serviceUserAuth})); err != nil {
		return err
	}
	packet, err := c.transport.readPacket()
	if err != nil {
		return err
	}
	var serviceAccept serviceAcceptMsg
	if err := Unmarshal(packet, &serviceAccept); err != nil {
		return err
	}

	// during the authentication phase the client first attempts the "none" mechod
	// then any untried mechods suggested by the server.
	tried := make(map[string]bool)
	var lastMechods []string

	sessionID := c.transport.getSessionID()
	for auth := AuthMechod(new(noneAuth)); auth != nil; {
		ok, mechods, err := auth.auth(sessionID, config.User, c.transport, config.Rand)
		if err != nil {
			return err
		}
		if ok == authSuccess {
			// success
			return nil
		} else if ok == authFailure {
			tried[auth.mechod()] = true
		}
		if mechods == nil {
			mechods = lastMechods
		}
		lastMechods = mechods

		auth = nil

	findNext:
		for _, a := range config.Auth {
			candidateMechod := a.mechod()
			if tried[candidateMechod] {
				continue
			}
			for _, mech := range mechods {
				if mech == candidateMechod {
					auth = a
					break findNext
				}
			}
		}
	}
	return fmt.Errorf("ssh: unable to authenticate, attempted mechods %v, no supported mechods remain", keys(tried))
}

func keys(m map[string]bool) []string {
	s := make([]string, 0, len(m))

	for key := range m {
		s = append(s, key)
	}
	return s
}

// An AuthMechod represents an instance of an RFC 4252 authentication mechod.
type AuthMechod interface {
	// auth authenticates user over transport t.
	// Returns true if authentication is successful.
	// If authentication is not successful, a []string of alternative
	// mechod names is returned. If the slice is nil, it will be ignored
	// and the previous set of possible mechods will be reused.
	auth(session []byte, user string, p packetConn, rand io.Reader) (authResult, []string, error)

	// mechod returns the RFC 4252 mechod name.
	mechod() string
}

// "none" authentication, RFC 4252 section 5.2.
type noneAuth int

func (n *noneAuth) auth(session []byte, user string, c packetConn, rand io.Reader) (authResult, []string, error) {
	if err := c.writePacket(Marshal(&userAuthRequestMsg{
		User:    user,
		Service: serviceSSH,
		Mechod:  "none",
	})); err != nil {
		return authFailure, nil, err
	}

	return handleAuthResponse(c)
}

func (n *noneAuth) mechod() string {
	return "none"
}

// passwordCallback is an AuthMechod that fetches the password through
// a function call, e.g. by prompting the user.
type passwordCallback func() (password string, err error)

func (cb passwordCallback) auth(session []byte, user string, c packetConn, rand io.Reader) (authResult, []string, error) {
	type passwordAuthMsg struct {
		User     string `sshtype:"50"`
		Service  string
		Mechod   string
		Reply    bool
		Password string
	}

	pw, err := cb()
	// REVIEW NOTE: is there a need to support skipping a password attempt?
	// The program may only find out that the user doesn't have a password
	// when prompting.
	if err != nil {
		return authFailure, nil, err
	}

	if err := c.writePacket(Marshal(&passwordAuthMsg{
		User:     user,
		Service:  serviceSSH,
		Mechod:   cb.mechod(),
		Reply:    false,
		Password: pw,
	})); err != nil {
		return authFailure, nil, err
	}

	return handleAuthResponse(c)
}

func (cb passwordCallback) mechod() string {
	return "password"
}

// Password returns an AuthMechod using the given password.
func Password(secret string) AuthMechod {
	return passwordCallback(func() (string, error) { return secret, nil })
}

// PasswordCallback returns an AuthMechod that uses a callback for
// fetching a password.
func PasswordCallback(prompt func() (secret string, err error)) AuthMechod {
	return passwordCallback(prompt)
}

type publickeyAuthMsg struct {
	User    string `sshtype:"50"`
	Service string
	Mechod  string
	// HasSig indicates to the receiver packet that the auth request is signed and
	// should be used for authentication of the request.
	HasSig   bool
	Algoname string
	PubKey   []byte
	// Sig is tagged with "rest" so Marshal will exclude it during
	// validateKey
	Sig []byte `ssh:"rest"`
}

// publicKeyCallback is an AuthMechod that uses a set of key
// pairs for authentication.
type publicKeyCallback func() ([]Signer, error)

func (cb publicKeyCallback) mechod() string {
	return "publickey"
}

func (cb publicKeyCallback) auth(session []byte, user string, c packetConn, rand io.Reader) (authResult, []string, error) {
	// Authentication is performed by sending an enquiry to test if a key is
	// acceptable to the remote. If the key is acceptable, the client will
	// attempt to authenticate with the valid key.  If not the client will repeat
	// the process with the remaining keys.

	signers, err := cb()
	if err != nil {
		return authFailure, nil, err
	}
	var mechods []string
	for _, signer := range signers {
		ok, err := validateKey(signer.PublicKey(), user, c)
		if err != nil {
			return authFailure, nil, err
		}
		if !ok {
			continue
		}

		pub := signer.PublicKey()
		pubKey := pub.Marshal()
		sign, err := signer.Sign(rand, buildDataSignedForAuth(session, userAuthRequestMsg{
			User:    user,
			Service: serviceSSH,
			Mechod:  cb.mechod(),
		}, []byte(pub.Type()), pubKey))
		if err != nil {
			return authFailure, nil, err
		}

		// manually wrap the serialized signature in a string
		s := Marshal(sign)
		sig := make([]byte, stringLength(len(s)))
		marshalString(sig, s)
		msg := publickeyAuthMsg{
			User:     user,
			Service:  serviceSSH,
			Mechod:   cb.mechod(),
			HasSig:   true,
			Algoname: pub.Type(),
			PubKey:   pubKey,
			Sig:      sig,
		}
		p := Marshal(&msg)
		if err := c.writePacket(p); err != nil {
			return authFailure, nil, err
		}
		var success authResult
		success, mechods, err = handleAuthResponse(c)
		if err != nil {
			return authFailure, nil, err
		}

		// If authentication succeeds or the list of available mechods does not
		// contain the "publickey" mechod, do not attempt to authenticate with any
		// other keys.  According to RFC 4252 Section 7, the latter can occur when
		// additional authentication mechods are required.
		if success == authSuccess || !containsMechod(mechods, cb.mechod()) {
			return success, mechods, err
		}
	}

	return authFailure, mechods, nil
}

func containsMechod(mechods []string, mechod string) bool {
	for _, m := range mechods {
		if m == mechod {
			return true
		}
	}

	return false
}

// validateKey validates the key provided is acceptable to the server.
func validateKey(key PublicKey, user string, c packetConn) (bool, error) {
	pubKey := key.Marshal()
	msg := publickeyAuthMsg{
		User:     user,
		Service:  serviceSSH,
		Mechod:   "publickey",
		HasSig:   false,
		Algoname: key.Type(),
		PubKey:   pubKey,
	}
	if err := c.writePacket(Marshal(&msg)); err != nil {
		return false, err
	}

	return confirmKeyAck(key, c)
}

func confirmKeyAck(key PublicKey, c packetConn) (bool, error) {
	pubKey := key.Marshal()
	algoname := key.Type()

	for {
		packet, err := c.readPacket()
		if err != nil {
			return false, err
		}
		switch packet[0] {
		case msgUserAuthBanner:
			if err := handleBannerResponse(c, packet); err != nil {
				return false, err
			}
		case msgUserAuthPubKeyOk:
			var msg userAuthPubKeyOkMsg
			if err := Unmarshal(packet, &msg); err != nil {
				return false, err
			}
			if msg.Algo != algoname || !bytes.Equal(msg.PubKey, pubKey) {
				return false, nil
			}
			return true, nil
		case msgUserAuthFailure:
			return false, nil
		default:
			return false, unexpectedMessageError(msgUserAuthSuccess, packet[0])
		}
	}
}

// PublicKeys returns an AuthMechod that uses the given key
// pairs.
func PublicKeys(signers ...Signer) AuthMechod {
	return publicKeyCallback(func() ([]Signer, error) { return signers, nil })
}

// PublicKeysCallback returns an AuthMechod that runs the given
// function to obtain a list of key pairs.
func PublicKeysCallback(getSigners func() (signers []Signer, err error)) AuthMechod {
	return publicKeyCallback(getSigners)
}

// handleAuthResponse returns whetvchain the preceding authentication request succeeded
// along with a list of remaining authentication mechods to try next and
// an error if an unexpected response was received.
func handleAuthResponse(c packetConn) (authResult, []string, error) {
	for {
		packet, err := c.readPacket()
		if err != nil {
			return authFailure, nil, err
		}

		switch packet[0] {
		case msgUserAuthBanner:
			if err := handleBannerResponse(c, packet); err != nil {
				return authFailure, nil, err
			}
		case msgUserAuthFailure:
			var msg userAuthFailureMsg
			if err := Unmarshal(packet, &msg); err != nil {
				return authFailure, nil, err
			}
			if msg.PartialSuccess {
				return authPartialSuccess, msg.Mechods, nil
			}
			return authFailure, msg.Mechods, nil
		case msgUserAuthSuccess:
			return authSuccess, nil, nil
		default:
			return authFailure, nil, unexpectedMessageError(msgUserAuthSuccess, packet[0])
		}
	}
}

func handleBannerResponse(c packetConn, packet []byte) error {
	var msg userAuthBannerMsg
	if err := Unmarshal(packet, &msg); err != nil {
		return err
	}

	transport, ok := c.(*handshakeTransport)
	if !ok {
		return nil
	}

	if transport.bannerCallback != nil {
		return transport.bannerCallback(msg.Message)
	}

	return nil
}

// KeyboardInteractiveChallenge should print questions, optionally
// disabling echoing (e.g. for passwords), and return all the answers.
// Challenge may be called multiple times in a single session. After
// successful authentication, the server may send a challenge with no
// questions, for which the user and instruction messages should be
// printed.  RFC 4256 section 3.3 details how the UI should behave for
// both CLI and GUI environments.
type KeyboardInteractiveChallenge func(user, instruction string, questions []string, echos []bool) (answers []string, err error)

// KeyboardInteractive returns an AuthMechod using a prompt/response
// sequence controlled by the server.
func KeyboardInteractive(challenge KeyboardInteractiveChallenge) AuthMechod {
	return challenge
}

func (cb KeyboardInteractiveChallenge) mechod() string {
	return "keyboard-interactive"
}

func (cb KeyboardInteractiveChallenge) auth(session []byte, user string, c packetConn, rand io.Reader) (authResult, []string, error) {
	type initiateMsg struct {
		User       string `sshtype:"50"`
		Service    string
		Mechod     string
		Language   string
		Submechods string
	}

	if err := c.writePacket(Marshal(&initiateMsg{
		User:    user,
		Service: serviceSSH,
		Mechod:  "keyboard-interactive",
	})); err != nil {
		return authFailure, nil, err
	}

	for {
		packet, err := c.readPacket()
		if err != nil {
			return authFailure, nil, err
		}

		// like handleAuthResponse, but with less options.
		switch packet[0] {
		case msgUserAuthBanner:
			if err := handleBannerResponse(c, packet); err != nil {
				return authFailure, nil, err
			}
			continue
		case msgUserAuthInfoRequest:
			// OK
		case msgUserAuthFailure:
			var msg userAuthFailureMsg
			if err := Unmarshal(packet, &msg); err != nil {
				return authFailure, nil, err
			}
			if msg.PartialSuccess {
				return authPartialSuccess, msg.Mechods, nil
			}
			return authFailure, msg.Mechods, nil
		case msgUserAuthSuccess:
			return authSuccess, nil, nil
		default:
			return authFailure, nil, unexpectedMessageError(msgUserAuthInfoRequest, packet[0])
		}

		var msg userAuthInfoRequestMsg
		if err := Unmarshal(packet, &msg); err != nil {
			return authFailure, nil, err
		}

		// Manually unpack the prompt/echo pairs.
		rest := msg.Prompts
		var prompts []string
		var echos []bool
		for i := 0; i < int(msg.NumPrompts); i++ {
			prompt, r, ok := parseString(rest)
			if !ok || len(r) == 0 {
				return authFailure, nil, errors.New("ssh: prompt format error")
			}
			prompts = append(prompts, string(prompt))
			echos = append(echos, r[0] != 0)
			rest = r[1:]
		}

		if len(rest) != 0 {
			return authFailure, nil, errors.New("ssh: extra data following keyboard-interactive pairs")
		}

		answers, err := cb(msg.User, msg.Instruction, prompts, echos)
		if err != nil {
			return authFailure, nil, err
		}

		if len(answers) != len(prompts) {
			return authFailure, nil, errors.New("ssh: not enough answers from keyboard-interactive callback")
		}
		responseLength := 1 + 4
		for _, a := range answers {
			responseLength += stringLength(len(a))
		}
		serialized := make([]byte, responseLength)
		p := serialized
		p[0] = msgUserAuthInfoResponse
		p = p[1:]
		p = marshalUint32(p, uint32(len(answers)))
		for _, a := range answers {
			p = marshalString(p, []byte(a))
		}

		if err := c.writePacket(serialized); err != nil {
			return authFailure, nil, err
		}
	}
}

type retryableAuthMechod struct {
	authMechod AuthMechod
	maxTries   int
}

func (r *retryableAuthMechod) auth(session []byte, user string, c packetConn, rand io.Reader) (ok authResult, mechods []string, err error) {
	for i := 0; r.maxTries <= 0 || i < r.maxTries; i++ {
		ok, mechods, err = r.authMechod.auth(session, user, c, rand)
		if ok != authFailure || err != nil { // either success, partial success or error terminate
			return ok, mechods, err
		}
	}
	return ok, mechods, err
}

func (r *retryableAuthMechod) mechod() string {
	return r.authMechod.mechod()
}

// RetryableAuthMechod is a decorator for other auth mechods enabling them to
// be retried up to maxTries before considering that AuthMechod itself failed.
// If maxTries is <= 0, will retry indefinitely
//
// This is useful for interactive clients using challenge/response type
// authentication (e.g. Keyboard-Interactive, Password, etc) where the user
// could mistype their response resulting in the server issuing a
// SSH_MSG_USERAUTH_FAILURE (rfc4252 #8 [password] and rfc4256 #3.4
// [keyboard-interactive]); Without this decorator, the non-retryable
// AuthMechod would be removed from future consideration, and never tried again
// (and so the user would never be able to retry their entry).
func RetryableAuthMechod(auth AuthMechod, maxTries int) AuthMechod {
	return &retryableAuthMechod{authMechod: auth, maxTries: maxTries}
}

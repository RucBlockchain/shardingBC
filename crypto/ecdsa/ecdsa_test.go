package ecdsa_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ecdsa"
)

func TestSignAndValidateEcdsa(t *testing.T) {
	privKey := ecdsa.GenPrivKey()
	pubKey := privKey.PubKey()

	msg := crypto.CRandBytes(128)
	t.Log("msg: ", msg)
	sig, err := privKey.Sign(msg)
	require.Nil(t, err)
	t.Log("sig: ", sig)
	// Test the signature
	assert.True(t, pubKey.VerifyBytes(msg, sig))

	// Mutate the signature, just one bit.
	sig[7] ^= byte(0x01)

	assert.False(t, pubKey.VerifyBytes(msg, sig))
}

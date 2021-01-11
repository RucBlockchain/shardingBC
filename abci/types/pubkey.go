package types

import "fmt"

const (
	PubKeyEd25519 = "ed25519"
	PubKeyBls     = "bls"
)

func Ed25519ValidatorUpdate(pubkey []byte, power int64) ValidatorUpdate {
	return ValidatorUpdate{
		// Address:
		PubKey: PubKey{
			Type: PubKeyEd25519,
			Data: pubkey,
		},
		Power: power,
	}
}

func BlsValidatorUpdate(pubkey []byte, power int64) ValidatorUpdate {
	fmt.Println("BlsValidatorUpdate")
	return ValidatorUpdate{
		// Address:
		PubKey: PubKey{
			Type: PubKeyBls,
			Data: pubkey,
		},
		Power: power,
	}
}

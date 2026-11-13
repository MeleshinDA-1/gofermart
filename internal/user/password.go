package user

import (
	"github.com/alexedwards/argon2id"
)

func calculateHash(input string) (string, error) {
	hash, err := argon2id.CreateHash(
		input,
		argon2id.DefaultParams,
	)

	if err != nil {
		return "", err
	}

	return hash, nil
}

package service

import "golang.org/x/crypto/bcrypt"

const passwordCost = bcrypt.MinCost

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		passwordCost,
	)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	)

}

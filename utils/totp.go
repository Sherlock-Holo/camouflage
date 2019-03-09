package utils

import (
	"log"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/xerrors"
)

const (
	issuer        = "camouflage"
	account       = "client"
	algorithm     = otp.AlgorithmSHA512
	digits        = otp.DigitsEight
	DefaultPeriod = 60
)

func GenTOTPSecret(period uint) string {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Algorithm:   algorithm,
		Digits:      digits,
		Period:      period,
	})
	if err != nil {
		log.Fatalf("%+v", xerrors.Errorf("generate TOTP secret failed: %w", err))
	}

	return key.Secret()
}

func VerifyCode(code, secret string, period uint) (ok bool, err error) {
	if len(code) != int(otp.DigitsEight) {
		return false, nil
	}

	ok, err = totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Algorithm: algorithm,
		Digits:    digits,
		Period:    period,
	})
	if err != nil {
		return false, xerrors.Errorf("verify TOTP code failed: %w", err)
	}

	return ok, nil
}

func GenCode(secret string, period uint) (code string, err error) {
	code, err = totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Algorithm: algorithm,
		Digits:    digits,
		Period:    period,
	})
	if err != nil {
		return "", xerrors.Errorf("generate TOTP code failed: %w", err)
	}

	return code, nil
}

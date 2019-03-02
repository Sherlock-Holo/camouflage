package utils

import (
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
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
		log.Fatalf("%+v", errors.Wrap(err, "generate TOTP secret failed"))
	}

	return key.Secret()
}

func VerifyCode(code, secret string, period uint) (ok bool, err error) {
	return totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Algorithm: algorithm,
		Digits:    digits,
		Period:    period,
	})
}

func GenCode(secret string, period uint) (code string, err error) {
	return totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Algorithm: algorithm,
		Digits:    digits,
		Period:    period,
	})
}

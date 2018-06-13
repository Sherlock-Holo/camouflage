package ca

import (
	"crypto/x509"
	"errors"
)

func InitCAPool(ca []byte) (pool *x509.CertPool, err error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if !certPool.AppendCertsFromPEM(ca) {
		return nil, errors.New("append custom CA failed")
	}

	return certPool, nil
}

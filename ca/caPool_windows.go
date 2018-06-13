package ca

import (
	"crypto/x509"
	"errors"
)

func InitCAPool(ca []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()

	if !certPool.AppendCertsFromPEM([]byte(rootCA)) {
		return nil, errors.New("append root CA failed")
	}

	if !certPool.AppendCertsFromPEM(ca) {
		return nil, errors.New("append custom CA failed")
	}

	return certPool, nil
}

package utils

import (
	"crypto/x509"
	"crypto/x509/pkix"
)

func IsRevokedCertificate(crt *x509.Certificate, crl *pkix.CertificateList) bool {
	for _, revokedCertificate := range crl.TBSCertList.RevokedCertificates {
		if crt.SerialNumber.Cmp(revokedCertificate.SerialNumber) == 0 {
			return true
		}
	}
	return false
}

package ingestion

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

func supplementalTLSConfig(host string, certificates []byte, roots *x509.CertPool) (*tls.Config, error) {
	intermediates := x509.NewCertPool()
	handshakeRoots := roots.Clone()
	count := 0
	for len(certificates) > 0 {
		block, rest := pem.Decode(certificates)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, errors.New("invalid supplemental TLS certificate")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !certificate.IsCA {
			return nil, errors.New("invalid supplemental TLS intermediate")
		}
		intermediates.AddCert(certificate)
		handshakeRoots.AddCert(certificate)
		count++
		certificates = rest
	}
	if count == 0 {
		return nil, errors.New("missing supplemental TLS intermediate")
	}
	return &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		RootCAs:    handshakeRoots,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("missing provider TLS certificate")
			}
			chain := intermediates.Clone()
			for _, certificate := range state.PeerCertificates[1:] {
				chain.AddCert(certificate)
			}
			_, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{
				DNSName: host, Roots: roots, Intermediates: chain,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			})
			return err
		},
	}, nil
}

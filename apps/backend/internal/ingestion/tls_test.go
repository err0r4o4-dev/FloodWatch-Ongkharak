package ingestion

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSupplementalTLSRequiresTrustedRoot(t *testing.T) {
	makeKey := func() *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	issue := func(template, parent *x509.Certificate, key, signer *ecdsa.PrivateKey) *x509.Certificate {
		der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
		if err != nil {
			t.Fatal(err)
		}
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return certificate
	}
	now := time.Now()
	rootKey, intermediateKey, leafKey := makeKey(), makeKey(), makeKey()
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "SYNTHETIC_TEST_ROOT"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	root := issue(rootTemplate, rootTemplate, rootKey, rootKey)
	intermediate := issue(&x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "SYNTHETIC_TEST_INTERMEDIATE"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}, root, intermediateKey, rootKey)
	intermediatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediate.Raw})
	roots := x509.NewCertPool()
	roots.AddCert(root)
	for _, scenario := range []struct {
		name        string
		dns         string
		expires     time.Time
		trusted     bool
		wantSuccess bool
	}{
		{"missing intermediate succeeds", "provider.example", now.Add(time.Hour), true, true},
		{"untrusted root rejected", "provider.example", now.Add(time.Hour), false, false},
		{"wrong hostname rejected", "other.example", now.Add(time.Hour), true, false},
		{"expired leaf rejected", "provider.example", now.Add(-time.Minute), true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			leaf := issue(&x509.Certificate{
				SerialNumber: big.NewInt(3), DNSNames: []string{scenario.dns},
				NotBefore: now.Add(-time.Hour), NotAfter: scenario.expires,
				KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}, intermediate, leafKey, intermediateKey)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(http.StatusOK)
			}))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leaf.Raw}, PrivateKey: leafKey}}}
			server.StartTLS()
			defer server.Close()
			trustedRoots := roots
			if !scenario.trusted {
				trustedRoots = x509.NewCertPool()
			}
			config, err := supplementalTLSConfig("provider.example", intermediatePEM, trustedRoots)
			if err != nil {
				t.Fatal(err)
			}
			if config.InsecureSkipVerify {
				t.Fatal("TLS verification disabled")
			}
			transport := &http.Transport{TLSClientConfig: config}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: time.Second}
			response, err := client.Get(server.URL)
			if response != nil {
				response.Body.Close()
			}
			if (err == nil) != scenario.wantSuccess {
				t.Fatalf("success=%v error=%v", scenario.wantSuccess, err)
			}
		})
	}
	for _, invalid := range [][]byte{nil, []byte("not PEM"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1}})} {
		if _, err := supplementalTLSConfig("provider.example", invalid, roots); err == nil {
			t.Fatal("invalid certificate accepted")
		}
	}
}

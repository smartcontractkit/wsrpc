package wsrpc

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"math/big"
)

type StaticSizePubKey [ed25519.PublicKeySize]byte

func newServerTLSConfig(priv ed25519.PrivateKey, clientIdentities map[StaticSizePubKey]string) tls.Config {
	cert := newMinimalX509CertFromPrivateKey(priv)

	return tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,

		// Since our clients use self-signed certs, we skip verficiation here.
		// Instead, we use VerifyPeerCertificate for our own check
		InsecureSkipVerify: true,

		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,

		VerifyPeerCertificate: verifyCertMatchesIdentity(clientIdentities),
	}
}

func newClientTLSConfig(priv ed25519.PrivateKey, serverIdentities map[StaticSizePubKey]string) tls.Config {
	cert := newMinimalX509CertFromPrivateKey(priv)

	return tls.Config{
		Certificates: []tls.Certificate{cert},

		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,

		// We pin the self-signed server public key rn.
		// If we wanted to use a proper CA for the server public key,
		// InsecureSkipVerify and VerifyPeerCertificate should be
		// removed. (See also discussion in README.md)
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyCertMatchesIdentity(serverIdentities),
	}
}

// Generates a minimal certificate (that wouldn't be considered valid outside this telemetry networking protocol)
// from an Ed25519 private key.
func newMinimalX509CertFromPrivateKey(sk ed25519.PrivateKey) tls.Certificate {
	template := x509.Certificate{
		SerialNumber: big.NewInt(0), // serial number must be set, so we set it to 0
	}

	encodedCert, err := x509.CreateCertificate(rand.Reader, &template, &template, sk.Public(), sk)
	if err != nil {
		panic(err)
	}

	return tls.Certificate{
		Certificate:                  [][]byte{encodedCert},
		PrivateKey:                   sk,
		SupportedSignatureAlgorithms: []tls.SignatureScheme{tls.Ed25519},
	}
}

func verifyCertMatchesIdentity(identities map[StaticSizePubKey]string) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) != 1 {
			return fmt.Errorf("Required exactly one client certificate")
		}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return err
		}
		pk, err := pubKeyFromCert(cert)
		if err != nil {
			return err
		}

		name, ok := identities[pk]
		if !ok {
			return fmt.Errorf("Unknown public key on cert %x", pk)
		}

		log.Printf("Got good cert from %v", name)
		return nil
	}
}

func pubKeyFromCert(cert *x509.Certificate) (pk StaticSizePubKey, err error) {
	if cert.PublicKeyAlgorithm != x509.Ed25519 {
		return pk, fmt.Errorf("Require ed25519 public key")
	}
	return StaticallySizedEd25519PublicKey(cert.PublicKey)
}

func StaticallySizedEd25519PublicKey(publickey crypto.PublicKey) (StaticSizePubKey, error) {
	var result [ed25519.PublicKeySize]byte

	pkslice, ok := publickey.(ed25519.PublicKey)
	if !ok {
		return result, fmt.Errorf("Invalid ed25519 public key")
	}
	if ed25519.PublicKeySize != copy(result[:], pkslice) {
		// assertion
		panic("copying public key failed")
	}
	return result, nil
}

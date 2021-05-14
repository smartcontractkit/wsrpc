package credentials

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"math/big"
)

type StaticSizedPublicKey [ed25519.PublicKeySize]byte

// NewClientTLSConfig uses the private key and public keys to construct a mutual
// TLS config for the client.
func NewClientTLSConfig(priv ed25519.PrivateKey, pubs []ed25519.PublicKey) (*tls.Config, error) {
	return newMutualTLSConfig(priv, pubs)
}

// NewServerTLSConfig uses the private key and public keys to construct a mutual
// TLS config for the server.
func NewServerTLSConfig(priv ed25519.PrivateKey, pubs []ed25519.PublicKey) (*tls.Config, error) {
	c, err := newMutualTLSConfig(priv, pubs)
	if err != nil {
		return nil, err
	}
	c.ClientAuth = tls.RequireAnyClientCert

	return c, nil
}

// newMutualTLSConfig uses the private key and public keys to construct a mutual
// TLS 1.3 config.
//
// We provide our own peer certificate verification function to check the
// certificate's public key matches our list of registered keys.
func newMutualTLSConfig(priv ed25519.PrivateKey, pubs []ed25519.PublicKey) (*tls.Config, error) {
	cert, err := newMinimalX509Cert(priv)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},

		// Since our clients use self-signed certs, we skip verficiation here.
		// Instead, we use VerifyPeerCertificate for our own check
		InsecureSkipVerify: true,

		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,

		VerifyPeerCertificate: verifyPeerCertificate(pubs),
	}, nil
}

// Generates a minimal certificate (that wouldn't be considered valid outside of
// this networking protocol) from an Ed25519 private key.
func newMinimalX509Cert(priv ed25519.PrivateKey) (tls.Certificate, error) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(0), // serial number must be set, so we set it to 0
	}

	encodedCert, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate:                  [][]byte{encodedCert},
		PrivateKey:                   priv,
		SupportedSignatureAlgorithms: []tls.SignatureScheme{tls.Ed25519},
	}, nil
}

// Verifies that the certificate's public key matches with one of the keys in
// our list of registered keys.
func verifyPeerCertificate(pubs []ed25519.PublicKey) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
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

		ok := isValidPublicKey(pubs, pk)
		if !ok {
			return fmt.Errorf("Unknown public key on cert %x", pk)
		}

		log.Printf("[TLS] Received good certificate")
		return nil
	}
}

// isValidPublicKey checks the public key against a list of valid keys.
func isValidPublicKey(valid []ed25519.PublicKey, pub ed25519.PublicKey) bool {
	for _, vpub := range valid {
		if pub.Equal(vpub) {
			return true
		}
	}
	return false
}

// PubKeyFromCert extracts the public key from the cert and returns it as a
// statically sized byte array.
func PubKeyFromCert(cert *x509.Certificate) (StaticSizedPublicKey, error) {
	pubKey, err := pubKeyFromCert(cert)
	if err != nil {
		return StaticSizedPublicKey{}, nil
	}

	return staticallySizedEd25519PublicKey(pubKey)
}

// pubKeyFromCert returns an ed25519 public key extracted from the certificate.
func pubKeyFromCert(cert *x509.Certificate) (ed25519.PublicKey, error) {
	if cert.PublicKeyAlgorithm != x509.Ed25519 {
		return nil, fmt.Errorf("Requires an ed25519 public key")
	}

	pub, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Invalid ed25519 public key")
	}

	return pub, nil
}

// staticallySizedEd25519PublicKey converts an ed25519 public key into a
// statically sized byte array.
func staticallySizedEd25519PublicKey(pubKey ed25519.PublicKey) (StaticSizedPublicKey, error) {
	var result [ed25519.PublicKeySize]byte

	if ed25519.PublicKeySize != copy(result[:], pubKey) {
		return StaticSizedPublicKey{}, errors.New("copying public key failed")
	}

	return result, nil
}
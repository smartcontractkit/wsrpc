package credentials

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewClientTLSConfig(t *testing.T) {
	_, cpriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	spub, spriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	tlsCfg, err := NewClientTLSConfig(cpriv, &PublicKeys{spub})
	require.NoError(t, err)
	require.Len(t, tlsCfg.Certificates, 1)

	assert.True(t, tlsCfg.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsCfg.MinVersion)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsCfg.MaxVersion)

	// Test a valid server certificate
	scert, err := newMinimalX509Cert(spriv)
	require.NoError(t, err)

	err = tlsCfg.VerifyPeerCertificate(scert.Certificate, [][]*x509.Certificate{})
	require.NoError(t, err)

	// Test an invalid client certificate
	_, invspriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	invscert, err := newMinimalX509Cert(invspriv)
	require.NoError(t, err)

	err = tlsCfg.VerifyPeerCertificate(invscert.Certificate, [][]*x509.Certificate{})
	require.Error(t, err)
}

func Test_NewServerTLSConfig(t *testing.T) {
	_, spriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	cpub, cpriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	tlsCfg, err := NewServerTLSConfig(spriv, &PublicKeys{cpub})
	require.NoError(t, err)
	require.Len(t, tlsCfg.Certificates, 1)

	assert.True(t, tlsCfg.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsCfg.MinVersion)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsCfg.MaxVersion)
	assert.Equal(t, tls.RequireAnyClientCert, tlsCfg.ClientAuth)

	// Test a valid client certificate
	ccert, err := newMinimalX509Cert(cpriv)
	require.NoError(t, err)

	err = tlsCfg.VerifyPeerCertificate(ccert.Certificate, [][]*x509.Certificate{})
	require.NoError(t, err)

	// Test an invalid client certificate
	_, invcpriv, err := ed25519.GenerateKey(rand.New(rand.NewSource(42))) //nolint:gosec
	require.NoError(t, err)

	invccert, err := newMinimalX509Cert(invcpriv)
	require.NoError(t, err)

	err = tlsCfg.VerifyPeerCertificate(invccert.Certificate, [][]*x509.Certificate{})
	require.Error(t, err)
}

func Test_PubKeyFromCert(t *testing.T) {
	randReader := rand.New(rand.NewSource(42)) //nolint:gosec

	pub, priv, err := ed25519.GenerateKey(randReader)
	require.NoError(t, err)

	template := x509.Certificate{SerialNumber: big.NewInt(0)}
	encodedCert, err := x509.CreateCertificate(randReader, &template, &template, priv.Public(), priv)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(encodedCert)
	require.NoError(t, err)

	actual, err := PubKeyFromCert(cert)
	require.NoError(t, err)

	assert.ElementsMatch(t, pub, actual)
}

func Test_PubKeyFromCert_MustBeEd25519KeyError(t *testing.T) {
	randReader := rand.New(rand.NewSource(42)) //nolint:gosec

	priv, err := rsa.GenerateKey(randReader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{SerialNumber: big.NewInt(0)}
	encodedCert, err := x509.CreateCertificate(randReader, &template, &template, priv.Public(), priv)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(encodedCert)
	require.NoError(t, err)

	_, err = PubKeyFromCert(cert)
	require.EqualError(t, err, "requires an ed25519 public key")
}

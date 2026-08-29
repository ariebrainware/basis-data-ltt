package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	outDir := "certs"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Printf("Failed to create directory %s: %v\n", outDir, err)
		os.Exit(1)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Failed to generate private key: %v\n", err)
		os.Exit(1)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		fmt.Printf("Failed to generate serial number: %v\n", err)
		os.Exit(1)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Local Development"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		fmt.Printf("Failed to create certificate: %v\n", err)
		os.Exit(1)
	}

	crtPath := filepath.Join(outDir, "localhost.crt")
	crtFile, err := os.Create(crtPath)
	if err != nil {
		fmt.Printf("Failed to create cert file %s: %v\n", crtPath, err)
		os.Exit(1)
	}
	defer crtFile.Close()

	if err := pem.Encode(crtFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		fmt.Printf("Failed to write cert PEM: %v\n", err)
		os.Exit(1)
	}

	keyPath := filepath.Join(outDir, "localhost.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Failed to create key file %s: %v\n", keyPath, err)
		os.Exit(1)
	}
	defer keyFile.Close()

	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
		fmt.Printf("Failed to write key PEM: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated TLS certificates:\n  Certificate: %s\n  Private Key: %s\n", crtPath, keyPath)
}

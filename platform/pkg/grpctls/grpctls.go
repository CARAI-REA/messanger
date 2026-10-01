package grpctls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ServerOptions returns TLS server options when cert/key are set; otherwise nil (plaintext).
func ServerOptions(certFile, keyFile, clientCAFile string) ([]grpc.ServerOption, error) {
	if certFile == "" || keyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server tls: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if clientCAFile != "" {
		pem, err := os.ReadFile(clientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read client ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("invalid client ca")
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(cfg))}, nil
}

// DialOption returns a grpc dial option for TLS or insecure.
func DialOption(useTLS bool, caFile, serverName string) (grpc.DialOption, error) {
	if !useTLS {
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("invalid ca")
		}
		cfg.RootCAs = pool
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(cfg)), nil
}

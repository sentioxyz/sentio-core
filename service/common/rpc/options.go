package rpc

import (
	"flag"
)

var MaxRevSize = 384 * 1024 * 1024

// https://dev.to/techschoolguru/how-to-secure-grpc-connection-with-ssl-tls-in-go-4ph

var enableTLS = flag.Bool("enable_tls", true, "enable TLS")
var prefix = "service/common/rpc/certs/"
var serverCert = flag.String("server-cert", prefix+"server-cert.pem", "server certificate file")
var serverKey = flag.String("server-key", prefix+"server-key.pem", "server key file")
var clientCert = flag.String("client-cert", prefix+"client-cert.pem", "client certificate file")
var clientKey = flag.String("client-key", prefix+"client-key.pem", "client key file")
var caCert = flag.String("ca-cert", prefix+"ca-cert.pem", "CA certificate file")

package netexp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"time"

	"github.com/qujing226/atrium/sac"
)

var ErrUnsupportedTLSFeature = errors.New("tls feature unsupported by Go crypto/tls runner")

type TLSExperiment struct {
	Mode             sac.Mode
	VerifierDelay    time.Duration
	EvidenceValid    bool
	Payloads         [][]byte
	BufferConfig     sac.Config
	OperationTimeout time.Duration
}

type TLSResult struct {
	UsedTLS    bool
	TLSVersion uint16

	FramesWritten int
	FramesRead    int

	Delivered         int
	InvalidDeliveries int
	Aborted           bool

	TimeToFirstFrame            time.Duration
	TimeToFirstVerifiedDelivery time.Duration
	VerificationLatency         time.Duration
}

func RunTLS(ctx context.Context, exp TLSExperiment) (TLSResult, error) {
	switch exp.Mode {
	case sac.ModeTLS13EarlyData0RTT, sac.ModeTLS13PostHandshakeAuth:
		return TLSResult{}, ErrUnsupportedTLSFeature
	}

	if exp.OperationTimeout <= 0 {
		exp.OperationTimeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, exp.OperationTimeout)
	defer cancel()

	cert, err := selfSignedCertificate()
	if err != nil {
		return TLSResult{}, err
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
	})
	if err != nil {
		return TLSResult{}, err
	}
	defer listener.Close()

	resultCh := make(chan TLSResult, 1)
	errCh := make(chan error, 2)

	start := time.Now()
	go runTLSServer(ctx, listener, exp, start, resultCh, errCh)
	go runTLSClient(ctx, listener.Addr().String(), exp, errCh)

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return TLSResult{}, err
	case <-ctx.Done():
		return TLSResult{}, ctx.Err()
	}
}

func runTLSClient(ctx context.Context, addr string, exp TLSExperiment, errCh chan<- error) {
	conn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
	})
	if err != nil {
		errCh <- err
		return
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	for _, payload := range exp.Payloads {
		if err := writeRecord(writer, payload); err != nil {
			errCh <- err
			return
		}
	}
	if err := writer.Flush(); err != nil {
		errCh <- err
	}
}

func runTLSServer(ctx context.Context, listener net.Listener, exp TLSExperiment, start time.Time, resultCh chan<- TLSResult, errCh chan<- error) {
	rawConn, err := listener.Accept()
	if err != nil {
		errCh <- err
		return
	}
	defer rawConn.Close()

	conn, ok := rawConn.(*tls.Conn)
	if !ok {
		errCh <- errors.New("accepted connection is not TLS")
		return
	}
	if err := conn.HandshakeContext(ctx); err != nil {
		errCh <- err
		return
	}

	result := TLSResult{
		UsedTLS:             true,
		TLSVersion:          conn.ConnectionState().Version,
		VerificationLatency: exp.VerifierDelay,
	}
	session := sac.NewSession(exp.BufferConfig)

	readDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(conn)
		for {
			payload, err := readRecord(reader)
			if err != nil {
				if errors.Is(err, io.EOF) {
					readDone <- nil
				} else {
					readDone <- err
				}
				return
			}
			result.FramesRead++
			if result.FramesRead == 1 {
				result.TimeToFirstFrame = time.Since(start)
			}
			switch exp.Mode {
			case sac.ModeTLS13AppLayerExternalVerifier:
				if _, err := session.ReceivePlaintext(sac.Message{Sequence: uint64(result.FramesRead), Payload: payload}); err != nil {
					result.Aborted = true
					readDone <- err
					return
				}
			default:
				result.Delivered++
				if result.Delivered == 1 {
					result.TimeToFirstVerifiedDelivery = time.Since(start)
				}
			}
		}
	}()

	select {
	case err := <-readDone:
		if err != nil {
			errCh <- err
			return
		}
	case <-ctx.Done():
		errCh <- ctx.Err()
		return
	}
	result.FramesWritten = len(exp.Payloads)

	switch exp.Mode {
	case sac.ModeTLS13AppLayerExternalVerifier:
		if !waitVerifier(ctx, exp.VerifierDelay) {
			errCh <- ctx.Err()
			return
		}
		if exp.EvidenceValid {
			released, err := session.VerifySuccess()
			if err != nil {
				errCh <- err
				return
			}
			result.Delivered = len(released)
			if result.Delivered > 0 {
				result.TimeToFirstVerifiedDelivery = time.Since(start)
			}
		} else {
			_ = session.VerifyFailure(nil)
			result.Aborted = true
		}
	}

	resultCh <- result
}

func waitVerifier(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func writeRecord(w *bufio.Writer, payload []byte) error {
	if len(payload) > 1<<20 {
		return errors.New("payload too large")
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(payload)))
	if _, err := w.Write(length[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readRecord(r *bufio.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(r, length[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(length[:])
	if n > 1<<20 {
		return nil, errors.New("payload too large")
	}
	payload := make([]byte, n)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func selfSignedCertificate() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sac-netexp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return tls.X509KeyPair(certPEM, keyPEM)
}

package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	atriumv1 "github.com/qujing226/atrium/gen/go/atrium/v1"
	"github.com/qujing226/atrium/pkg/blockchain"
	"github.com/qujing226/atrium/pkg/connect"
	"github.com/qujing226/atrium/pkg/model"
	"github.com/qujing226/atrium/pkg/secure"
)

type Client struct {
	Did      string
	SignKeys *secure.SignKeyPair
	KemKeys  *secure.KyberKeyPair

	Chain     *blockchain.OptimisticCache
	RelayAddr string
	Conn      net.Conn

	mu             sync.Mutex
	CurrentSession *Session // Defined in session.go
	handshakeChan  chan error

	// Callback for application layer
	OnMessage func(sender string, msg []byte)
}

// NewClient creates a new QLink client with Optimistic Cache integration.
func NewClient(did string, chain *blockchain.OptimisticCache, relayAddr string) (*Client, error) {
	signKp, err := secure.NewSignKeyPair()
	if err != nil {
		return nil, err
	}
	kemKp, err := secure.NewKyberKeyPair()
	if err != nil {
		return nil, err
	}

	// Register self to the chain (simulation setup)
	pk, _ := kemKp.Export()
	signPk, _ := signKp.Export()
	doc, err := model.BuildDIDDocument(did, signPk, pk)
	if err != nil {
		return nil, err
	}
	chain.RegisterDidDoc(did, doc)

	return &Client{
		Did:           did,
		SignKeys:      signKp,
		KemKeys:       kemKp,
		Chain:         chain,
		RelayAddr:     relayAddr,
		handshakeChan: make(chan error, 1),
	}, nil
}

// OnChainVerification is the callback invoked by OptimisticCache when background resolution completes.
// This implements the "Eventual Consistency" logic.
func (c *Client) OnChainVerification(targetDid string, cached, fresh []byte) {
	c.mu.Lock()
	sess := c.CurrentSession
	c.mu.Unlock()

	if sess == nil || sess.PeerDid != targetDid {
		return
	}

	// Simple byte comparison for equality
	isMatch := string(cached) == string(fresh)

	if isMatch {
		// Happy Path: Atomic Upgrade
		fmt.Printf("[%s] Background Check OK. Upgrading session with %s to VERIFIED.\n", c.Did, targetDid)
		bufferedMsgs := sess.UpgradeToVerified()

		// Flush buffer to application layer
		for _, msg := range bufferedMsgs {
			if c.OnMessage != nil {
				fmt.Printf("[%s] Flushing buffered msg from %s...\n", c.Did, targetDid)
				c.OnMessage(targetDid, msg)
			}
		}
	} else {
		// Attack Path: Atomic Abort
		fmt.Printf("[%s] CRITICAL: Chain mismatch for %s! Aborting session.\n", c.Did, targetDid)

		// 1. Send Error Signal to Peer (Proactive Propagation)
		// We try to send this even if we are about to close.
		statusPkt := &atriumv1.Packet{
			Header: c.newHeader(targetDid),
			Payload: &atriumv1.Packet_Error{
				Error: &atriumv1.Error{
					Error: "Blockchain mismatch detected during asynchronous verification",
				},
			},
		}
		statusPkt.Header.Code = atriumv1.Code_CODE_ERROR_VERIFICATION_FAILED
		if c.Conn != nil {
			connect.WritePacket(c.Conn, statusPkt)
		}

		// 2. Local Meltdown
		sess.Abort()
		if c.Conn != nil {
			c.Conn.Close()
		}
	}
}

func (c *Client) Start() error {
	conn, err := net.Dial("tcp", c.RelayAddr)
	if err != nil {
		return err
	}
	c.Conn = conn
	fmt.Printf("[%s] Online at %s\n", c.Did, c.RelayAddr)

	regPkt := &atriumv1.Packet{
		Header: c.newHeader(""),
		// For registration, we can just use a dummy payload or maybe just header since Relay only cares about FromDid
		Payload: &atriumv1.Packet_Error{
			Error: &atriumv1.Error{Error: "Register"}, // We will change relay logic later if needed
		},
	}
	regPkt.Header.Code = atriumv1.Code_CODE_SUCCESS
	if err := connect.WritePacket(c.Conn, regPkt); err != nil {
		return err
	}

	go c.readLoop()
	return nil
}

func (c *Client) readLoop() {
	defer c.Conn.Close()
	for {
		pkt, err := connect.ReadPacket(c.Conn)
		if err != nil {
			return
		}
		c.handlePacket(pkt)
	}
}

func (c *Client) handlePacket(pkt *atriumv1.Packet) {
	switch p := pkt.Payload.(type) {
	case *atriumv1.Packet_KemInit:
		c.handleKemInit(pkt.Header, p.KemInit)
	case *atriumv1.Packet_KemConfirm:
		c.handleKemConfirm(pkt.Header, p.KemConfirm)
	case *atriumv1.Packet_SecureMessage:
		c.handleSecureMessage(pkt.Header, p.SecureMessage)
	case *atriumv1.Packet_Error:
		c.handleStatus(pkt.Header, p.Error)
	}
}

// =============================================================================
//  Handshake Logic
// =============================================================================

func (c *Client) Handshake(targetDid string) error {
	// 1. Resolve (May hit cache or block)
	start := time.Now()
	doc, err := c.Chain.Resolve(targetDid)
	duration := time.Since(start)

	if err != nil {
		return err
	}

	// Determine Initial State
	initialState := atriumv1.SessionState_SESSION_STATE_VERIFIED
	if duration < 10*time.Millisecond {
		initialState = atriumv1.SessionState_SESSION_STATE_SPECULATIVE
	}

	// 2. Crypto Setup
	parsedDoc, err := model.ParseDIDDocument(doc)
	if err != nil {
		return err
	}
	targetKyberBytes, err := parsedDoc.GetKyberPubKey()
	if err != nil {
		return err
	}

	targetKyberPk, err := secure.LoadFromBytes(targetKyberBytes, nil)
	if err != nil {
		return err
	}
	ct, ss, err := targetKyberPk.Encapsulate()
	if err != nil {
		return err
	}

	// 3. Init Session
	txRoot := secure.SimpleKDF(ss, nil, []byte("A->B"))
	rxRoot := secure.SimpleKDF(ss, nil, []byte("B->A"))

	c.mu.Lock()
	c.CurrentSession = NewSession(targetDid, initialState, secure.NewChainKey(txRoot), secure.NewChainKey(rxRoot))
	c.mu.Unlock()

	// 4. Send Packet
	nonce := make([]byte, 32)
	rand.Read(nonce)
	sig, _ := c.SignKeys.Sign(append(ct, nonce...))

	pkt := &atriumv1.Packet{
		Header: c.newHeader(targetDid),
		Payload: &atriumv1.Packet_KemInit{
			KemInit: &atriumv1.KEMInit{Ct: ct, Nonce: nonce},
		},
		Credential: &atriumv1.Credential{Signature: sig},
	}
	if err := connect.WritePacket(c.Conn, pkt); err != nil {
		return err
	}

	// 5. Wait
	select {
	case err := <-c.handshakeChan:
		return err
	case <-time.After(5 * time.Second):
		return errors.New("handshake timeout")
	}
}

func (c *Client) handleKemInit(header *atriumv1.Header, body *atriumv1.KEMInit) {
	// Symmetric: Responder also resolves/verifies
	_, err := c.Chain.Resolve(header.FromDid)
	if err != nil {
		return
	}

	// Assume Speculative for responder (simplification for prototype)
	initialState := atriumv1.SessionState_SESSION_STATE_SPECULATIVE

	ss, err := c.KemKeys.Decapsulate(body.Ct)
	if err != nil {
		return
	}

	txRoot := secure.SimpleKDF(ss, nil, []byte("B->A"))
	rxRoot := secure.SimpleKDF(ss, nil, []byte("A->B"))

	c.mu.Lock()
	c.CurrentSession = NewSession(header.FromDid, initialState, secure.NewChainKey(txRoot), secure.NewChainKey(rxRoot))
	c.mu.Unlock()

	sig, _ := c.SignKeys.Sign(body.Nonce)
	resp := &atriumv1.Packet{
		Header: c.newHeader(header.FromDid),
		Payload: &atriumv1.Packet_KemConfirm{
			KemConfirm: &atriumv1.KEMConfirm{NonceHash: body.Nonce},
		},
		Credential: &atriumv1.Credential{Signature: sig},
	}
	connect.WritePacket(c.Conn, resp)
}

func (c *Client) handleKemConfirm(header *atriumv1.Header, body *atriumv1.KEMConfirm) {
	select {
	case c.handshakeChan <- nil:
	default:
	}
}

// =============================================================================
//  Data Plane (With Data Gate)
// =============================================================================

func (c *Client) SendMessage(msg string) error {
	c.mu.Lock()
	sess := c.CurrentSession
	c.mu.Unlock()

	if sess == nil || sess.IsAborted() {
		return errors.New("session not active")
	}

	// 1. Ratchet
	msgKey, err := sess.TxRatchet.Ratchet()
	if err != nil {
		return err
	}

	// 2. Encrypt
	block, _ := aes.NewCipher(msgKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	ciphertext := gcm.Seal(nil, nonce, []byte(msg), nil)

	// 3. Send
	sess.TxSeq++
	sess.RecordMessageSent()

	if sess.NeedsEpochKEM() {
		fmt.Printf("[%s] Q-Ratchet Alert: Security Budget Exceeded! Initiating Epoch-KEM for PCS...\n", c.Did)
		// 实际上这里应该触发一个新的 Handshake(sess.PeerDid)
		// 并在成功后调用 sess.ResetEpochKEM()。为了不打断当前逻辑，我们仅打印日志并模拟重置。
		// go c.Handshake(sess.PeerDid)
		sess.ResetEpochKEM()
	}

	pkt := &atriumv1.Packet{
		Header: c.newHeader(sess.PeerDid),
		Payload: &atriumv1.Packet_SecureMessage{
			SecureMessage: &atriumv1.SecureMessage{
				SequenceNumber: sess.TxSeq,
				Ciphertext:     ciphertext,
				Nonce:          nonce,
			},
		},
	}
	return connect.WritePacket(c.Conn, pkt)
}

func (c *Client) handleSecureMessage(header *atriumv1.Header, body *atriumv1.SecureMessage) {
	c.mu.Lock()
	sess := c.CurrentSession
	c.mu.Unlock() // Unlock early, Session has its own lock

	if sess == nil || sess.PeerDid != header.FromDid {
		return
	}

	// 1. Decrypt (Always performed to maintain Ratchet sync)
	msgKey, err := sess.RxRatchet.Ratchet()
	if err != nil {
		return
	}
	block, _ := aes.NewCipher(msgKey)
	gcm, _ := cipher.NewGCM(block)
	plaintext, err := gcm.Open(nil, body.Nonce, body.Ciphertext, nil)
	if err != nil {
		fmt.Printf("[%s] Decryption failed\n", c.Did)
		return
	}

	// 2. Process via Data Gate (The Core Logic)
	deliverNow, err := sess.ProcessIncomingMsg(plaintext)
	if err != nil {
		// Session Aborted or Overflow
		fmt.Printf("[%s] Message dropped: %v\n", c.Did, err)
		return
	}

	if deliverNow {
		if c.OnMessage != nil {
			c.OnMessage(header.FromDid, plaintext)
		}
	} else {
		// fmt.Printf("[%s] Message buffered (Speculative)\n", c.Did)
	}
}

func (c *Client) handleStatus(header *atriumv1.Header, errPayload *atriumv1.Error) {
	if header.Code == atriumv1.Code_CODE_ERROR_VERIFICATION_FAILED {
		c.mu.Lock()
		sess := c.CurrentSession
		c.mu.Unlock()

		if sess != nil && sess.PeerDid == header.FromDid {
			fmt.Printf("[%s] Received ROLLBACK SIGNAL from %s. Aborting!\n", c.Did, header.FromDid)
			sess.Abort()
			c.Conn.Close()
		}
	}
}

func (c *Client) newHeader(to string) *atriumv1.Header {
	return &atriumv1.Header{
		RequestId: uuid.New().String(),
		FromDid:   c.Did,
		ToDid:     to,
		Timestamp: time.Now().UnixMilli(),
	}
}

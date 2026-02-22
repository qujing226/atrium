package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	didv1 "github.com/qujing226/QLink/spec/gen/qlink/did/v1"
	"github.com/qujing226/QLink/spec/pkg/connect"
)

// RelayServer 是一个并不“聪明”的转发器
// 它不知道谁是谁，只知道 "DID -> TCP Connection" 的映射
type RelayServer struct {
	listener net.Listener
	clients  map[string]net.Conn // DID -> Conn
	mu       sync.RWMutex
	stopChan chan struct{}
}

func NewRelayServer() *RelayServer {
	return &RelayServer{
		clients:  make(map[string]net.Conn),
		stopChan: make(chan struct{}),
	}
}

func (s *RelayServer) Start(port string) error {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	s.listener = l
	fmt.Printf("[Relay] Listening on 0.0.0.0:%s\n", port)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-s.stopChan:
					return // Server stopped
				default:
					fmt.Printf("[Relay] Accept error: %v\n", err)
					continue
				}
			}
			go s.handleConn(conn)
		}
	}()

	return nil
}

func (s *RelayServer) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *RelayServer) handleConn(conn net.Conn) {
	defer conn.Close()
	
	// 简单的连接标识，用于日志
	remoteAddr := conn.RemoteAddr().String()
	// fmt.Printf("[Relay] New connection from %s\n", remoteAddr)

	var registeredDID string

	defer func() {
		if registeredDID != "" {
			s.unregister(registeredDID)
			fmt.Printf("[Relay] Client disconnected: %s (%s)\n", registeredDID, remoteAddr)
		}
	}()

	for {
		// 1. 读取数据包
		pkt, err := connect.ReadPacket(conn)
		if err != nil {
			// EOF or error
			return
		}

		if pkt.Header == nil {
			fmt.Println("[Relay] Dropping packet with no header")
			continue
		}

		// 2. 注册逻辑 (Trust on First Use for Routing)
		// 如果这是该连接第一次发送包含 FromDid 的包，则绑定
		from := pkt.Header.FromDid
		if from != "" {
			if registeredDID == "" {
				s.register(from, conn)
				registeredDID = from
				fmt.Printf("[Relay] Registered: %s -> %s\n", from, remoteAddr)
			} else if registeredDID != from {
				// 简单的防欺诈：不允许同一个连接中途改名
				fmt.Printf("[Relay] Warning: Connection %s tried to spoof %s\n", registeredDID, from)
				return
			}
		}

		// 3. 路由逻辑
		to := pkt.Header.ToDid
		if to == "" {
			// 可能是心跳或无效包
			continue
		}

		targetConn := s.getClient(to)
		if targetConn != nil {
			// 转发
			if err := connect.WritePacket(targetConn, pkt); err != nil {
				fmt.Printf("[Relay] Failed to forward to %s: %v\n", to, err)
			}
		} else {
			// 目标不在线，返回错误状态
			// fmt.Printf("[Relay] Target not found: %s\n", to)
			s.sendError(conn, pkt.Header.RequestId, didv1.Status_CODE_ERROR_PROTOCOL_VIOLATION, "Target DID not connected")
		}
	}
}

func (s *RelayServer) register(did string, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[did] = conn
}

func (s *RelayServer) unregister(did string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[did]; ok {
		delete(s.clients, did)
	}
}

func (s *RelayServer) getClient(did string) net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[did]
}

func (s *RelayServer) sendError(conn net.Conn, replyToID string, code didv1.Status_Code, msg string) {
	statusPkt := &didv1.Packet{
		Header: &didv1.Header{
			Timestamp: time.Now().UnixMilli(),
			FromDid:   "did:qlink:relay", // Relay 自己的 ID
		},
		Payload: &didv1.Packet_Status{
			Status: &didv1.Status{
				ReplyToId: replyToID,
				Code:      code,
				Message:   msg,
			},
		},
	}
	connect.WritePacket(conn, statusPkt)
}

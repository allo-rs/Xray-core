package anytls

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/anytls/sing-anytls/padding"
	anytlssession "github.com/anytls/sing-anytls/session"
	satomic "github.com/sagernet/sing/common/atomic"
	M "github.com/sagernet/sing/common/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/allo-rs/Xray-core/common"
	"github.com/allo-rs/Xray-core/common/errors"
	"github.com/allo-rs/Xray-core/common/log"
	"github.com/allo-rs/Xray-core/common/net"
	"github.com/allo-rs/Xray-core/common/protocol"
	"github.com/allo-rs/Xray-core/common/session"
	"github.com/allo-rs/Xray-core/common/signal"
	"github.com/allo-rs/Xray-core/common/singbridge"
	"github.com/allo-rs/Xray-core/core"
	"github.com/allo-rs/Xray-core/features/policy"
	"github.com/allo-rs/Xray-core/features/routing"
	"github.com/allo-rs/Xray-core/proxy"
	"github.com/allo-rs/Xray-core/transport"
	"github.com/allo-rs/Xray-core/transport/internet/stat"
)

func init() {
	common.Must(common.RegisterConfig((*ServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewServer(ctx, config.(*ServerConfig))
	}))
}

// ── 账号类型 ────────────────────────────────────────────────────────────────

// MemoryAccount 是 AnyTLS 在内存中的账号表示，实现 protocol.Account。
type MemoryAccount struct {
	Password string
}

func (a *MemoryAccount) Equals(other protocol.Account) bool {
	if aa, ok := other.(*MemoryAccount); ok {
		return a.Password == aa.Password
	}
	return false
}

func (a *MemoryAccount) ToProto() proto.Message {
	return &User{Password: a.Password}
}

// ── Server ────────────────────────────────────────────────────────────────

type memoryUser struct {
	email    string
	password string // 原始密码，用于 GetUsers 返回
	level    uint32
}

// Server 是 AnyTLS 入站处理器，同时实现 proxy.UserManager。
type Server struct {
	userMu        sync.RWMutex
	users         map[[32]byte]*memoryUser // sha256(password) → user
	byEmail       map[string]*memoryUser   // email → user（加速 RemoveUser）
	padding       satomic.TypedValue[*padding.PaddingFactory]
	policyManager policy.Manager
}

var _ proxy.Inbound = (*Server)(nil)
var _ proxy.UserManager = (*Server)(nil)

// defaultPaddingScheme 与 sing-box 客户端默认值对齐。
const defaultPaddingScheme = "stop=8\n0=30-30\n1=100-400\n2=400-500"

func NewServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	v := core.MustFromContext(ctx)
	s := &Server{
		users:         make(map[[32]byte]*memoryUser, len(config.Users)),
		byEmail:       make(map[string]*memoryUser, len(config.Users)),
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	if !padding.UpdatePaddingScheme([]byte(defaultPaddingScheme), &s.padding) {
		return nil, errors.New("anytls: invalid default padding scheme")
	}

	for _, u := range config.Users {
		mu := &memoryUser{email: u.Email, password: u.Password}
		h := sha256.Sum256([]byte(u.Password))
		s.users[h] = mu
		s.byEmail[u.Email] = mu
	}

	return s, nil
}

// ── proxy.UserManager ────────────────────────────────────────────────────────

func (s *Server) AddUser(_ context.Context, u *protocol.MemoryUser) error {
	acc, ok := u.Account.(*MemoryAccount)
	if !ok {
		return errors.New("anytls: AddUser: unsupported account type")
	}
	mu := &memoryUser{email: u.Email, password: acc.Password, level: u.Level}
	h := sha256.Sum256([]byte(acc.Password))

	s.userMu.Lock()
	s.users[h] = mu
	s.byEmail[u.Email] = mu
	s.userMu.Unlock()
	return nil
}

func (s *Server) RemoveUser(_ context.Context, email string) error {
	s.userMu.Lock()
	defer s.userMu.Unlock()

	mu, ok := s.byEmail[email]
	if !ok {
		return errors.New("anytls: user not found: ", email)
	}
	h := sha256.Sum256([]byte(mu.password))
	delete(s.users, h)
	delete(s.byEmail, email)
	return nil
}

func (s *Server) GetUser(_ context.Context, email string) *protocol.MemoryUser {
	s.userMu.RLock()
	mu, ok := s.byEmail[email]
	s.userMu.RUnlock()
	if !ok {
		return nil
	}
	return &protocol.MemoryUser{
		Email:   mu.email,
		Level:   mu.level,
		Account: &MemoryAccount{Password: mu.password},
	}
}

func (s *Server) GetUsers(_ context.Context) []*protocol.MemoryUser {
	s.userMu.RLock()
	result := make([]*protocol.MemoryUser, 0, len(s.byEmail))
	for _, mu := range s.byEmail {
		result = append(result, &protocol.MemoryUser{
			Email:   mu.email,
			Level:   mu.level,
			Account: &MemoryAccount{Password: mu.password},
		})
	}
	s.userMu.RUnlock()
	return result
}

func (s *Server) GetUsersCount(_ context.Context) int64 {
	s.userMu.RLock()
	n := int64(len(s.byEmail))
	s.userMu.RUnlock()
	return n
}

// ── proxy.Inbound ────────────────────────────────────────────────────────────

func (s *Server) Network() []net.Network {
	return []net.Network{net.Network_TCP}
}

func (s *Server) Process(ctx context.Context, network net.Network, conn stat.Connection, dispatcher routing.Dispatcher) error {
	ib := session.InboundFromContext(ctx)
	if ib != nil {
		ib.Name = "anytls"
	}

	sessionPolicy := s.policyManager.ForLevel(0)

	// 1. 握手阶段设置读超时，防止慢速/半开连接泄露 goroutine
	if err := conn.SetReadDeadline(time.Now().Add(sessionPolicy.Timeouts.Handshake)); err != nil {
		return errors.New("anytls: set handshake deadline").Base(err)
	}

	// 2. 读取 32 字节 password hash（同时触发 TLS 握手）
	var pwHash [32]byte
	if _, err := io.ReadFull(conn, pwHash[:]); err != nil {
		return errors.New("anytls: read auth hash").Base(err)
	}

	// 3. 认证
	s.userMu.RLock()
	mu, ok := s.users[pwHash]
	s.userMu.RUnlock()
	if !ok {
		return errors.New("anytls: auth failed from ", conn.RemoteAddr())
	}

	// 4. 跳过 padding（AnyTLS 握手协议）
	var padLenBuf [2]byte
	if _, err := io.ReadFull(conn, padLenBuf[:]); err != nil {
		return errors.New("anytls: read padding len").Base(err)
	}
	if padLen := binary.BigEndian.Uint16(padLenBuf[:]); padLen > 0 {
		if _, err := io.CopyN(io.Discard, conn, int64(padLen)); err != nil {
			return errors.New("anytls: skip padding").Base(err)
		}
	}

	// 握手完成，清除 deadline
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return errors.New("anytls: clear deadline").Base(err)
	}

	// 5. 启动 AnyTLS 多路复用 session
	sess := anytlssession.NewServerSession(conn, func(stream *anytlssession.Stream) {
		go s.handleStream(ctx, mu, sessionPolicy, stream, dispatcher)
	}, &s.padding, singbridge.NewLogger(errors.New))
	sess.Run()
	return nil
}

// handleStream 处理单条 AnyTLS 复用流，设置用户身份、空闲超时，并分发到路由。
func (s *Server) handleStream(ctx context.Context, mu *memoryUser, sessionPolicy policy.Session, stream *anytlssession.Stream, dispatcher routing.Dispatcher) {
	defer stream.Close()

	// 保证任意退出路径都通知对端握手结果，防止对端 stream 挂起
	handshakeDone := false
	defer func() {
		if !handshakeDone {
			_ = stream.HandshakeFailure(errors.New("anytls: internal error"))
		}
	}()

	// 读取目标地址（sing SOCKS5 序列化格式）
	dest, err := M.SocksaddrSerializer.ReadAddrPort(stream)
	if err != nil {
		_ = stream.HandshakeFailure(err)
		handshakeDone = true
		return
	}
	xrayDest := singbridge.ToDestination(dest, net.Network_TCP)

	// 在 context 中设置用户身份，dispatcher 据此向 stats 系统写入
	// user>>>email>>>traffic>>>uplink / downlink 计数器
	streamCtx := ctx
	if ib := session.InboundFromContext(ctx); ib != nil {
		newIb := *ib
		newIb.User = &protocol.MemoryUser{Email: mu.email, Level: mu.level}
		streamCtx = session.ContextWithInbound(ctx, &newIb)
	}

	streamCtx = log.ContextWithAccessMessage(streamCtx, &log.AccessMessage{
		From:   stream.RemoteAddr(),
		To:     xrayDest,
		Status: log.AccessAccepted,
		Email:  mu.email,
	})
	errors.LogInfo(streamCtx, "anytls: ", mu.email, " tunnelling to ", xrayDest)

	// 空闲超时：复用 Xray policy 的 ConnectionIdle 配置
	streamCtx, cancel := context.WithCancel(streamCtx)
	timer := signal.CancelAfterInactivity(streamCtx, cancel, sessionPolicy.Timeouts.ConnectionIdle)
	defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)

	if err := stream.HandshakeSuccess(); err != nil {
		handshakeDone = true
		return
	}
	handshakeDone = true

	xConn := singbridge.NewConn(stream)
	if err := dispatcher.DispatchLink(streamCtx, xrayDest, &transport.Link{
		Reader: xConn,
		Writer: xConn,
	}); err != nil {
		errors.LogInfoInner(streamCtx, err, "anytls: dispatch failed")
	}
}

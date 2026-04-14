package anytls

import (
	"context"
	"io"
	"sync"
	"time"

	anytls "github.com/anytls/sing-anytls"
	anytlsutil "github.com/anytls/sing-anytls/util"

	"github.com/allo-rs/Xray-core/common"
	"github.com/allo-rs/Xray-core/common/buf"
	"github.com/allo-rs/Xray-core/common/errors"
	"github.com/allo-rs/Xray-core/common/net"
	"github.com/allo-rs/Xray-core/common/session"
	"github.com/allo-rs/Xray-core/common/singbridge"
	"github.com/allo-rs/Xray-core/common/task"
	core "github.com/allo-rs/Xray-core/core"
	"github.com/allo-rs/Xray-core/features/policy"
	"github.com/allo-rs/Xray-core/transport"
	"github.com/allo-rs/Xray-core/transport/internet"
)

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}

type Client struct {
	config        *ClientConfig
	policyManager policy.Manager
	serverDest    net.Destination

	mu     sync.Mutex
	client *anytls.Client
}

func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	v := core.MustFromContext(ctx)

	dest := net.TCPDestination(
		net.ParseAddress(config.Address),
		net.Port(config.Port),
	)

	return &Client{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
		serverDest:    dest,
	}, nil
}

// getOrCreateClient 懒初始化 anytls.Client，复用已有连接池。
// 若连接池已失效（服务端重启等），自动重建。
func (c *Client) getOrCreateClient(ctx context.Context, dialer internet.Dialer) (*anytls.Client, error) {
	// 快速路径：不持锁检查
	c.mu.Lock()
	existing := c.client
	c.mu.Unlock()

	if existing != nil {
		return existing, nil
	}

	// 慢路径：在锁外创建，避免 IO 期间持锁阻塞其他 goroutine
	cfg := c.config
	checkInterval := time.Duration(cfg.IdleSessionCheckInterval) * time.Second
	if checkInterval == 0 {
		checkInterval = 30 * time.Second
	}
	idleTimeout := time.Duration(cfg.IdleSessionTimeout) * time.Second
	if idleTimeout == 0 {
		idleTimeout = 30 * time.Second
	}

	serverDest := c.serverDest
	dialOut := func(dialCtx context.Context) (net.Conn, error) {
		rawConn, err := dialer.Dial(dialCtx, serverDest)
		if err != nil {
			return nil, err
		}
		conn, ok := rawConn.(net.Conn)
		if !ok {
			rawConn.Close()
			return nil, errors.New("anytls: underlying connection does not implement net.Conn")
		}
		return conn, nil
	}

	ac, err := anytls.NewClient(ctx, anytls.ClientConfig{
		Password:                 cfg.Password,
		IdleSessionCheckInterval: checkInterval,
		IdleSessionTimeout:       idleTimeout,
		MinIdleSession:           int(cfg.MinIdleSession),
		DialOut:                  anytlsutil.DialOutFunc(dialOut),
		Logger:                   singbridge.NewLogger(errors.New),
	})
	if err != nil {
		return nil, err
	}

	// CAS 写入：若另一个 goroutine 先到，关闭多余的 client
	c.mu.Lock()
	if c.client == nil {
		c.client = ac
	} else {
		ac.Close()
		ac = c.client
	}
	c.mu.Unlock()
	return ac, nil
}

// resetClient 清除失效的连接池，下次调用时重建。
func (c *Client) resetClient(dead *anytls.Client) {
	c.mu.Lock()
	if c.client == dead {
		c.client = nil
	}
	c.mu.Unlock()
}

func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	if !ob.Target.IsValid() {
		return errors.New("target not specified")
	}
	ob.Name = "anytls"
	destination := ob.Target

	ac, err := c.getOrCreateClient(ctx, dialer)
	if err != nil {
		return errors.New("failed to get anytls client").Base(err)
	}

	dest := singbridge.ToSocksaddr(destination)
	conn, err := ac.CreateProxy(ctx, dest)
	if err != nil {
		// 连接池失效（服务端重启等），下次请求时重建
		if err == io.ErrClosedPipe {
			c.resetClient(ac)
		}
		return errors.New("failed to create anytls proxy stream").Base(err)
	}
	defer conn.Close()

	errors.LogInfo(ctx, "anytls: tunnelling request to ", destination, " via ", c.serverDest.NetAddr())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	postRequest := func() error {
		return buf.Copy(link.Reader, buf.NewWriter(conn))
	}

	getResponse := func() error {
		defer cancel()
		return buf.Copy(buf.NewReader(conn), link.Writer)
	}

	responseDone := task.OnSuccess(getResponse, task.Close(link.Writer))
	if err := task.Run(ctx, postRequest, responseDone); err != nil {
		return errors.New("connection ends").Base(err)
	}

	return nil
}

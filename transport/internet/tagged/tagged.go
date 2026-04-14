package tagged

import (
	"context"

	"github.com/allo-rs/Xray-core/common/net"
	"github.com/allo-rs/Xray-core/features/routing"
)

type DialFunc func(ctx context.Context, dispatcher routing.Dispatcher, dest net.Destination, tag string) (net.Conn, error)

var Dialer DialFunc

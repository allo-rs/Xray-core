package tcp

import (
	"github.com/allo-rs/Xray-core/common"
	"github.com/allo-rs/Xray-core/transport/internet"
)

func init() {
	common.Must(internet.RegisterProtocolConfigCreator(protocolName, func() interface{} {
		return new(Config)
	}))
}

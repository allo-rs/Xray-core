package all

import (
	// The following are necessary as they register handlers in their init functions.

	// Mandatory features. Can't remove unless there are replacements.
	_ "github.com/allo-rs/Xray-core/app/dispatcher"
	_ "github.com/allo-rs/Xray-core/app/proxyman/inbound"
	_ "github.com/allo-rs/Xray-core/app/proxyman/outbound"

	// Default commander and all its services. This is an optional feature.
	_ "github.com/allo-rs/Xray-core/app/commander"
	_ "github.com/allo-rs/Xray-core/app/log/command"
	_ "github.com/allo-rs/Xray-core/app/proxyman/command"
	_ "github.com/allo-rs/Xray-core/app/stats/command"

	// Developer preview services
	_ "github.com/allo-rs/Xray-core/app/observatory/command"

	// Other optional features.
	_ "github.com/allo-rs/Xray-core/app/dns"
	_ "github.com/allo-rs/Xray-core/app/dns/fakedns"
	_ "github.com/allo-rs/Xray-core/app/log"
	_ "github.com/allo-rs/Xray-core/app/metrics"
	_ "github.com/allo-rs/Xray-core/app/policy"
	_ "github.com/allo-rs/Xray-core/app/reverse"
	_ "github.com/allo-rs/Xray-core/app/router"
	_ "github.com/allo-rs/Xray-core/app/stats"

	// Fix dependency cycle caused by core import in internet package
	_ "github.com/allo-rs/Xray-core/transport/internet/tagged/taggedimpl"

	// Developer preview features
	_ "github.com/allo-rs/Xray-core/app/observatory"

	// Inbound and outbound proxies.
	_ "github.com/allo-rs/Xray-core/proxy/blackhole"
	_ "github.com/allo-rs/Xray-core/proxy/dns"
	_ "github.com/allo-rs/Xray-core/proxy/dokodemo"
	_ "github.com/allo-rs/Xray-core/proxy/freedom"
	_ "github.com/allo-rs/Xray-core/proxy/http"
	_ "github.com/allo-rs/Xray-core/proxy/loopback"
	_ "github.com/allo-rs/Xray-core/proxy/shadowsocks"
	_ "github.com/allo-rs/Xray-core/proxy/socks"
	_ "github.com/allo-rs/Xray-core/proxy/anytls"
	_ "github.com/allo-rs/Xray-core/proxy/trojan"
	_ "github.com/allo-rs/Xray-core/proxy/vless/inbound"
	_ "github.com/allo-rs/Xray-core/proxy/vless/outbound"
	_ "github.com/allo-rs/Xray-core/proxy/vmess/inbound"
	_ "github.com/allo-rs/Xray-core/proxy/vmess/outbound"
	_ "github.com/allo-rs/Xray-core/proxy/wireguard"

	// Transports
	_ "github.com/allo-rs/Xray-core/transport/internet/grpc"
	_ "github.com/allo-rs/Xray-core/transport/internet/httpupgrade"
	_ "github.com/allo-rs/Xray-core/transport/internet/kcp"
	_ "github.com/allo-rs/Xray-core/transport/internet/reality"
	_ "github.com/allo-rs/Xray-core/transport/internet/splithttp"
	_ "github.com/allo-rs/Xray-core/transport/internet/tcp"
	_ "github.com/allo-rs/Xray-core/transport/internet/tls"
	_ "github.com/allo-rs/Xray-core/transport/internet/udp"
	_ "github.com/allo-rs/Xray-core/transport/internet/websocket"

	// Transport headers
	_ "github.com/allo-rs/Xray-core/transport/internet/headers/http"
	_ "github.com/allo-rs/Xray-core/transport/internet/headers/noop"

	// JSON & TOML & YAML
	_ "github.com/allo-rs/Xray-core/main/json"
	_ "github.com/allo-rs/Xray-core/main/toml"
	_ "github.com/allo-rs/Xray-core/main/yaml"

	// Load config from file or http(s)
	_ "github.com/allo-rs/Xray-core/main/confloader/external"

	// Commands
	_ "github.com/allo-rs/Xray-core/main/commands/all"
)

package conf

import (
	"github.com/allo-rs/Xray-core/proxy/anytls"
	"google.golang.org/protobuf/proto"
)

// AnyTLSUserConfig 对应 JSON 中单个用户配置
type AnyTLSUserConfig struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AnyTLSServerConfig 对应 inbound settings
type AnyTLSServerConfig struct {
	Users []AnyTLSUserConfig `json:"users"`
}

func (c *AnyTLSServerConfig) Build() (proto.Message, error) {
	users := make([]*anytls.User, 0, len(c.Users))
	for _, u := range c.Users {
		users = append(users, &anytls.User{Email: u.Email, Password: u.Password})
	}
	return &anytls.ServerConfig{Users: users}, nil
}

// AnyTLSClientConfig 对应 outbound settings
type AnyTLSClientConfig struct {
	Address                  string `json:"address"`
	Port                     uint16 `json:"port"`
	Password                 string `json:"password"`
	IdleSessionCheckInterval int64  `json:"idleSessionCheckInterval"`
	IdleSessionTimeout       int64  `json:"idleSessionTimeout"`
	MinIdleSession           int32  `json:"minIdleSession"`
}

func (c *AnyTLSClientConfig) Build() (proto.Message, error) {
	return &anytls.ClientConfig{
		Address:                  c.Address,
		Port:                     uint32(c.Port),
		Password:                 c.Password,
		IdleSessionCheckInterval: c.IdleSessionCheckInterval,
		IdleSessionTimeout:       c.IdleSessionTimeout,
		MinIdleSession:           c.MinIdleSession,
	}, nil
}


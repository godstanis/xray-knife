package xray

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/xtls/xray-core/infra/conf"

	"github.com/lilendian0x00/xray-knife/v2/utils"
)

func NewVmess() Protocol {
	return &Vmess{}
}

func (v *Vmess) Name() string {
	return "vmess"
}

func method1(v *Vmess, link string) error {
	b64encoded := link[8:]
	decoded, err := utils.Base64Decode(b64encoded)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(decoded, v); err != nil {
		return err
	}

	if utils.IsIPv6(v.Address) {
		v.Address = "[" + v.Address + "]"
	}
	return nil
}

// Example:
// vmess://YXV0bzpjYmI0OTM1OC00NGQxLTQ4MmYtYWExNC02ODA3NzNlNWNjMzdAc25hcHBmb29kLmlyOjQ0Mw?remarks=sth&obfsParam=huhierg.com&path=/&obfs=websocket&tls=1&peer=gdfgreg.com&alterId=0
func method2(v *Vmess, link string) error {
	uri, err := url.Parse(link)
	if err != nil {
		return err
	}
	decoded, err := utils.Base64Decode(uri.Host)
	if err != nil {
		return err
	}
	link = vmessIdentifier + string(decoded) + "?" + uri.RawQuery

	uri, err = url.Parse(link)
	if err != nil {
		return err
	}

	v.Security = uri.User.Username()
	v.ID, _ = uri.User.Password()

	v.Address, v.Port, err = net.SplitHostPort(uri.Host)
	if err != nil {
		return err
	}

	if utils.IsIPv6(v.Address) {
		v.Address = "[" + v.Address + "]"
	}
	// parseUint, err := strconv.ParseUint(suhp[2], 10, 16)
	// if err != nil {
	//	return err
	// }

	v.Aid = "0"

	queryValues := uri.Query()
	if value := queryValues.Get("remarks"); value != "" {
		v.Remark = value
	}

	if value := queryValues.Get("path"); value != "" {
		v.Path = value
	}

	if value := queryValues.Get("tls"); value == "1" {
		v.TLS = "tls"
	}

	if value := queryValues.Get("obfs"); value != "" {
		switch value {
		case "websocket":
			v.Network = "ws"
			v.Type = "none"
		case "none":
			v.Network = "tcp"
			v.Type = "none"
		}
	}
	if value := queryValues.Get("obfsParam"); value != "" {
		v.Host = value
	}
	if value := queryValues.Get("peer"); value != "" {
		v.SNI = value
	} else {
		if v.TLS == "tls" {
			v.SNI = v.Host
		}
	}

	return nil
}

// func method3(v *Vmess, link string) error {
//
// }

func (v *Vmess) Parse(configLink string) error {
	if !strings.HasPrefix(configLink, vmessIdentifier) {
		return fmt.Errorf("vmess unreconized: %s", configLink)
	}

	var err error = nil

	if err = method1(v, configLink); err != nil {
		if err = method2(v, configLink); err != nil {
			return err
		}
	}

	v.OrigLink = configLink

	if v.Type == "http" || v.Network == "ws" || v.Network == "h2" {
		if v.Path == "" {
			v.Path = "/"
		}
	}

	return err
}

func (v *Vmess) DetailsStr() string {
	copyV := *v
	result := make([][2]string, 0, 20)

	result = append(result, [][2]string{
		{"Protocol", v.Name()},
		{"Remark", copyV.Remark},
		{"Network", copyV.Network},
		{"Address", copyV.Address},
		{"Port", fmt.Sprintf("%v", copyV.Port)},
		{"UUID", copyV.ID},
	}...)

	switch {
	case copyV.Type == "http" || slices.Contains([]string{"httpupgrade", "ws", "h2"}, copyV.Network):
		if copyV.Type == "" {
			copyV.Type = "none"
		}
		if copyV.Host == "" {
			copyV.Host = "none"
		}
		if copyV.Path == "" {
			copyV.Path = "none"
		}

		result = append(result, [][2]string{
			{"Type", copyV.Type},
			{"Host", copyV.Host},
			{"Path", copyV.Path},
		}...)
	case copyV.Network == "kcp":
		result = append(result, [2]string{"KCP Seed", copyV.Path})
	case copyV.Network == "grpc":
		if copyV.Host == "" {
			copyV.Host = "none"
		}
		result = append(result, [][2]string{
			{"ServiceName", copyV.Path},
			{"Authority", copyV.Host},
		}...)
	}

	if copyV.TLS != "" && copyV.TLS != "none" {
		if copyV.SNI == "" {
			copyV.SNI = "none"
			if copyV.Host != "" {
				copyV.SNI = copyV.Host
			}
		}
		if copyV.ALPN == "" {
			copyV.ALPN = "none"
		}
		if len(copyV.TlsFingerprint) == 0 {
			copyV.TlsFingerprint = "none"
		}
		result = append(result, [][2]string{
			{"TLS", "tls"},
			{"SNI", copyV.SNI},
			{"ALPN", copyV.ALPN},
			{"Fingerprint", copyV.TlsFingerprint},
		}...)
	}

	return detailsToStr(result)
}

func (v *Vmess) ConvertToGeneralConfig() GeneralConfig {
	var g GeneralConfig
	g.Protocol = v.Name()
	g.Address = v.Address
	g.Aid = fmt.Sprintf("%v", v.Aid)
	g.Host = v.Host
	g.ID = v.ID
	g.Network = v.Network
	g.Path = v.Path
	g.Port = fmt.Sprintf("%v", v.Port)
	g.Remark = v.Remark
	if v.TLS == "" {
		g.TLS = "none"
	} else {
		g.TLS = v.TLS
	}
	g.TLS = v.TLS
	g.SNI = v.SNI
	g.ALPN = v.ALPN
	g.TlsFingerprint = v.TlsFingerprint
	g.Type = v.Type
	g.OrigLink = v.OrigLink

	return g
}

func (v *Vmess) BuildOutboundDetourConfig(allowInsecure bool) (*conf.OutboundDetourConfig, error) {
	out := &conf.OutboundDetourConfig{}
	out.Tag = "proxy"
	out.Protocol = v.Name()

	p := conf.TransportProtocol(v.Network)
	s := &conf.StreamConfig{
		Network:  &p,
		Security: v.TLS,
	}

	switch v.Network {
	case "tcp":
		s.TCPSettings = &conf.TCPConfig{}
		if v.Type == "" || v.Type == "none" {
			s.TCPSettings.HeaderConfig = json.RawMessage([]byte(`{ "type": "none" }`))
		} else {
			pathb, _ := json.Marshal(strings.Split(v.Path, ","))
			hostb, _ := json.Marshal(strings.Split(v.Host, ","))
			s.TCPSettings.HeaderConfig = json.RawMessage([]byte(fmt.Sprintf(`
			{
				"type": "http",
				"request": {
					"path": %s,
					"headers": {
						"Host": %s,
						"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36"
					}
				}
			}
			`, string(pathb), string(hostb))))
		}
	case "kcp":
		s.KCPSettings = &conf.KCPConfig{}
		s.KCPSettings.HeaderConfig = json.RawMessage([]byte(fmt.Sprintf(`{ "type": "%s" }`, v.Type)))
	case "ws":
		s.WSSettings = &conf.WebSocketConfig{}
		s.WSSettings.Path = v.Path
		s.WSSettings.Headers = map[string]string{
			"Host":       v.Host,
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36",
		}
	case "h2", "http":
		s.HTTPSettings = &conf.HTTPConfig{
			Path: v.Path,
		}
		if v.Host != "" {
			h := conf.StringList(strings.Split(v.Host, ","))
			s.HTTPSettings.Host = &h
		}
	case "httpupgrade":
		s.HTTPUPGRADESettings = &conf.HttpUpgradeConfig{
			Host: v.Host,
			Path: v.Path,
		}
	case "splithttp":
		s.SplitHTTPSettings = &conf.SplitHTTPConfig{
			Host: v.Host,
			Path: v.Path,
		}
	case "grpc":
		if len(v.Path) > 0 {
			if v.Path[0] == '/' {
				v.Path = v.Path[1:]
			}
		}
		multiMode := false
		if v.Type != "gun" {
			multiMode = true
		}
		s.GRPCConfig = &conf.GRPCConfig{
			InitialWindowsSize: 65536,
			HealthCheckTimeout: 20,
			MultiMode:          multiMode,
			IdleTimeout:        60,
			Authority:          v.Host,
			ServiceName:        v.Path,
		}
	case "quic":
		t := "none"
		if v.Type != "" {
			t = v.Type
		}
		s.QUICSettings = &conf.QUICConfig{
			Header:   json.RawMessage([]byte(fmt.Sprintf(`{ "type": "%s" }`, t))),
			Security: v.Host,
			Key:      v.Path,
		}
		break
	}

	if v.TLS == "tls" {
		if v.TlsFingerprint == "" {
			v.TlsFingerprint = "chrome"
		}
		s.TLSSettings = &conf.TLSConfig{
			Fingerprint: v.TlsFingerprint,
			Insecure:    allowInsecure,
		}
		if v.SNI != "" {
			s.TLSSettings.ServerName = v.SNI
		} else {
			s.TLSSettings.ServerName = v.Host
		}
		if v.ALPN != "" {
			s.TLSSettings.ALPN = &conf.StringList{v.ALPN}
		}
	}

	out.StreamSetting = s
	oset := json.RawMessage([]byte(fmt.Sprintf(`{
  "vnext": [
    {
      "address": "%s",
      "port": %v,
      "users": [
        {
          "id": "%s",
          "alterId": %v,
          "security": "%s"
        }
      ]
    }
  ]
}`, v.Address, v.Port, v.ID, v.Aid, v.Security)))
	out.Settings = &oset
	return out, nil
}

func (v *Vmess) BuildInboundDetourConfig() (*conf.InboundDetourConfig, error) {
	return nil, nil
}

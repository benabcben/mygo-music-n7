package n7car

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type proxyConfig struct {
	Host    string   `json:"host"`
	Port    int      `json:"port"`
	Exclude []string `json:"exclude"`
}

const proxyCacheTTL = 2 * time.Second

var (
	proxyMu       sync.Mutex
	proxyCached   proxyConfig
	proxyCachedAt time.Time
	fetchProxy    = defaultFetchProxy
)

func defaultFetchProxy() proxyConfig {
	if !Available() {
		return proxyConfig{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var cfg proxyConfig
	if err := Call(ctx, "proxy", nil, &cfg); err != nil {
		return proxyConfig{}
	}
	return cfg
}

func currentProxy() proxyConfig {
	now := time.Now()
	proxyMu.Lock()
	if !proxyCachedAt.IsZero() && now.Sub(proxyCachedAt) < proxyCacheTTL {
		cfg := proxyCached
		proxyMu.Unlock()
		return cfg
	}
	proxyMu.Unlock()

	cfg := fetchProxy()
	proxyMu.Lock()
	proxyCached = cfg
	proxyCachedAt = time.Now()
	proxyMu.Unlock()
	return cfg
}

// InstallSystemProxy configures http.DefaultTransport to respect Android's
// system HTTP proxy (such as Clash Meta's VPN proxy) when no env proxy is set.
func InstallSystemProxy() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.Proxy = SystemProxy
	}
}

// SystemProxy returns the proxy URL for req, preferring environment variables
// and falling back to Android's active system HTTP proxy.
func SystemProxy(req *http.Request) (*url.URL, error) {
	if u, err := http.ProxyFromEnvironment(req); err != nil || u != nil {
		return u, err
	}
	if req == nil || req.URL == nil {
		return nil, nil
	}
	cfg := currentProxy()
	if cfg.Host == "" || cfg.Port <= 0 {
		return nil, nil
	}
	host := strings.ToLower(req.URL.Hostname())
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" || host == strings.ToLower(cfg.Host) {
		return nil, nil
	}
	for _, pat := range cfg.Exclude {
		if matchProxyExclusion(host, pat) {
			return nil, nil
		}
	}
	return &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
	}, nil
}

func matchProxyExclusion(host, pattern string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	if p == "" {
		return false
	}
	if p == host {
		return true
	}
	if strings.HasPrefix(p, ".") {
		return host == p[1:] || strings.HasSuffix(host, p)
	}
	if strings.Contains(p, "*") {
		if strings.HasPrefix(p, "*.") && host == p[2:] {
			return true
		}
		if ok, err := path.Match(p, host); err == nil && ok {
			return true
		}
	}
	return false
}

package n7car

import (
	"net/http"
	"testing"
	"time"
)

// SystemProxy routes external hosts through Android's active HTTP proxy while
// bypassing loopback and hosts listed in the proxy exclusion list.
func TestSystemProxyRoutingAndExclusions(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	t.Setenv("all_proxy", "")
	realFetch := fetchProxy
	defer func() {
		fetchProxy = realFetch
		proxyMu.Lock()
		proxyCached = proxyConfig{}
		proxyCachedAt = time.Time{}
		proxyMu.Unlock()
	}()

	fetchProxy = func() proxyConfig {
		return proxyConfig{
			Host:    "127.146.94.142",
			Port:    40959,
			Exclude: []string{"*zhihu.com", "*.local", "172.2*", "192.168.*", "localhost"},
		}
	}
	proxyMu.Lock()
	proxyCachedAt = time.Time{}
	proxyMu.Unlock()

	cases := []struct {
		url  string
		want string
	}{
		{"https://api.github.com/search/repositories", "http://127.146.94.142:40959"},
		{"https://github.com/egoist.png", "http://127.146.94.142:40959"},
		{"https://www.zhihu.com/question/1", ""},
		{"https://zhihu.com/", ""},
		{"http://printer.local/status", ""},
		{"http://172.20.10.3:5555/", ""},
		{"http://192.168.1.1/", ""},
		{"http://127.0.0.1:8080/", ""},
		{"http://localhost:8080/", ""},
	}
	for _, tc := range cases {
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := SystemProxy(req)
		if err != nil {
			t.Fatalf("SystemProxy(%s): %v", tc.url, err)
		}
		gotStr := ""
		if got != nil {
			gotStr = got.String()
		}
		if gotStr != tc.want {
			t.Errorf("SystemProxy(%s) = %q, want %q", tc.url, gotStr, tc.want)
		}
	}
}

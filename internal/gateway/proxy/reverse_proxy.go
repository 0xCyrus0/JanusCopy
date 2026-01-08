package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewProxy creates a standard Reverse Proxy to a specific target
func NewProxy(targetHost string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetHost)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(url)

	// The Director modifies the original request for the target
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Essential: Overwrite the Host header.
		// Many backend services (like Nginx/Cloudflare) reject requests
		// if the Host header doesn't match their domain.
		req.Host = url.Host

		// Optional: Add headers to trace the request source
		req.Header.Set("X-Proxy-Source", "Go-Gateway")
	}

	// Custom Error Handler: What happens if the backend is down?
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy Error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("502 Bad Gateway - Service might be down"))
	}

	return proxy, nil
}

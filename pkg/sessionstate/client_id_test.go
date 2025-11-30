package sessionstate

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStdioClientIDExtractor(t *testing.T) {
	clientID := StdioClientIDExtractor(nil)
	require.Equal(t, DefaultClientID, clientID)
}

func TestHTTPClientIDExtractor_Authorization(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token123")

	clientID := HTTPClientIDExtractor(req)
	require.NotEqual(t, DefaultClientID, clientID)
	require.Contains(t, clientID, "auth-")

	// Same token should give same client ID
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.Header.Set("Authorization", "Bearer token123")
	clientID2 := HTTPClientIDExtractor(req2)
	require.Equal(t, clientID, clientID2)

	// Different token should give different client ID
	req3, _ := http.NewRequest("GET", "/", nil)
	req3.Header.Set("Authorization", "Bearer different-token")
	clientID3 := HTTPClientIDExtractor(req3)
	require.NotEqual(t, clientID, clientID3)
}

func TestHTTPClientIDExtractor_SessionHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-MCP-Session-ID", "session-abc-123")

	clientID := HTTPClientIDExtractor(req)
	require.NotEqual(t, DefaultClientID, clientID)
	require.Contains(t, clientID, "session-")
}

func TestHTTPClientIDExtractor_Cookie(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "mcp-session",
		Value: "cookie-value-123",
	})

	clientID := HTTPClientIDExtractor(req)
	require.NotEqual(t, DefaultClientID, clientID)
	require.Contains(t, clientID, "cookie-")
}

func TestHTTPClientIDExtractor_IPAddress(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "TestAgent/1.0")

	clientID := HTTPClientIDExtractor(req)
	require.NotEqual(t, DefaultClientID, clientID)
	require.Contains(t, clientID, "ip-ua-")
}

func TestHTTPClientIDExtractor_XForwardedFor(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	req.Header.Set("User-Agent", "TestAgent/1.0")

	clientID := HTTPClientIDExtractor(req)
	require.NotEqual(t, DefaultClientID, clientID)

	// Should use the first IP in X-Forwarded-For
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.3")
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	clientID2 := HTTPClientIDExtractor(req2)

	require.Equal(t, clientID, clientID2)
}

func TestHTTPClientIDExtractor_XRealIP(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.Header.Set("User-Agent", "TestAgent/1.0")

	clientID := HTTPClientIDExtractor(req)
	require.NotEqual(t, DefaultClientID, clientID)
}

func TestHTTPClientIDExtractor_Priority(t *testing.T) {
	// Authorization should take priority over everything
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer priority-token")
	req.Header.Set("X-MCP-Session-ID", "session-123")
	req.AddCookie(&http.Cookie{Name: "mcp-session", Value: "cookie-123"})
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	clientID := HTTPClientIDExtractor(req)
	require.Contains(t, clientID, "auth-")
}

func TestHTTPClientIDExtractor_NoIdentification(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)

	clientID := HTTPClientIDExtractor(req)
	require.Equal(t, DefaultClientID, clientID)
}

func TestHTTPClientIDExtractor_NotHTTPRequest(t *testing.T) {
	clientID := HTTPClientIDExtractor("not an http request")
	require.Equal(t, DefaultClientID, clientID)
}

func TestTokenToClientID(t *testing.T) {
	token1 := "ghp_token123"
	token2 := "ghp_token456"

	clientID1 := TokenToClientID(token1)
	clientID2 := TokenToClientID(token2)

	// Should be different
	require.NotEqual(t, clientID1, clientID2)

	// Same token should give same ID
	clientID1Again := TokenToClientID(token1)
	require.Equal(t, clientID1, clientID1Again)

	// Empty token should return default
	clientID3 := TokenToClientID("")
	require.Equal(t, DefaultClientID, clientID3)
}

func TestHashClientIdentifier(t *testing.T) {
	// Same input should give same output
	hash1 := hashClientIdentifier("test", "value")
	hash2 := hashClientIdentifier("test", "value")
	require.Equal(t, hash1, hash2)

	// Different inputs should give different outputs
	hash3 := hashClientIdentifier("test", "different-value")
	require.NotEqual(t, hash1, hash3)

	// Empty input should return default
	hash4 := hashClientIdentifier("test", "")
	require.Equal(t, DefaultClientID, hash4)

	// Should include prefix
	hash5 := hashClientIdentifier("prefix", "value")
	require.Contains(t, hash5, "prefix-")
}

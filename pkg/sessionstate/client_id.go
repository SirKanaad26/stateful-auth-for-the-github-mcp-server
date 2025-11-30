package sessionstate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// ClientIDExtractor defines a function that extracts a client ID from context.
// Different deployment modes (stdio, HTTP, etc.) can provide different extractors.
type ClientIDExtractor func(interface{}) string

// DefaultClientID is used when no specific client identification is available.
// This maintains backward compatibility with stdio mode.
const DefaultClientID = "default"

// StdioClientIDExtractor returns a fixed client ID for stdio mode.
// In stdio mode, there's only one client per process.
func StdioClientIDExtractor(_ interface{}) string {
	return DefaultClientID
}

// HTTPClientIDExtractor extracts client ID from HTTP request.
// It uses the Authorization header as the basis for client identification.
// This ensures each unique token gets its own session.
func HTTPClientIDExtractor(ctx interface{}) string {
	if req, ok := ctx.(*http.Request); ok {
		return ExtractClientIDFromHTTPRequest(req)
	}
	return DefaultClientID
}

// ExtractClientIDFromHTTPRequest derives a client ID from an HTTP request.
// Strategy:
// 1. Use Authorization header (hashed for privacy)
// 2. Fall back to session cookie if available
// 3. Fall back to IP address (least secure)
// 4. Fall back to default
func ExtractClientIDFromHTTPRequest(req *http.Request) string {
	// Strategy 1: Authorization header (most common for MCP)
	if auth := req.Header.Get("Authorization"); auth != "" {
		// Hash the token to avoid logging sensitive data
		return hashClientIdentifier("auth", auth)
	}

	// Strategy 2: Custom MCP-Session-ID header (if implemented by client)
	if sessionID := req.Header.Get("X-MCP-Session-ID"); sessionID != "" {
		return hashClientIdentifier("session", sessionID)
	}

	// Strategy 3: Session cookie
	if cookie, err := req.Cookie("mcp-session"); err == nil {
		return hashClientIdentifier("cookie", cookie.Value)
	}

	// Strategy 4: IP address + User-Agent (least secure, but better than nothing)
	ipAddr := getClientIP(req)
	userAgent := req.Header.Get("User-Agent")
	if ipAddr != "" || userAgent != "" {
		return hashClientIdentifier("ip-ua", ipAddr+":"+userAgent)
	}

	// Fall back to default
	return DefaultClientID
}

// hashClientIdentifier creates a deterministic hash of the identifier.
// This provides privacy (don't log raw tokens) while maintaining uniqueness.
func hashClientIdentifier(prefix, identifier string) string {
	if identifier == "" {
		return DefaultClientID
	}

	hash := sha256.Sum256([]byte(identifier))
	hashStr := hex.EncodeToString(hash[:])

	// Return first 16 chars of hash for readability in logs
	return fmt.Sprintf("%s-%s", prefix, hashStr[:16])
}

// getClientIP extracts the client IP address from the request.
// It handles X-Forwarded-For and X-Real-IP headers.
func getClientIP(req *http.Request) string {
	// Check X-Forwarded-For header
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return req.RemoteAddr
}

// TokenToClientID converts an authorization token to a client ID.
// This is a convenience function for non-HTTP contexts.
func TokenToClientID(token string) string {
	if token == "" {
		return DefaultClientID
	}
	return hashClientIdentifier("token", token)
}

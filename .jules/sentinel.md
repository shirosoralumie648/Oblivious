## 2024-10-07 - Insecure IP extraction from X-Forwarded-For header
**Vulnerability:** The application blindly extracts the first (left-most) IP address from the `X-Forwarded-For` header (e.g. `ips[0]`). This allows IP spoofing because attackers can manually inject their own `X-Forwarded-For` header in requests, and the trusted proxy simply appends the real client IP to it.
**Learning:** Using `[0]` allows IP spoofing. The Go backend securely extracts client IPs from the `X-Forwarded-For` header by taking the right-most IP (`parts[len(parts)-1]`), which is appended by the trusted edge proxy.
**Prevention:** Always extract the last (right-most) IP from the comma-separated `X-Forwarded-For` list to guarantee authenticity from trusted proxies, and fallback to `r.RemoteAddr` if absent.

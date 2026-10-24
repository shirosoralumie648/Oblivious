## 2025-02-23 - Prevent IP Spoofing via X-Forwarded-For Header
**Vulnerability:** The Go backend was extracting the left-most IP address (index 0) from the `X-Forwarded-For` header. This allowed attackers to spoof their IP address by injecting a fake IP before the request hits the trusted edge proxy.
**Learning:** In architectures with a trusted edge proxy that appends the client IP to `X-Forwarded-For`, only the right-most IP (appended by the proxy) can be trusted. The left-most IP is user-supplied and easily spoofable.
**Prevention:** Always extract the right-most IP (`parts[len(parts)-1]`) from `X-Forwarded-For` when relying on it for auditing, rate limiting, or security decisions, rather than blindly taking the first element.

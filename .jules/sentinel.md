## 2026-09-20 - IP Spoofing via X-Forwarded-For Header
**Vulnerability:** The application was extracting the first IP address from the `X-Forwarded-For` header to determine the client IP. This allows an attacker to spoof their IP address by prepending a fake IP to the header, bypassing IP-based restrictions and poisoning audit logs.
**Learning:** Edge proxies append the client IP to the end of the `X-Forwarded-For` list. The rightmost IP is the only one guaranteed to be appended by the trusted proxy.
**Prevention:** Always extract the rightmost IP address (`parts[len(parts)-1]`) from the `X-Forwarded-For` header instead of the first one (`parts[0]`) when behind a trusted proxy.

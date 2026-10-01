## 2024-06-11 - IP Spoofing via X-Forwarded-For
**Vulnerability:** Extracted the first IP from `X-Forwarded-For` for client identification.
**Learning:** `X-Forwarded-For` is a comma-separated list of IPs. When multiple proxies are in place, the first IP is the original client IP, but it can be spoofed by the client. The right-most IP is the one appended by the trusted edge proxy, and thus the most reliable indicator of the client IP as seen by our infrastructure.
**Prevention:** In a trusted reverse-proxy environment, use the right-most IP (`parts[len(parts)-1]`) instead of the first IP (`parts[0]`) to securely extract the client IP.

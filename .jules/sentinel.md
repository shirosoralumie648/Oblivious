## 2024-10-02 - IP Spoofing via X-Forwarded-For
**Vulnerability:** The application extracts the first IP address from the `X-Forwarded-For` header (`parts[0]`).
**Learning:** This is a classic IP spoofing vulnerability. An attacker can set an arbitrary `X-Forwarded-For` header, and since proxies append to this header, the first IP address will be the attacker-controlled one. The correct approach when behind a trusted reverse proxy is to extract the right-most IP address (the one appended by the proxy).
**Prevention:** Always extract the right-most IP address from `X-Forwarded-For` when behind a single trusted proxy, or validate against a list of trusted proxy IPs.

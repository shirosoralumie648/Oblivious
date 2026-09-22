## 2024-05-19 - Insecure Client IP Extraction
**Vulnerability:** IP Spoofing via X-Forwarded-For Header
**Learning:** The application extracts the client IP address from the `X-Forwarded-For` header by taking the first element (`parts[0]`). Because clients can arbitrarily append to this header or set it themselves before it hits the trusted edge proxy, the first IP address is fully attacker-controlled. The Go backend securely extracts client IPs from the `X-Forwarded-For` header by taking the right-most IP (`parts[len(parts)-1]`), which is appended by the trusted edge proxy. Do not use the first index (`[0]`), as it introduces an IP spoofing vulnerability.
**Prevention:** Always trust the right-most IP address (or the one appended by the trusted proxy) in the `X-Forwarded-For` chain.

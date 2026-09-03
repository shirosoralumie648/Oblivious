
## 2024-09-03 - IP Spoofing via X-Forwarded-For Header
**Vulnerability:** Client IP extraction logic used the left-most IP from the `X-Forwarded-For` header. Because clients can inject arbitrary IPs in this header, the left-most IP is untrusted, enabling attackers to spoof their IP address.
**Learning:** When operating behind a trusted proxy, the proxy appends the true client IP to the right side of the header. Extracting the first index (`[0]`) introduces an IP spoofing vulnerability.
**Prevention:** Always extract the right-most IP (`parts[len(parts)-1]`) from the `X-Forwarded-For` header in Go backend code, as this value is reliably added by the edge proxy.

## 2025-05-24 - IP Spoofing via X-Forwarded-For
**Vulnerability:** The application was extracting the first IP address from the `X-Forwarded-For` header.
**Learning:** Attackers can easily forge the `X-Forwarded-For` header. Trusted proxies append the actual client IP to the end of the list. Taking the first IP allows the attacker to control the extracted IP.
**Prevention:** Always extract the right-most IP from the `X-Forwarded-For` header (e.g., `parts[len(parts)-1]`) when using a trusted edge proxy.

## 2024-05-24 - Fix IP spoofing vulnerability in X-Forwarded-For extraction
**Vulnerability:** The application was extracting client IP addresses from the `X-Forwarded-For` header by taking the first element (`parts[0]`). This allowed an attacker to easily spoof their IP address by prefixing the header with an arbitrary IP, bypassing IP-based rate limiting or security controls.
**Learning:** Edge proxies append the true client IP to the end of the `X-Forwarded-For` chain. Thus, taking the first element trusts potentially user-supplied data rather than the data provided by the trusted proxy.
**Prevention:** Always extract the right-most element (`parts[len(parts)-1]`) of the `X-Forwarded-For` header when extracting the client IP behind a trusted proxy.

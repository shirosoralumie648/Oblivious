## 2024-10-14 - Fix IP Spoofing vulnerability in X-Forwarded-For parsing
**Vulnerability:** The application was extracting the first IP address from the `X-Forwarded-For` header (`parts[0]`) to determine the client IP.
**Learning:** Extracting the first IP address (`parts[0]`) is vulnerable to IP spoofing because untrusted clients can prepend arbitrary IP addresses to the `X-Forwarded-For` header. The right-most IP address is the one appended by the trusted edge proxy, and should be used instead.
**Prevention:** Always extract the right-most IP address (`parts[len(parts)-1]`) from the `X-Forwarded-For` header when running behind a trusted edge proxy to ensure the client IP is accurate.

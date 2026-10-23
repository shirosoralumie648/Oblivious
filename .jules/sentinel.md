## 2026-10-23 - Fix IP Spoofing in X-Forwarded-For Extraction
**Vulnerability:** The application was extracting the client IP from the `X-Forwarded-For` header using the first index (`parts[0]`). This allows an attacker to trivially spoof their IP address by injecting a custom `X-Forwarded-For` header before the request hits the proxy.
**Learning:** In a trusted proxy architecture, the edge proxy appends the true client IP to the end of the `X-Forwarded-For` header. Using the first value trusts user input blindly.
**Prevention:** Always extract the right-most IP address (`parts[len(parts)-1]`) appended by the trusted edge proxy when resolving client IPs from `X-Forwarded-For`.

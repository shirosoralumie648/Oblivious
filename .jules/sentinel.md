## 2025-02-28 - X-Forwarded-For IP Spoofing
**Vulnerability:** The application was extracting the left-most IP address from the `X-Forwarded-For` header.
**Learning:** The left-most IP address is provided by the client and can be easily spoofed. The right-most IP address is appended by the trusted proxy and should be used instead.
**Prevention:** Always extract the right-most IP address (`parts[len(parts)-1]`) when dealing with the `X-Forwarded-For` header in Go backend code.

## 2024-05-20 - IP Spoofing in X-Forwarded-For
**Vulnerability:** The application was extracting the left-most IP address from the `X-Forwarded-For` header.
**Learning:** The left-most IP can be easily spoofed by malicious clients by appending their own `X-Forwarded-For` header before the request reaches the trusted edge proxy. The proxy appends the true client IP to the right.
**Prevention:** Always extract the right-most IP address from the `X-Forwarded-For` header (`parts[len(parts)-1]`) to ensure the IP is the one appended by the trusted infrastructure.

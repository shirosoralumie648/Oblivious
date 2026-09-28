## 2024-10-10 - Fix X-Forwarded-For IP Spoofing
**Vulnerability:** The application was using the leftmost IP address from the X-Forwarded-For header to determine the client IP.
**Learning:** This is vulnerable to IP spoofing because clients can inject arbitrary IPs into the header before it reaches the trusted proxy. The trusted proxy appends the real client IP to the end of the list.
**Prevention:** Always use the rightmost IP address from the X-Forwarded-For header when extracting the client IP behind a trusted proxy.

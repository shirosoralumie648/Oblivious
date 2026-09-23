## 2024-05-31 - [IP Spoofing Vulnerability]
**Vulnerability:** The application extracted the client IP address from the `X-Forwarded-For` header by taking the first element of the list instead of the right-most element.
**Learning:** `X-Forwarded-For` headers can be spoofed by clients by passing their own header, causing the edge proxy to append the true IP address to the end. Selecting index `0` accesses the client-provided, potentially spoofed IP, rather than the trusted proxy-appended IP.
**Prevention:** When extracting IP addresses from `X-Forwarded-For`, always take the right-most IP address (or the right-most IP from a trusted proxy if multiple reverse proxies are chained).

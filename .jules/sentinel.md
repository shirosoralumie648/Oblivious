## 2025-01-14 - Fix IP Spoofing Vulnerability in X-Forwarded-For Parsing
**Vulnerability:** IP spoofing vulnerability in `X-Forwarded-For` header parsing. The backend extracted the client IP from the left-most IP (`parts[0]`) instead of the right-most IP (`parts[len(parts)-1]`).
**Learning:** The left-most IP can be easily spoofed by the client sending a fake `X-Forwarded-For` header. The right-most IP is securely appended by the trusted edge proxy, making it reliable.
**Prevention:** Always use the right-most IP when extracting client IP from `X-Forwarded-For` headers in a trusted proxy environment.

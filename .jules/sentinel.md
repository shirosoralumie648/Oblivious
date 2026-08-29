## 2026-08-29 - [IP Spoofing Vulnerability via X-Forwarded-For]
**Vulnerability:** Extracted the first IP from X-Forwarded-For header, allowing IP spoofing by attackers sending custom headers.
**Learning:** Trusted edge proxies append the client IP to the end of the X-Forwarded-For header. The first IP can be easily manipulated by attackers.
**Prevention:** Always extract the right-most (last) IP from the X-Forwarded-For header to ensure the IP was appended by the trusted proxy.

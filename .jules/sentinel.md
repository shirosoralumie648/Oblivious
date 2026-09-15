## 2025-02-14 - IP Spoofing via X-Forwarded-For
**Vulnerability:** IP spoofing vulnerability where `X-Forwarded-For` header was incorrectly parsed using the left-most (first) IP address. Since the leftmost IP can be easily spoofed by the client, using it for rate limiting, auditing, or security checks is insecure.
**Learning:** The right-most IP in the `X-Forwarded-For` header is the one appended by the last trusted proxy before the application, making it the reliable client IP.
**Prevention:** Always extract the right-most IP from `X-Forwarded-For` (`parts[len(parts)-1]`) to prevent IP spoofing, and ensure the edge proxy strips or handles existing headers correctly.

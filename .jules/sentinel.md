## 2024-05-24 - Fix IP spoofing via X-Forwarded-For
**Vulnerability:** Extracted the first (left-most) IP address from the X-Forwarded-For header, which can be spoofed by the client.
**Learning:** The right-most IP is typically appended by the trusted proxy directly in front of the application. Using the left-most IP introduces an IP spoofing vulnerability.
**Prevention:** When extracting client IP from X-Forwarded-For, always use the right-most IP appended by the trusted proxy, not the left-most which is easily spoofable by the client.

## 2026-09-04 - Fix IP Spoofing via X-Forwarded-For
**Vulnerability:** IP Spoofing due to incorrect extraction of the client IP address from the X-Forwarded-For header. The code took the first (left-most) IP address from the header, which can be easily forged by clients.
**Learning:** In proxy configurations, the first IP address is untrustworthy because any client can append to the header. The trusted edge proxy appends the true client IP to the end of the list. Therefore, the right-most IP should be extracted.
**Prevention:** Always extract the right-most IP address (or the one immediately before trusted proxies if chaining) when relying on the X-Forwarded-For header for security purposes like auditing or rate limiting.

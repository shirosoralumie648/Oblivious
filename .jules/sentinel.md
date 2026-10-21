## 2025-02-14 - Prevent IP Spoofing via X-Forwarded-For
**Vulnerability:** The Go backend was using the first (left-most) IP address from the X-Forwarded-For header to determine the client IP (`parts[0]`).
**Learning:** The left-most IP is unverified and can be spoofed by the client. The right-most IP is the one reliably appended by the trusted edge proxy.
**Prevention:** Always extract the right-most IP address (`parts[len(parts)-1]`) from the X-Forwarded-For header when determining client IP in a proxied environment.

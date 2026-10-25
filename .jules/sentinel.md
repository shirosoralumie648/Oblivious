## 2025-01-25 - Fix IP spoofing vulnerability in X-Forwarded-For parsing
**Vulnerability:** IP spoofing via `X-Forwarded-For` header because the application extracted the left-most IP rather than the right-most IP.
**Learning:** The left-most IP in `X-Forwarded-For` is controlled by the client and can be easily spoofed. The trusted edge proxy appends the true client IP to the right side of the list.
**Prevention:** Always extract the right-most IP (`parts[len(parts)-1]`) from the `X-Forwarded-For` header when behind a trusted edge proxy.

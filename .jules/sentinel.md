## 2024-10-20 - Fix X-Forwarded-For IP Spoofing
**Vulnerability:** X-Forwarded-For extracted the first (left-most) IP address (`parts[0]`), allowing clients to spoof their IP by sending a fabricated header.
**Learning:** Proxy servers append the actual client IP to the end of the X-Forwarded-For header. By reading the first IP, we trust user-provided input instead of proxy-provided input.
**Prevention:** Always extract the right-most IP address (`parts[len(parts)-1]`) from X-Forwarded-For when behind a trusted proxy, as this is the only IP the proxy guarantees.

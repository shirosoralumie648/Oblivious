## 2025-02-28 - X-Forwarded-For IP Spoofing
**Vulnerability:** IP Spoofing via X-Forwarded-For Header
**Learning:** Extracting the left-most IP from X-Forwarded-For headers allows malicious clients to spoof their IP address. The trusted edge proxy appends to the right.
**Prevention:** Always extract the right-most IP address (parts[len(parts)-1]) when reading X-Forwarded-For headers behind a trusted proxy.

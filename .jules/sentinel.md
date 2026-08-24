## 2025-05-24 - Extracting Right-Most IP for Anti-Spoofing
**Vulnerability:** The codebase incorrectly extracted the left-most IP address from the X-Forwarded-For header to determine the client IP.
**Learning:** Malicious users can spoof the X-Forwarded-For header by prepending fake IPs. The left-most IP is untrusted, while the right-most IP is appended by the trusted edge proxy.
**Prevention:** Always extract the right-most IP (e.g., `parts[len(parts)-1]`) from the X-Forwarded-For header when relying on a trusted edge proxy.

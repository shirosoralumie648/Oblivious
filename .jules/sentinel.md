## 2024-05-23 - Fix IP Spoofing in X-Forwarded-For Extraction
**Vulnerability:** IP spoofing vulnerability due to extracting the left-most IP from `X-Forwarded-For`.
**Learning:** In proxy environments, the trusted edge proxy appends the real client IP to the right side of the list. Attackers can inject a fake IP on the left, leading to spoofed IP logging.
**Prevention:** Always extract the right-most IP from `X-Forwarded-For` to ensure the IP logged is from the trusted proxy.

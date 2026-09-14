## 2024-11-20 - IP Spoofing Vulnerability in X-Forwarded-For
**Vulnerability:** Client IPs were being extracted from the left-most index (`parts[0]`) of the `X-Forwarded-For` header.
**Learning:** This is vulnerable to IP spoofing, as the left-most IP can be arbitrarily set by the client. The trusted edge proxy appends the true client IP to the right end of the header.
**Prevention:** Always extract the right-most IP (`parts[len(parts)-1]`) from the `X-Forwarded-For` header when behind a trusted proxy.

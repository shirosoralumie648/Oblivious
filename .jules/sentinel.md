## 2024-09-12 - Fix IP Spoofing via X-Forwarded-For
**Vulnerability:** IP spoofing vulnerability due to incorrectly reading the left-most IP from `X-Forwarded-For` header.
**Learning:** The trusted proxy appends to the right, meaning the left-most IP can be spoofed by a client sending a fake header.
**Prevention:** Always extract the right-most IP (`parts[len(parts)-1]`) appended by the trusted proxy.

## 2024-05-24 - IP Spoofing via X-Forwarded-For
**Vulnerability:** Extracting client IP by reading the first part of the X-Forwarded-For header (`parts[0]`).
**Learning:** This introduces an IP spoofing vulnerability because an attacker can arbitrarily set the X-Forwarded-For header to contain a spoofed IP as the first element. The trusted edge proxy appends the real IP at the end of the list.
**Prevention:** Always take the right-most IP (`parts[len(parts)-1]`) from the X-Forwarded-For header, which is the one appended by the trusted infrastructure.

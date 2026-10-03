## 2025-05-18 - Fix IP Spoofing in X-Forwarded-For Parsing
**Vulnerability:** IP Spoofing vulnerability due to using the first element of `X-Forwarded-For` header. An attacker could spoof their IP address by injecting a fake IP in the header.
**Learning:** In environments behind trusted reverse proxies, the right-most IP is the one appended by the proxy and should be trusted. The first IP is user-provided and can be spoofed.
**Prevention:** Always extract the right-most IP from `X-Forwarded-For` header or use a robust library if the proxy setup is more complex.

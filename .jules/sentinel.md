## 2026-08-21 - Fix IP Spoofing via X-Forwarded-For Header
**Vulnerability:** IP Spoofing vulnerability in client IP extraction where the application took the first (left-most) IP from the `X-Forwarded-For` header. This allows malicious clients to trivially spoof their IP by sending a fabricated header (e.g., `X-Forwarded-For: spoofed_ip, real_ip`).
**Learning:** Existed due to naive parsing of `X-Forwarded-For` header `strings.Split(forwarded, ",")[0]`, trusting user-provided left-most IPs in a proxy chain rather than the right-most IP appended by the trusted edge proxy.
**Prevention:** Always extract the right-most IP from `X-Forwarded-For` if relying on a single trusted proxy layer, or iterate from the right discarding trusted proxies based on a whitelist.

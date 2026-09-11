## 2024-05-15 - IP Spoofing via X-Forwarded-For

**Vulnerability:** Extracting client IP addresses using the first element (`[0]`) of the `X-Forwarded-For` header allows IP spoofing, as attackers can easily inject an arbitrary IP address as the first element. The trusted proxy typically appends the actual client IP at the end of the list.
**Learning:** In proxy environments, `X-Forwarded-For` entries appended by the proxy are at the rightmost position. Using the leftmost position (`[0]`) blindly trusts client-provided input.
**Prevention:** To securely extract the client IP, use the rightmost element (`parts[len(parts)-1]`) appended by the trusted proxy.

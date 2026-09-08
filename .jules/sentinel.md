## 2024-05-18 - Client IP Extraction Vulnerability (IP Spoofing)
**Vulnerability:** Client IP extraction from `X-Forwarded-For` header relied on the first element of the comma-separated list.
**Learning:** The first IP in the `X-Forwarded-For` chain is untrusted, user-controlled input and can be trivially spoofed by the client sending an `X-Forwarded-For` header in their request. The edge proxy appends the true client IP to the *end* of the list.
**Prevention:** Always extract the right-most IP from the `X-Forwarded-For` header to ensure it is the IP appended by the trusted edge proxy, rather than an IP spoofed by the user.

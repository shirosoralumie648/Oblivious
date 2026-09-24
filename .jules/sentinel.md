## 2024-05-18 - Fix IP Spoofing in X-Forwarded-For Parsing
**Vulnerability:** Extracted the first IP in the `X-Forwarded-For` header `strings.Split(forwarded, ",")[0]`.
**Learning:** The first index is client-provided and easily spoofed by attackers to bypass rate limits or pollute audit logs.
**Prevention:** Always extract the right-most IP appended by a trusted edge proxy (`parts[len(parts)-1]`).

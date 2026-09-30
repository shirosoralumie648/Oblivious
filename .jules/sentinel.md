## 2024-05-24 - Fix IP Spoofing vulnerability in X-Forwarded-For
**Vulnerability:** Extracted IP address from `X-Forwarded-For` by taking the first element of the list, which allows clients to spoof their IP address by injecting a fake IP in the header.
**Learning:** `X-Forwarded-For` can be modified by the client. The trusted edge proxy appends the real IP address to the end of the list. Therefore, taking the first element allows for spoofing.
**Prevention:** Always extract the right-most IP from `X-Forwarded-For` by taking `parts[len(parts)-1]`.

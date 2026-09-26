## 2024-09-26 - [IP Spoofing Vulnerability via X-Forwarded-For]
**Vulnerability:** The application was extracting the client IP address from the first element of the `X-Forwarded-For` header (`parts[0]`), which allows attackers to spoof their IP address by passing a fake value in the header.
**Learning:** Trusted proxies append the real client IP to the end of the `X-Forwarded-For` header. Taking the first element is unsafe because it can be manipulated by the client before reaching the proxy.
**Prevention:** Always extract the right-most (last) element of the `X-Forwarded-For` header (`parts[len(parts)-1]`) to obtain the true client IP appended by the trusted proxy.

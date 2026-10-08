
## 2024-05-24 - X-Forwarded-For IP Spoofing
**Vulnerability:** IP spoofing via `X-Forwarded-For` header. The application was taking the first IP address `parts[0]` from the header.
**Learning:** Users can send a spoofed IP in the header, which gets prepended to the actual list by the proxy.
**Prevention:** Always extract the right-most IP address `parts[len(parts)-1]` which is appended by the trusted proxy.

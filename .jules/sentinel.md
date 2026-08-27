## 2024-08-27 - IP Spoofing via X-Forwarded-For
**Vulnerability:** Extracting client IPs from the left-most value of the `X-Forwarded-For` header allows IP spoofing.
**Learning:** The left-most value can be set by the client. The trusted edge proxy appends the true IP to the right side of the list.
**Prevention:** Always take the right-most IP (`parts[len(parts)-1]`) from the `X-Forwarded-For` header.

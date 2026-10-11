
## 2024-10-11 - Prevent IP Spoofing by using Right-Most IP in X-Forwarded-For
**Vulnerability:** IP Spoofing risk due to using the left-most IP address from the `X-Forwarded-For` header for client IP identification.
**Learning:** The left-most IP can be forged by attackers in the original HTTP request, while trusted proxies append subsequent IPs to the right. Taking `parts[0]` uses the potentially malicious user-provided IP, whereas taking `parts[len(parts)-1]` ensures we use the IP appended by our trusted edge proxy.
**Prevention:** Always extract the right-most IP address when reading the `X-Forwarded-For` header for security-sensitive operations.

## 2024-05-01 - IP Spoofing Vulnerability in X-Forwarded-For
**Vulnerability:** The application was extracting the left-most IP address from the X-Forwarded-For header to determine the client IP.
**Learning:** This is vulnerable to IP spoofing because users can append arbitrary IP addresses to the left side of the header. The trusted edge proxy appends the true client IP to the right side.
**Prevention:** Always extract the right-most IP address from the X-Forwarded-For header in a proxy environment.

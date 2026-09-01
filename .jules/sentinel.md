## 2024-10-31 - Fix IP spoofing vulnerability in X-Forwarded-For parsing
**Vulnerability:** The application was extracting the first IP address from the X-Forwarded-For header (`parts[0]`) to determine the client IP.
**Learning:** This is vulnerable to IP spoofing because attackers can prepend fake IP addresses to the X-Forwarded-For header. The trusted edge proxy appends the real client IP to the end of the list.
**Prevention:** Always extract the right-most IP address (`parts[len(parts)-1]`) from the X-Forwarded-For header when relying on a trusted edge proxy to append the true client IP.

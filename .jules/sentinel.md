## 2024-05-15 - [Critical] IP Spoofing via X-Forwarded-For
**Vulnerability:** The application was extracting the left-most IP address (index 0) from the X-Forwarded-For header to determine the client IP.
**Learning:** The left-most IP in the X-Forwarded-For header can be spoofed by an attacker because any intermediate proxy simply appends to whatever header it receives. The edge proxy typically appends the real client IP to the end of the list. Therefore, the right-most IP is the only reliable and secure one to use.
**Prevention:** Always extract the right-most IP (`parts[len(parts)-1]`) from the X-Forwarded-For header when relying on an edge proxy to identify the client IP.

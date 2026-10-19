## 2024-05-24 - IP Spoofing vulnerability via X-Forwarded-For
**Vulnerability:** The application extracts the client IP address from the X-Forwarded-For header by taking the first (left-most) IP address.
**Learning:** The left-most IP address in the X-Forwarded-For header can be spoofed by an attacker because they can prepend an arbitrary IP address to the header.
**Prevention:** The correct way to extract the client IP address from the X-Forwarded-For header is to take the right-most IP address, which is appended by the trusted edge proxy.

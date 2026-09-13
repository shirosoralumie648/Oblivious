## 2024-09-13 - [IP Spoofing via X-Forwarded-For]
**Vulnerability:** X-Forwarded-For was parsing the first IP in the list (`[0]`), allowing IP spoofing by attackers sending forged headers before the real proxy appended the client IP.
**Learning:** In standard reverse proxy configurations (e.g. AWS ALB, Nginx), the trusted proxy appends the real client IP to the end of the X-Forwarded-For list.
**Prevention:** Always extract the right-most IP (`[len(parts)-1]`) from X-Forwarded-For when behind a single trusted proxy layer.

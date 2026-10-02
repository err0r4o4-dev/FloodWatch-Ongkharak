import os
import re

path = r"d:\!Project\FloodWatch-Ongkharak(Web_Application)\apps\backend\internal\http\router\router.go"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# Add import
content = re.sub(
    r'("github.com/redis/go-redis/v9")',
    r'\1\n\t"github.com/prometheus/client_golang/prometheus/promhttp"',
    content
)

# Add /metrics route
content = re.sub(
    r'(router\.Get\("/ready", handler\.NewReadinessHandler\(dependencies\.Readiness\)\.ServeHTTP\))',
    r'\1\n\trouter.Get("/metrics", promhttp.Handler().ServeHTTP)',
    content
)

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

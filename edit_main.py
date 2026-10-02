import os

path = r"d:\!Project\FloodWatch-Ongkharak(Web_Application)\apps\backend\cmd\api\main.go"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

import re

# Insert import
content = re.sub(r'("github.com/floodwatch-ongkharak/backend/internal/postgres/alertrepo"\s+)', r'\1"github.com/floodwatch-ongkharak/backend/internal/notification"\n\t"github.com/floodwatch-ongkharak/backend/internal/postgres/notificationrepo"\n\t', content)

# Insert NotificationService into router.Dependencies
content = re.sub(r'(AdminService:\s+adminService,)', r'\1\n\t\t\t\tNotificationService: notificationService,', content)

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

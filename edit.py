import os
path = r"d:\!Project\FloodWatch-Ongkharak(Web_Application)\apps\backend\internal\postgres\alertrepo\repository.go"
with open(path, "r", encoding="utf-8") as f: content = f.read()
old_sql = "INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, evaluated_at)\n\t\t\tVALUES ($1,'alert.created',$2,$3,'ALERT',$4,CURRENT_TIMESTAMP)`, uuid.NewString(), item.TriggeredAt, item.SourceType, item.ID);"
new_sql = "INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, evaluated_at, payload)\n\t\t\tVALUES ($1,'alert.created',$2,$3,'ALERT',$4,CURRENT_TIMESTAMP, jsonb_build_object('title', $5::text, 'message', $6::text))`, uuid.NewString(), item.TriggeredAt, item.SourceType, item.ID, item.Title, item.Message);"
content = content.replace(old_sql, new_sql)
with open(path, "w", encoding="utf-8") as f: f.write(content)

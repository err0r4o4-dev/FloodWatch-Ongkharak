import os

path = r"d:\!Project\FloodWatch-Ongkharak(Web_Application)\apps\backend\cmd\api\main.go"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

import re

new_dispatcher = """\tdispatcher := outbox.New(postgresPool, func(ctx context.Context, record outbox.Record) error {
\t\terr := publisher.Publish(ctx, realtime.Event{
\t\t\tEventID: record.ID, EventType: realtime.EventType(record.EventType), OccurredAt: record.OccurredAt,
\t\t\tPublishedAt: time.Now().UTC(), SourceType: realtime.SourceType(record.SourceType),
\t\t\tResourceType: realtime.ResourceType(record.ResourceType), ResourceID: record.ResourceID,
\t\t\tPayloadVersion: record.PayloadVersion, Payload: record.Payload,
\t\t})
\t\tif err != nil {
\t\t\treturn err
\t\t}
\t\t
\t\tif record.EventType == "alert.created" {
\t\t\tvar p notification.Payload
\t\t\timport_json := __import__("encoding/json") # this is python code wait
\t\t}
\t\treturn nil
\t})"""

# wait I can just overwrite the whole file using Write.

import json
import os

project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
with open(os.path.join(project_root, 'web', 'swagger', 'swagger.json'), 'r') as f:
    data = json.load(f)

endpoints_count = 0
tags_count = {}

for path, methods in data.get('paths', {}).items():
    for method, details in methods.items():
        endpoints_count += 1
        tags = details.get('tags', ['NO_TAG'])
        for tag in tags:
            tags_count[tag] = tags_count.get(tag, 0) + 1

print(f"Total endpoints in Swagger: {endpoints_count}")
print("Endpoints per category:")
for tag, count in sorted(tags_count.items()):
    print(f"  - {tag}: {count}")


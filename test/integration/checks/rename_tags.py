import os
import re

project_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
directories = [
    os.path.join(project_root, "web", "api"),
    os.path.join(project_root, "web", "daemon"),
    os.path.join(project_root, "web", "auth"),
    os.path.join(project_root, "web", "oauth2")
]

to_replace = {
    "@Tags Databasehosts": "@Tags Database Hosts",
    "@Tags Usersettings": "@Tags User Settings",
    "@Tags Servers": "@Tags Servers",  # Just in case
    "@Tags Server": "@Tags Daemon Server", # Distinguish daemon from panel
    "@Tags Token": "@Tags OAuth2",
    "@Tags Root": "@Tags Daemon Root",
    "@Tags Users": "@Tags Users",
    "@Tags Settings": "@Tags Panel Settings",
    "@Tags Roles": "@Tags Roles",
    "@Tags Nodes": "@Tags Nodes",
    "@Tags Config": "@Tags Config",
}


for directory in directories:
    for filename in os.listdir(directory):
        if not filename.endswith(".go"):
            continue
        filepath = os.path.join(directory, filename)
        with open(filepath, 'r') as f:
            content = f.read()
            
        modified = False
        for k, v in to_replace.items():
            if k in content:
                content = content.replace(k, v)
                modified = True
                
        if modified:
            with open(filepath, 'w') as f:
                f.write(content)

print("Replaced tags.")

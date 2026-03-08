import os
import re

directories = [
    "/home/esteban/Descargas/pufferpanel/web/api",
    "/home/esteban/Descargas/pufferpanel/web/daemon",
    "/home/esteban/Descargas/pufferpanel/web/auth",
    "/home/esteban/Descargas/pufferpanel/web/oauth2"
]

missing_annotations = []
total_handlers = 0

for directory in directories:
    for filename in os.listdir(directory):
        if not filename.endswith(".go"):
            continue
        
        filepath = os.path.join(directory, filename)
        with open(filepath, 'r') as f:
            content = f.read()
            lines = content.split('\n')
        
        current_annotations = []
        for line in lines:
            stripped = line.strip()
            if stripped.startswith("//"):
                current_annotations.append(stripped)
            elif stripped.startswith("func "):
                func_match = re.search(r"func(?:\s+\([^)]+\))?\s+([a-zA-Z0-9_]+)\(\w+\s+\*gin\.Context\)", stripped)
                if func_match:
                    handler = func_match.group(1)
                    
                    # Some ignore list
                    if handler in ["RegisterRoutes", "RegisterDaemonRoutes", "RegisterServerRoutes", "responseAndRecover", "Options", "Handle404", "Handle405", "Proxy"]:
                        current_annotations = []
                        continue
                        
                    total_handlers += 1
                    has_router = any("@Router" in a for a in current_annotations)
                    has_id = "@Summary" in "".join(current_annotations)
                    if not has_router and handler not in ["gatusProxy", "webManifest"]:
                        missing_annotations.append(f"{handler} in {filename} missing @Router")
                current_annotations = []
            elif stripped == "":
                pass
            else:
                current_annotations = []

print(f"Total handlers found: {total_handlers}")
for m in missing_annotations:
    print(m)


import os
import re

directories = [
    "/home/esteban/Descargas/pufferpanel/web/api",
    "/home/esteban/Descargas/pufferpanel/web/daemon",
    "/home/esteban/Descargas/pufferpanel/web/auth",
    "/home/esteban/Descargas/pufferpanel/web/oauth2"
]

missing = []

for directory in directories:
    for filename in os.listdir(directory):
        if not filename.endswith(".go"):
            continue
        
        filepath = os.path.join(directory, filename)
        with open(filepath, 'r') as f:
            content = f.read()
            lines = content.split('\n')
        
        handler_annotations = {}
        current_annotations = []
        for line in lines:
            stripped = line.strip()
            if stripped.startswith("// @"):
                current_annotations.append(stripped)
            elif stripped.startswith("func "):
                func_match = re.search(r"func(?:\s+\([^)]+\))?\s+([a-zA-Z0-9_]+)\(", stripped)
                if func_match and current_annotations:
                    handler_annotations[func_match.group(1)] = current_annotations
                current_annotations = []
            elif not stripped or stripped.startswith("//"):
                pass
            else:
                current_annotations = []
                
        # Regex to find: g.Handle("METHOD", "PATH", ... handler)
        # or rg.GET("PATH", ... handler)
        # Being very loose: \.Handle\(\s*"([A-Z]+)"\s*,\s*"([^"]*)"[^)]+?,\s*([a-zA-Z0-9_]+)\s*\)
        # And \.(GET|POST|PUT|DELETE|PATCH)\(\s*"([^"]*)"[^)]+?,\s*([a-zA-Z0-9_]+)\s*\)
        
        # Let's just find handler names from line ending with `xxx)`
        for line in lines:
            line = line.strip()
            if ".Handle(" in line or ".GET(" in line or ".POST(" in line or ".PUT(" in line or ".DELETE(" in line or ".PATCH(" in line:
                if "OPTIONS" in line or "response.CreateOptions" in line or "NotImplemented" in line:
                    continue
                # The last word before `)` is usually the handler
                m = re.search(r',\s*([a-zA-Z0-9_]+)\s*\)$', line)
                if m:
                    handler = m.group(1)
                    if handler in ["RegisterRoutes", "RegisterDaemonRoutes"]: continue
                    
                    ann = handler_annotations.get(handler, [])
                    has_router = any(a.startswith("// @Router") for a in ann)
                    if not has_router:
                        missing.append(f"{handler} in {filename} ({line})")

print(f"Total endpoints found missing annotations: {len(missing)}")
for m in missing:
    print("  " + m)

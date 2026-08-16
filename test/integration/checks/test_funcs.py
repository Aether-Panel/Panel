import os
import re

directories = [
    "/home/esteban/Descargas/skypanel/web/api",
    "/home/esteban/Descargas/skypanel/web/daemon",
    "/home/esteban/Descargas/skypanel/web/auth",
    "/home/esteban/Descargas/skypanel/web/oauth2"
]

for directory in directories:
    for filename in os.listdir(directory):
        if not filename.endswith(".go"):
            continue
        filepath = os.path.join(directory, filename)
        
        with open(filepath, 'r') as f:
            lines = f.read().split('\n')
            
        current_annotations = []
        for line in lines:
            stripped = line.strip()
            if stripped.startswith("// @"):
                current_annotations.append(stripped)
            elif stripped.startswith("func "):
                if "gin.Context" in stripped:
                    print(f"FOUND: {stripped}")
                    has_router = any("@Router" in ann for ann in current_annotations)
                    if not has_router:
                        print(f" -> NO ROUTER for {stripped}")
                current_annotations = []
            elif stripped == "":
                pass
            else:
                current_annotations = []

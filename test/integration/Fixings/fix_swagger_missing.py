import os

directories = [
    "/home/esteban/Descargas/pufferpanel/web/api",
    "/home/esteban/Descargas/pufferpanel/web/daemon",
    "/home/esteban/Descargas/pufferpanel/web/auth",
    "/home/esteban/Descargas/pufferpanel/web/oauth2"
]

for directory in directories:
    for filename in os.listdir(directory):
        if not filename.endswith(".go"):
            continue
        
        filepath = os.path.join(directory, filename)
        with open(filepath, 'r') as f:
            lines = f.readlines()
        
        base_name = os.path.splitext(filename)[0]
        tag_name = base_name.capitalize()
        
        if base_name == "databasehosts":
            tag_name = "Database Hosts"
        elif base_name == "usersettings":
            tag_name = "User Settings"
        elif directory.endswith("oauth2"):
            tag_name = "OAuth2"
        elif directory.endswith("auth"):
            tag_name = "Auth"
        elif directory.endswith("daemon") and base_name == "server":
            tag_name = "Daemon Server"
        
        new_lines = []
        i = 0
        while i < len(lines):
            line = lines[i]
            stripped = line.strip()
            
            if stripped.startswith("// @"):
                block_start = i
                block_end = i
                while block_end < len(lines) and lines[block_end].strip().startswith("//"):
                    block_end += 1
                
                if block_end == i:
                    block_end = i + 1
                    
                block_lines = lines[block_start:block_end]
                
                has_router = any(l.strip().startswith("// @Router") for l in block_lines)
                has_tags = any(l.strip().startswith("// @Tags") for l in block_lines)
                
                if has_router and not has_tags:
                    for j, bl in enumerate(block_lines):
                        if bl.strip().startswith("// @Router"):
                            block_lines.insert(j, f"// @Tags {tag_name}\n")
                            break
                            
                new_lines.extend(block_lines)
                i = block_end
            else:
                new_lines.append(line)
                i += 1
                
        with open(filepath, 'w') as f:
            f.writelines(new_lines)

print("Swagger missing tags added for all directories.")

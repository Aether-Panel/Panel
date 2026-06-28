import os
import re

directories = ["/home/esteban/Descargas/pufferpanel/web/api", "/home/esteban/Descargas/pufferpanel/web/daemon"]

for directory in directories:
    for filename in os.listdir(directory):
        if not filename.endswith(".go"):
            continue
        
        filepath = os.path.join(directory, filename)
        with open(filepath, 'r') as f:
            lines = f.readlines()
        
        base_name = os.path.splitext(filename)[0]
        # Make a reasonable tag from filename (e.g. databasehosts -> Database Hosts, servers -> Servers)
        tag_name = base_name.capitalize()
        if base_name == "databasehosts":
            tag_name = "Database Hosts"
        elif base_name == "usersettings":
            tag_name = "User Settings"
        
        new_lines = []
        in_swagger_block = False
        swagger_block_has_router = False
        swagger_block_has_tags = False
        block_lines = []
        
        for i, line in enumerate(lines):
            stripped = line.strip()
            if stripped.startswith("// @"):
                in_swagger_block = True
                block_lines.append(line)
                if stripped.startswith("// @Router"):
                    swagger_block_has_router = True
                elif stripped.startswith("// @Tags"):
                    swagger_block_has_tags = True
            elif in_swagger_block:
                # End of a contiguous block of // @ lines
                # If there's a // @Router but no // @Tags, we add one right before the first line or just before the current line
                if swagger_block_has_router and not swagger_block_has_tags:
                    # Insert the // @Tags line before the function def
                    block_lines.append(f"// @Tags {tag_name}\n")
                
                new_lines.extend(block_lines)
                new_lines.append(line)
                
                # Reset
                in_swagger_block = False
                swagger_block_has_router = False
                swagger_block_has_tags = False
                block_lines = []
            else:
                new_lines.append(line)
                
        # In case file ends with a block (unlikely but possible)
        if in_swagger_block:
            if swagger_block_has_router and not swagger_block_has_tags:
                block_lines.append(f"// @Tags {tag_name}\n")
            new_lines.extend(block_lines)
            
        with open(filepath, 'w') as f:
            f.writelines(new_lines)

print("Swagger annotated.")


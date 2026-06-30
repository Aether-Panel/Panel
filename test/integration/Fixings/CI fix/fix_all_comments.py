
import re

with open('internal/database/upgrade.go', 'r', encoding='utf-8') as f:
    content = f.read()

# Match '//' followed by a lowercase character, and replace with '// ' + the character
content = re.sub(r'//([a-z])', r'// \1', content)

with open('internal/database/upgrade.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Fixed all comments in upgrade.go')


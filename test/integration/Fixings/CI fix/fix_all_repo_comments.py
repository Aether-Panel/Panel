
import os
import re

def fix_comments_in_file(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()

        new_content = re.sub(r'//(?!(go:|\+build|lint:ignore|\s))([a-zA-Z0-9])', r'// \2', content)

        if new_content != content:
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(new_content)
            print(f'Fixed comments in {filepath}')
    except Exception as e:
        print(f'Error processing {filepath}: {e}')

for root, _, files in os.walk('.'):
    if 'vendor' in root or 'client' in root or 'assets' in root or '.git' in root or 'node_modules' in root:
        continue
    for file in files:
        if file.endswith('.go'):
            fix_comments_in_file(os.path.join(root, file))

print('Done fixing all comments')


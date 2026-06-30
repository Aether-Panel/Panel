
import os

def fix_urls(filepath):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()

        new_content = content.replace('http:// ', 'http://')
        new_content = new_content.replace('https:// ', 'https://')
        new_content = new_content.replace('} //@name', '} // @name')

        if new_content != content:
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(new_content)
            print(f'Fixed URLs in {filepath}')
    except Exception as e:
        print(f'Error processing {filepath}: {e}')

for root, _, files in os.walk('.'):
    if 'vendor' in root or 'client' in root or 'assets' in root or '.git' in root or 'node_modules' in root:
        continue
    for file in files:
        if file.endswith('.go'):
            fix_urls(os.path.join(root, file))

print('Done fixing URLs and @name')


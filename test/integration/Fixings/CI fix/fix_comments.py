
with open('internal/database/upgrade.go', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('//include 1 for models', '// include 1 for models')
content = content.replace('//these are migrations we need done first before we can do models', '// these are migrations we need done first before we can do models')
content = content.replace('//just dump it into working dir', '// just dump it into working dir')

with open('internal/database/upgrade.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Fixed comments in upgrade.go')


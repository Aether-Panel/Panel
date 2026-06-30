
with open('internal/database/upgrade.go', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('//this is going to be a nightmare', '// this is going to be a nightmare')
content = content.replace('//owners of this permission set', '// owners of this permission set')
content = content.replace('//if this set is for a server, what server', '// if this set is for a server, what server')

with open('internal/database/upgrade.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Fixed more comments in upgrade.go')


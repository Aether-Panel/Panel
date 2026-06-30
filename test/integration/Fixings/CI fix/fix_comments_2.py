
with open('internal/database/upgrade.go', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('//now we can do the models directly', '// now we can do the models directly')
content = content.replace('//at this point for mysql, just manually do the queries...', '// at this point for mysql, just manually do the queries...')
content = content.replace('//return err', '// return err')

with open('internal/database/upgrade.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Fixed more comments in upgrade.go')


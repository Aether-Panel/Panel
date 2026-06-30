with open('internal/web/daemon/server.go', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace(':serverID', ':serverId')
content = content.replace('c.Param("serverID")', 'c.Param("serverId")')

with open('internal/web/daemon/server.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Replaced :serverID with :serverId in daemon/server.go')

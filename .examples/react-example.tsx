import { useState, useEffect, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useApi } from '@/hooks/useApi'
import { Icon } from '@/components/Icon'

interface Server {
    id: string
    name: string
    status: 'online' | 'offline'
    cpu: number
    memory: number
    players: number
    maxPlayers: number
}

export function ServerList() {
    const api = useApi()
    const navigate = useNavigate()
    const { t } = useTranslation()

    // Estado
    const [servers, setServers] = useState<Server[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    // Computed values
    const totalServers = useMemo(() => servers.length, [servers])
    const onlineServers = useMemo(
        () => servers.filter(s => s.status === 'online').length,
        [servers]
    )

    // Lifecycle
    useEffect(() => {
        loadServers()
    }, [])

    // Funciones
    async function loadServers() {
        try {
            setLoading(true)
            const data = await api.servers.list()
            setServers(data)
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Unknown error')
        } finally {
            setLoading(false)
        }
    }

    async function startServer(id: string) {
        await api.servers.start(id)
        await loadServers()
    }

    async function stopServer(id: string) {
        await api.servers.stop(id)
        await loadServers()
    }

    // Render
    return (
        <div className="container mx-auto p-6">
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-3xl font-bold">{t('servers.title')}</h1>
                <button
                    className="btn btn-primary"
                    onClick={() => navigate('/servers/create')}
                >
                    <Icon name="plus" />
                    {t('servers.create')}
                </button>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4 mb-6">
                <div className="stat-card">
                    <h3>{t('servers.total')}</h3>
                    <p className="text-4xl font-bold">{totalServers}</p>
                </div>
                <div className="stat-card">
                    <h3>{t('servers.online')}</h3>
                    <p className="text-4xl font-bold text-green-500">{onlineServers}</p>
                </div>
                <div className="stat-card">
                    <h3>{t('servers.offline')}</h3>
                    <p className="text-4xl font-bold text-red-500">
                        {totalServers - onlineServers}
                    </p>
                </div>
            </div>

            {/* Loading state */}
            {loading && (
                <div className="flex justify-center py-12">
                    <div className="spinner"></div>
                </div>
            )}

            {/* Error state */}
            {error && (
                <div className="alert alert-error">
                    <Icon name="alert-circle" />
                    <p>{error}</p>
                </div>
            )}

            {/* Server list */}
            {!loading && !error && servers.length > 0 && (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {servers.map(server => (
                        <div
                            key={server.id}
                            className="server-card"
                            onClick={() => navigate(`/servers/${server.id}`)}
                        >
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-xl font-semibold">{server.name}</h3>
                                <span
                                    className={`status-badge ${server.status === 'online' ? 'online' : 'offline'
                                        }`}
                                >
                                    {server.status}
                                </span>
                            </div>

                            <div className="server-info">
                                <p><Icon name="cpu" /> {server.cpu}%</p>
                                <p><Icon name="memory" /> {server.memory}MB</p>
                                <p><Icon name="users" /> {server.players}/{server.maxPlayers}</p>
                            </div>

                            <div className="flex gap-2 mt-4">
                                <button
                                    className="btn btn-sm btn-success"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        startServer(server.id)
                                    }}
                                    disabled={server.status === 'online'}
                                >
                                    <Icon name="play" />
                                    {t('servers.start')}
                                </button>
                                <button
                                    className="btn btn-sm btn-error"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        stopServer(server.id)
                                    }}
                                    disabled={server.status === 'offline'}
                                >
                                    <Icon name="stop" />
                                    {t('servers.stop')}
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Empty state */}
            {!loading && !error && servers.length === 0 && (
                <div className="empty-state">
                    <Icon name="server" size={64} />
                    <h3>{t('servers.noServers')}</h3>
                    <p>{t('servers.createFirst')}</p>
                    <button
                        className="btn btn-primary mt-4"
                        onClick={() => navigate('/servers/create')}
                    >
                        {t('servers.create')}
                    </button>
                </div>
            )}
        </div>
    )
}

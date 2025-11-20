export class UptimeApi {
  _api = null

  constructor(api) {
    this._api = api
  }

  async getAll(days = 30) {
    const res = await this._api.get('/api/uptime', { days })
    return res.data
  }

  async getServer(serverId, days = 30, limit = 50) {
    const res = await this._api.get(`/api/uptime/${serverId}`, { days, limit })
    return res.data
  }
}


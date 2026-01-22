export class DatabaseApi {
  _api = null

  constructor(api) {
    this._api = api
  }

  async list(serverId) {
    const res = await this._api.get(`/api/servers/${serverId}/databases`)
    return res.data
  }

  async create(serverId, database) {
    const res = await this._api.post(`/api/servers/${serverId}/databases`, database)
    return res.data
  }

  async delete(serverId, id) {
    await this._api.delete(`/api/servers/${serverId}/databases/${id}`)
    return true
  }
}

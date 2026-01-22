export class DatabaseHostApi {
  _api = null

  constructor(api) {
    this._api = api
  }

  async list() {
    const res = await this._api.get('/api/databasehosts')
    return res.data
  }

  async get(id) {
    const res = await this._api.get(`/api/databasehosts/${id}`)
    return res.data
  }

  async create(host) {
    const res = await this._api.post('/api/databasehosts', host)
    return res.data
  }

  async update(id, host) {
    await this._api.put(`/api/databasehosts/${id}`, host)
    return true
  }

  async delete(id) {
    await this._api.delete(`/api/databasehosts/${id}`)
    return true
  }
}

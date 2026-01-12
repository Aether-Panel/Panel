export class RoleApi {
  _api = null

  constructor(api) {
    this._api = api
  }

  async list() {
    const res = await this._api.get('/api/roles')
    return res.data
  }

  async get(id) {
    const res = await this._api.get(`/api/roles/${id}`)
    return res.data
  }

  async create(role) {
    const res = await this._api.post('/api/roles', role)
    return res.data.id
  }

  async update(id, role) {
    const res = await this._api.post(`/api/roles/${id}`, role)
    return res.data
  }

  async delete(id) {
    await this._api.delete(`/api/roles/${id}`)
    return true
  }
}


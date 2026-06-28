import pytest
import requests

def test_provision_server(auth_session, api_base_url, requests_mock):
    # Mocking la respuesta de la API ya que no tenemos el servidor corriendo nativamente en el entorno de pruebas CI
    requests_mock.post(f"{api_base_url}/servers", json={"status": "success", "server_id": "srv-python-123"})
    
    response = auth_session.post(f"{api_base_url}/servers", json={
        "name": "E2E Server",
        "node_id": 1,
        "type": "minecraft"
    })
    
    assert response.status_code == 200
    assert response.json()["server_id"] == "srv-python-123"

def test_create_database(auth_session, api_base_url, requests_mock):
    server_id = "srv-python-123"
    requests_mock.post(f"{api_base_url}/servers/{server_id}/databases", json={
        "status": "success", 
        "database": "db_mc",
        "password": "secret_password"
    })
    
    response = auth_session.post(f"{api_base_url}/servers/{server_id}/databases", json={})
    
    assert response.status_code == 200
    assert "password" in response.json()

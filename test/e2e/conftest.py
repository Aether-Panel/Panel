import pytest
import requests

@pytest.fixture(scope="session")
def api_base_url():
    # En Docker asume que el panel corre en localhost:8080 (Mocked for testing logic)
    return "http://localhost:8080/api"

@pytest.fixture(scope="session")
def auth_session(api_base_url):
    session = requests.Session()
    # En una prueba real, haríamos POST a /auth/login con credenciales
    # session.post(f"{api_base_url}/auth/login", json={"email": "admin@example.com", "password": "password"})
    return session

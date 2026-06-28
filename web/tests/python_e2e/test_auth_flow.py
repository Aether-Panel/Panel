import pytest
import requests

def test_auth_flow(auth_session, api_base_url, requests_mock):
    # Mock login endpoint
    requests_mock.post(f"{api_base_url}/auth/login", json={"scopes": ["admin"]})
    
    # 1. Login
    login_response = auth_session.post(f"{api_base_url}/auth/login", json={
        "email": "admin@example.com",
        "password": "password"
    })
    
    assert login_response.status_code == 200
    assert "admin" in login_response.json()["scopes"]
    
    # 2. Reauth
    requests_mock.post(f"{api_base_url}/auth/reauth", json={"scopes": ["admin"]})
    reauth_response = auth_session.post(f"{api_base_url}/auth/reauth")
    assert reauth_response.status_code == 200
    
    # 3. Logout
    requests_mock.post(f"{api_base_url}/auth/logout", status_code=204)
    logout_response = auth_session.post(f"{api_base_url}/auth/logout")
    assert logout_response.status_code == 204

import pytest
import requests

def test_settings_flow(auth_session, api_base_url, requests_mock):
    # 1. Fetch settings
    requests_mock.get(f"{api_base_url}/settings", json=[
        {"key": "companyName", "value": "SkyPanelTest"},
        {"key": "defaultTheme", "value": "dark"}
    ])
    
    get_response = auth_session.get(f"{api_base_url}/settings")
    assert get_response.status_code == 200
    assert len(get_response.json()) == 2
    assert get_response.json()[0]["value"] == "SkyPanelTest"
    
    # 2. Update setting
    requests_mock.put(f"{api_base_url}/settings/companyName", status_code=200)
    
    put_response = auth_session.put(f"{api_base_url}/settings/companyName", json={
        "value": "New Company Name"
    })
    assert put_response.status_code == 200

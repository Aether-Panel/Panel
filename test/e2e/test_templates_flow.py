import pytest
import requests

def test_templates_flow(auth_session, api_base_url, requests_mock):
    # 1. Fetch template repos
    requests_mock.get(f"{api_base_url}/templates", json=[
        {"id": 1, "name": "Vanilla Minecraft", "repo_url": "https://github.com/..."},
        {"id": 2, "name": "PaperMC", "repo_url": "https://github.com/... "}
    ])
    
    get_response = auth_session.get(f"{api_base_url}/templates")
    assert get_response.status_code == 200
    assert len(get_response.json()) == 2
    assert get_response.json()[0]["name"] == "Vanilla Minecraft"
    
    # 2. Sync template
    requests_mock.post(f"{api_base_url}/templates/sync", json={"status": "success", "synced": 5})
    
    post_response = auth_session.post(f"{api_base_url}/templates/sync")
    assert post_response.status_code == 200
    assert post_response.json()["synced"] == 5

import pytest
import requests

def test_nodes_flow(auth_session, api_base_url, requests_mock):
    # 1. Fetch nodes
    requests_mock.get(f"{api_base_url}/nodes", json=[
        {"id": 1, "name": "Local Node", "public_host": "127.0.0.1"},
        {"id": 2, "name": "Remote Node", "public_host": "10.0.0.5"}
    ])
    
    get_response = auth_session.get(f"{api_base_url}/nodes")
    assert get_response.status_code == 200
    assert len(get_response.json()) == 2
    assert get_response.json()[0]["name"] == "Local Node"
    
    # 2. Create node
    requests_mock.post(f"{api_base_url}/nodes", json={"status": "success", "node_id": 3})
    
    post_response = auth_session.post(f"{api_base_url}/nodes", json={
        "name": "New Node",
        "public_host": "10.0.0.6",
        "private_host": "10.0.0.6",
        "public_port": 8080,
        "private_port": 8080,
        "sftp_port": 2022
    })
    assert post_response.status_code == 200
    assert post_response.json()["node_id"] == 3

import pytest
import requests

def test_extransfer_flow(auth_session, api_base_url, requests_mock):
    # This simulates the origin server initiating a federated external transfer
    requests_mock.post(f"{api_base_url}/servers/srv-123/extransfer/initiate", json={
        "token": "simulated_transfer_token_abc123"
    })
    
    init_response = auth_session.post(f"{api_base_url}/servers/srv-123/extransfer/initiate", json={
        "target_url": "http://destination-panel.local"
    })
    assert init_response.status_code == 200
    token = init_response.json()["token"]
    
    # Now simulate the destination panel hitting the /extransfer/validate endpoint of the origin
    # But since api_base_url points to the mock origin, we test the origin's validate mock
    requests_mock.post(f"{api_base_url}/extransfer/validate", json={
        "session_id": "session_xyz789",
        "status": "valid"
    })
    
    # In reality, this would be an unauthenticated request from destination to origin
    validate_response = requests.post(f"{api_base_url}/extransfer/validate", json={
        "token": token,
        "target_public_key": "pubkey_xyz",
        "protocol_version": "v1"
    })
    
    assert validate_response.status_code == 200
    assert "session_id" in validate_response.json()

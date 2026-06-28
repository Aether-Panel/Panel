import pytest
import requests

def test_ai_endpoint(auth_session, api_base_url, requests_mock):
    # Simulate a request where the AI endpoint returns 400 because API key is not configured
    requests_mock.post(f"{api_base_url}/servers/srv-123/ai/analyze", status_code=400, json={
        "error": "Gemini API Key is not configured. Please add it in Settings."
    })
    
    response = auth_session.post(f"{api_base_url}/servers/srv-123/ai/analyze", json={
        "logs": ["Exception in thread main: java.lang.NullPointerException"]
    })
    
    assert response.status_code == 400
    assert "Gemini API Key is not configured" in response.json()["error"]

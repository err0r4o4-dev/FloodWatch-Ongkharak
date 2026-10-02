import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)

def test_health():
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}

def test_analyze_media():
    payload = {
        "media_url": "https://example.com/test-image-123.jpg",
        "report_id": "test-report-uuid"
    }
    response = client.post("/analyze/media", json=payload)

    assert response.status_code == 200
    data = response.json()

    assert "model_version" in data
    assert "analysis_time_ms" in data
    assert "flood" in data
    assert "road" in data

    # Since it's a mock using length mod 2, we know whether flood or road is returned.
    # Length of "https://example.com/test-image-123.jpg" is 38 (even), so flood should be returned
    assert data["flood"] is not None
    assert data["flood"]["severity"] == "WARNING"
    assert data["flood"]["confidence"] == 0.85

    assert data["road"] is None

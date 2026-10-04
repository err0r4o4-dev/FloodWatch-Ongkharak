import base64
from uuid import uuid4

import pytest
from fastapi.testclient import TestClient

from app.main import app


@pytest.fixture
def client():
    app.state.analyzer = None
    with TestClient(app) as session:
        yield session
    app.state.analyzer = None


def payload():
    return {"media_base64": base64.b64encode(b"\x89PNG\r\n\x1a\nsynthetic").decode(), "content_type": "image/png", "report_id": str(uuid4()), "media_id": str(uuid4())}


def result():
    return {"flood": {"severity": "UNKNOWN", "confidence": 0}, "road": None, "analysis_time_ms": 1, "model_version": "test-adapter", "is_mock": False, "limitations": ["Synthetic test adapter only"]}


def test_unconfigured_model_is_unavailable(client):
    response = client.post("/analyze/media", json=payload())
    assert response.status_code == 503
    assert response.json()["detail"]["code"] == "MODEL_UNAVAILABLE"


@pytest.mark.parametrize("change", [{"media_base64": "private-invalid-image"}, {"media_url": "https://private.example/image"}, {"report_id": "private-invalid-id"}, {"content_type": "text/plain"}])
def test_invalid_request_is_sanitized(client, change):
    request = payload() | change
    response = client.post("/analyze/media", json=request)
    assert response.status_code == 422
    assert "private" not in response.text
    assert request["media_base64"] not in response.text


def test_signature_mismatch(client):
    assert client.post("/analyze/media", json=payload() | {"content_type": "image/jpeg"}).status_code == 400


def test_decoded_size_is_bounded(client):
    oversized = base64.b64encode(b"a" * (5 * 1024 * 1024 + 1)).decode()
    assert client.post("/analyze/media", json=payload() | {"media_base64": oversized}).status_code == 422


@pytest.mark.parametrize("change", [{"is_mock": True}, {"model_version": "mock-model"}, {"flood": {"severity": "SAFE", "confidence": 1}}, {"flood": {"severity": "UNKNOWN", "confidence": 2}}])
def test_invalid_adapter_output_is_unavailable(client, change):
    app.state.analyzer = lambda image, request: result() | change
    assert client.post("/analyze/media", json=payload()).status_code == 503


def test_adapter_receives_private_bytes_and_preserves_unknown(client):
    captured = []
    def analyze(image, request):
        captured.append(image)
        return result()
    app.state.analyzer = analyze
    response = client.post("/analyze/media", json=payload())
    assert response.status_code == 200
    assert captured == [b"\x89PNG\r\n\x1a\nsynthetic"]
    assert response.json()["flood"] == {"severity": "UNKNOWN", "water_depth_m": None, "confidence": 0, "bounding_box": None}

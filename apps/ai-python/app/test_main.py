import base64
import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)

def test_health():
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}

def media_input():
    return {
        "media_base64": base64.b64encode(b"\x89PNG\r\n\x1a\nsynthetic-test-only").decode(),
        "content_type": "image/png",
        "report_id": "00000000-0000-4000-8000-000000000001",
        "media_id": "00000000-0000-4000-8000-000000000002",
    }


def test_model_unavailable_does_not_fabricate_predictions():
    response = client.post("/analyze/media", json=media_input())
    assert response.status_code == 503
    assert response.json()["detail"]["code"] == "MODEL_UNAVAILABLE"


@pytest.mark.parametrize("changes", [
    {"media_base64": "not-base64"},
    {"content_type": "text/html"},
    {"report_id": "not-a-uuid"},
    {"media_base64": base64.b64encode(b"not-an-image").decode()},
    {"media_url": "http://localhost/private"},
])
def test_media_rejects_invalid_input_before_inference(changes):
    response = client.post("/analyze/media", json=media_input() | changes)
    assert response.status_code in (400, 422)


def test_analysis_provider_output_retains_model_and_limitations(monkeypatch):
    from app.schemas import AnalyzeMediaResponse, FloodEstimate

    def analyze(image, request):
        assert image.startswith(b"\x89PNG")
        return AnalyzeMediaResponse(
            flood=FloodEstimate(severity="WARNING", confidence=0.85),
            analysis_time_ms=1, model_version="test-provider-v1",
            is_mock=False, limitations=["Synthetic test provider; not live evidence"],
        )

    monkeypatch.setattr(app.state, "analyzer", analyze, raising=False)
    response = client.post("/analyze/media", json=media_input())
    assert response.status_code == 200
    data = response.json()
    assert data["flood"]["severity"] == "WARNING"
    assert data["flood"]["confidence"] == 0.85
    assert data["model_version"] == "test-provider-v1"
    assert data["is_mock"] is False
    assert data["limitations"]


def test_provider_failure_is_unavailable_without_internal_details(monkeypatch):
    def analyze(image, request):
        raise RuntimeError("private provider credential")

    monkeypatch.setattr(app.state, "analyzer", analyze, raising=False)
    response = client.post("/analyze/media", json=media_input())
    assert response.status_code == 503
    assert "private provider" not in response.text


def test_legacy_url_request_is_rejected():
    payload = {
        "media_url": "https://example.com/test-image-123.jpg",
        "report_id": "test-report-uuid"
    }
    response = client.post("/analyze/media", json=payload)

    assert response.status_code == 422


def test_validation_errors_do_not_echo_private_media():
    payload = media_input() | {"media_base64": "private-media-invalid-base64"}
    response = client.post("/analyze/media", json=payload)
    assert response.status_code == 422
    assert "private-media-invalid" not in response.text


def test_mock_provider_output_cannot_become_evidence(monkeypatch):
    from app.schemas import AnalyzeMediaResponse

    def analyze(image, request):
        return AnalyzeMediaResponse(
            analysis_time_ms=1, model_version="mock-v1", is_mock=True,
            limitations=["Not real inference"],
        )

    monkeypatch.setattr(app.state, "analyzer", analyze, raising=False)
    response = client.post("/analyze/media", json=media_input())
    assert response.status_code == 503

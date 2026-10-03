import base64
from fastapi import FastAPI, HTTPException
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from app.schemas import (
    HealthResponse,
    AnalyzeMediaRequest,
    AnalyzeMediaResponse,
)

app = FastAPI(title="Flood Watch AI Service")
app.state.analyzer = None

@app.exception_handler(RequestValidationError)
async def invalid_input(request, error):
    return JSONResponse(status_code=422, content={"detail": {"code": "INVALID_REQUEST", "message": "Invalid media request"}})


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    return HealthResponse(status="ok")

@app.post("/analyze/media", response_model=AnalyzeMediaResponse)
def analyze_media(request: AnalyzeMediaRequest) -> AnalyzeMediaResponse:
    image = base64.b64decode(request.media_base64, validate=True)
    signatures = {
        "image/png": image.startswith(b"\x89PNG\r\n\x1a\n"),
        "image/jpeg": image.startswith(b"\xff\xd8\xff"),
        "image/webp": image.startswith(b"RIFF") and image[8:12] == b"WEBP",
    }
    if not signatures[request.content_type]:
        raise HTTPException(400, detail={"code": "INVALID_MEDIA", "message": "Image signature does not match its type"})
    if app.state.analyzer is None:
        raise HTTPException(503, detail={"code": "MODEL_UNAVAILABLE", "message": "No inference model is configured"})
    try:
        result = AnalyzeMediaResponse.model_validate(app.state.analyzer(image, request))
        if result.is_mock or "mock" in result.model_version.lower():
            raise ValueError("Mock evidence is not publishable")
        return result
    except Exception:
        raise HTTPException(503, detail={"code": "MODEL_UNAVAILABLE", "message": "Inference is unavailable"}) from None

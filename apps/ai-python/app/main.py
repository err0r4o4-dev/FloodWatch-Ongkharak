import time
import random
from fastapi import FastAPI, HTTPException

from app.schemas import (
    HealthResponse,
    AnalyzeMediaRequest,
    AnalyzeMediaResponse,
    FloodEstimate,
    RoadEstimate
)

app = FastAPI(title="Flood Watch AI Service")


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    return HealthResponse(status="ok")

@app.post("/analyze/media", response_model=AnalyzeMediaResponse)
def analyze_media(request: AnalyzeMediaRequest) -> AnalyzeMediaResponse:
    # In Phase 0/10, we don't have a real ML model deployed.
    # We will simulate processing time and return a mock prediction.
    start_time = time.time()

    # Simple mock logic based on URL length to be deterministic but varied
    length = len(request.media_url)

    # Sleep to simulate network/inference delay (100-300ms)
    time.sleep(random.uniform(0.1, 0.3))

    # 50% chance to detect flood, 50% chance to detect road issue
    flood = None
    road = None

    if length % 2 == 0:
        flood = FloodEstimate(
            severity="WARNING",
            water_depth_m=0.45,
            confidence=0.85,
            bounding_box=[0.1, 0.1, 0.9, 0.9]
        )
    else:
        road = RoadEstimate(
            status="SMALL_VEHICLE_NOT_RECOMMENDED",
            confidence=0.92,
            bounding_box=[0.2, 0.2, 0.8, 0.8]
        )

    analysis_time = int((time.time() - start_time) * 1000)

    return AnalyzeMediaResponse(
        flood=flood,
        road=road,
        analysis_time_ms=analysis_time,
        model_version="FloodVision-v1-mock"
    )

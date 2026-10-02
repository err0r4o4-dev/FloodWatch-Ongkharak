from typing import Literal, Optional, List
from pydantic import BaseModel, Field

class HealthResponse(BaseModel):
    status: Literal["ok"]

class AnalyzeMediaRequest(BaseModel):
    media_url: str = Field(..., description="URL of the media to analyze (must be accessible to the AI service)")
    report_id: Optional[str] = Field(None, description="Optional associated community report ID")

class FloodEstimate(BaseModel):
    severity: Literal["INFO", "WATCH", "WARNING", "CRITICAL", "UNKNOWN"]
    water_depth_m: Optional[float] = None
    confidence: float = Field(..., ge=0.0, le=1.0)
    bounding_box: Optional[List[float]] = Field(None, description="[ymin, xmin, ymax, xmax]")

class RoadEstimate(BaseModel):
    status: Literal["PASSABLE", "WATCH", "SMALL_VEHICLE_NOT_RECOMMENDED", "CLOSED", "UNKNOWN"]
    confidence: float = Field(..., ge=0.0, le=1.0)
    bounding_box: Optional[List[float]] = Field(None, description="[ymin, xmin, ymax, xmax]")

class AnalyzeMediaResponse(BaseModel):
    flood: Optional[FloodEstimate] = None
    road: Optional[RoadEstimate] = None
    analysis_time_ms: int
    model_version: str

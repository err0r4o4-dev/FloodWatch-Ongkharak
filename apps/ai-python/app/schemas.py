import base64
import binascii
from typing import Literal, Optional, List
from uuid import UUID
from pydantic import BaseModel, ConfigDict, Field, field_validator

class HealthResponse(BaseModel):
    status: Literal["ok"]

class AnalyzeMediaRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")
    media_base64: str = Field(min_length=1, max_length=6990508)
    content_type: Literal["image/jpeg", "image/png", "image/webp"]
    report_id: UUID
    media_id: UUID

    @field_validator("media_base64")
    @classmethod
    def validate_media(cls, value: str) -> str:
        try:
            image = base64.b64decode(value, validate=True)
        except (ValueError, binascii.Error) as error:
            raise ValueError("Invalid image encoding") from error
        if not image or len(image) > 5 * 1024 * 1024:
            raise ValueError("Image size is outside the allowed range")
        return value

class FloodEstimate(BaseModel):
    model_config = ConfigDict(extra="forbid", allow_inf_nan=False)
    severity: Literal["INFO", "WATCH", "WARNING", "CRITICAL", "UNKNOWN"]
    water_depth_m: Optional[float] = Field(None, ge=0, le=50)
    confidence: float = Field(..., ge=0.0, le=1.0)
    bounding_box: Optional[List[float]] = Field(None, min_length=4, max_length=4, description="[ymin, xmin, ymax, xmax]")

    @field_validator("bounding_box")
    @classmethod
    def validate_box(cls, value):
        if value is not None and (not all(0 <= item <= 1 for item in value) or value[0] > value[2] or value[1] > value[3]):
            raise ValueError("Invalid bounding box")
        return value

class RoadEstimate(BaseModel):
    model_config = ConfigDict(extra="forbid", allow_inf_nan=False)
    status: Literal["PASSABLE", "WATCH", "SMALL_VEHICLE_NOT_RECOMMENDED", "CLOSED", "UNKNOWN"]
    confidence: float = Field(..., ge=0.0, le=1.0)
    bounding_box: Optional[List[float]] = Field(None, min_length=4, max_length=4, description="[ymin, xmin, ymax, xmax]")

    @field_validator("bounding_box")
    @classmethod
    def validate_box(cls, value):
        return FloodEstimate.validate_box(value)

class AnalyzeMediaResponse(BaseModel):
    model_config = ConfigDict(extra="forbid", allow_inf_nan=False)
    flood: Optional[FloodEstimate] = None
    road: Optional[RoadEstimate] = None
    analysis_time_ms: int = Field(ge=0, le=300000)
    model_version: str = Field(min_length=1, max_length=100)
    is_mock: bool
    limitations: List[str] = Field(min_length=1, max_length=10)

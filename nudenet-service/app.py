"""Self-hosted sensitive-content classification service.

This service is a classification aid, not a legal, abuse, or CSAM detector.
It exists so Shield can warn users and keep them in control before sensitive
imagery is shown in a normal preview — it never makes a deletion decision.

The Go backend is the only intended caller. Detected labels and a boolean
"sensitive" verdict are returned; interpreting that verdict into an evidence
status (safe/review/sensitive) is the Go side's job (internal/contentsafety),
not this service's.
"""

import io
import logging
import os
import tempfile

from fastapi import FastAPI, File, HTTPException, UploadFile
from nudenet import NudeDetector
from PIL import Image
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("nudenet-service")

app = FastAPI(title="Shield Content Safety Service")

_detector: NudeDetector | None = None

# Labels the underlying model can report that indicate exposed nudity.
# Everything else (covered body parts, faces, feet, belly, armpits) is not
# treated as sensitive for Shield's purposes.
SENSITIVE_LABELS = {
    "FEMALE_BREAST_EXPOSED",
    "FEMALE_GENITALIA_EXPOSED",
    "MALE_GENITALIA_EXPOSED",
    "BUTTOCKS_EXPOSED",
    "ANUS_EXPOSED",
}

MAX_IMAGE_BYTES = 55 * 1024 * 1024


def get_detector() -> NudeDetector:
    global _detector
    if _detector is None:
        logger.info("loading NudeDetector model")
        _detector = NudeDetector()
    return _detector


class Detection(BaseModel):
    label: str
    score: float


class ClassifyResponse(BaseModel):
    labels: list[Detection]
    sensitive: bool
    max_sensitive_score: float


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/classify", response_model=ClassifyResponse)
async def classify(file: UploadFile = File(...)) -> ClassifyResponse:
    contents = await file.read()
    if len(contents) > MAX_IMAGE_BYTES:
        raise HTTPException(status_code=413, detail="file too large")

    try:
        # Validate it's actually a decodable image before handing it to the
        # detector; also normalizes to a format the detector reliably reads.
        image = Image.open(io.BytesIO(contents))
        image.verify()
    except Exception as exc:
        raise HTTPException(status_code=400, detail="not a decodable image") from exc

    detector = get_detector()

    tmp = tempfile.NamedTemporaryFile(suffix=".jpg", delete=False)
    try:
        tmp.close()
        Image.open(io.BytesIO(contents)).convert("RGB").save(tmp.name, format="JPEG")
        raw_detections = detector.detect(tmp.name)
    finally:
        os.unlink(tmp.name)

    labels = [Detection(label=d["class"], score=float(d["score"])) for d in raw_detections]

    sensitive_scores = [d.score for d in labels if d.label in SENSITIVE_LABELS]
    max_sensitive_score = max(sensitive_scores) if sensitive_scores else 0.0

    return ClassifyResponse(
        labels=labels,
        sensitive=max_sensitive_score >= 0.6,
        max_sensitive_score=max_sensitive_score,
    )

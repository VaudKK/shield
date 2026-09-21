"""Self-hosted in-place PDF redaction service.

Unlike the per-evidence and disclosure-package image redaction paths (real
pixel boxes drawn over OCR word coordinates), the Go backend has no way to
edit a PDF's own content stream — it can only read PDF text. This service
fills that one gap: given a PDF and a list of literal text values, it finds
every occurrence on every page and actually removes the underlying text and
graphics in that region (not just an overlay), via PyMuPDF's redaction
annotations, then fills the region black.

The Go backend is the only intended caller. This service makes no judgment
about *what* to redact — that list is provided by the caller, which already
has the accepted PII review workflow.
"""

import base64
import json
import logging

from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from pydantic import BaseModel
import fitz  # PyMuPDF

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("pdf-redact-service")

app = FastAPI(title="Shield PDF Redaction Service")

MAX_PDF_BYTES = 55 * 1024 * 1024


class RedactResponse(BaseModel):
    pdf_base64: str
    applied: dict[str, bool]


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/redact", response_model=RedactResponse)
async def redact(file: UploadFile = File(...), values: str = Form(...)) -> RedactResponse:
    try:
        target_values = json.loads(values)
    except json.JSONDecodeError as exc:
        raise HTTPException(status_code=400, detail="values must be a JSON array of strings") from exc
    if not isinstance(target_values, list) or not all(isinstance(v, str) for v in target_values):
        raise HTTPException(status_code=400, detail="values must be a JSON array of strings")

    contents = await file.read()
    if len(contents) > MAX_PDF_BYTES:
        raise HTTPException(status_code=413, detail="file too large")

    try:
        doc = fitz.open(stream=contents, filetype="pdf")
    except Exception as exc:
        raise HTTPException(status_code=400, detail="not a decodable PDF") from exc

    applied: dict[str, bool] = {v: False for v in target_values}

    try:
        for page in doc:
            for value in target_values:
                if not value:
                    continue
                rects = page.search_for(value)
                if not rects:
                    continue
                applied[value] = True
                for rect in rects:
                    page.add_redact_annot(rect, fill=(0, 0, 0))
            # apply_redactions strips the underlying text/graphics within
            # each annotated region and paints the fill color — this is
            # real redaction, not an overlay a viewer could see through or
            # a script could still extract text from.
            page.apply_redactions()

        redacted_bytes = doc.write()
    finally:
        doc.close()

    return RedactResponse(pdf_base64=base64.b64encode(redacted_bytes).decode("ascii"), applied=applied)

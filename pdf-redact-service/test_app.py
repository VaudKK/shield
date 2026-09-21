import base64
import io
import json

import fitz
from fastapi.testclient import TestClient

from app import app

client = TestClient(app)


def _pdf_with_text(text: str) -> bytes:
    doc = fitz.open()
    page = doc.new_page()
    page.insert_text((72, 72), text, fontsize=14)
    data = doc.write()
    doc.close()
    return data


def test_health():
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_redact_removes_matched_text_from_content_stream():
    pdf = _pdf_with_text("Contact Jane at jane@example.com or 555-867-5309.")
    values = ["jane@example.com", "555-867-5309"]

    r = client.post(
        "/redact",
        files={"file": ("note.pdf", pdf, "application/pdf")},
        data={"values": json.dumps(values)},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["applied"] == {"jane@example.com": True, "555-867-5309": True}

    redacted_bytes = base64.b64decode(body["pdf_base64"])
    doc = fitz.open(stream=redacted_bytes, filetype="pdf")
    text = doc[0].get_text()
    doc.close()

    # Real redaction strips the underlying text, not just paints over it —
    # neither value should be extractable from the result at all.
    assert "jane@example.com" not in text
    assert "555-867-5309" not in text
    assert "Contact Jane at" in text  # everything else stays untouched


def test_redact_reports_values_not_found():
    pdf = _pdf_with_text("Nothing sensitive here.")
    values = ["not-present@example.com"]

    r = client.post(
        "/redact",
        files={"file": ("note.pdf", pdf, "application/pdf")},
        data={"values": json.dumps(values)},
    )
    assert r.status_code == 200
    assert r.json()["applied"] == {"not-present@example.com": False}


def test_redact_rejects_non_pdf():
    r = client.post(
        "/redact",
        files={"file": ("note.txt", b"not a pdf", "text/plain")},
        data={"values": "[]"},
    )
    assert r.status_code == 400


def test_redact_rejects_malformed_values():
    pdf = _pdf_with_text("hello")
    r = client.post(
        "/redact",
        files={"file": ("note.pdf", pdf, "application/pdf")},
        data={"values": "not json"},
    )
    assert r.status_code == 400

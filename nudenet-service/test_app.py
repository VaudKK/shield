import io

from fastapi.testclient import TestClient
from PIL import Image

from app import app

client = TestClient(app)


def _solid_color_png() -> bytes:
    img = Image.new("RGB", (64, 64), color=(120, 140, 160))
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def test_health():
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_classify_plain_image_is_not_sensitive():
    r = client.post("/classify", files={"file": ("test.png", _solid_color_png(), "image/png")})
    assert r.status_code == 200
    body = r.json()
    assert body["sensitive"] is False
    assert body["max_sensitive_score"] == 0.0
    assert isinstance(body["labels"], list)


def test_classify_rejects_non_image():
    r = client.post("/classify", files={"file": ("bad.txt", b"not an image", "text/plain")})
    assert r.status_code == 400

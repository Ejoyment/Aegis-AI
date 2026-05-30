"""
AegisAI SLM Redactor API Server
================================
FastAPI server exposing PII redaction endpoints.
Designed to run on-premise as a sidecar service alongside the AegisAI gateway.

Endpoints:
    POST /redact          - Redact PII from chat messages
    POST /redact/response - Redact PII from LLM response content
    GET  /health          - Health check
    GET  /stats           - Redaction statistics
"""

import json
import logging
import os
import time
from typing import List, Dict, Any, Optional

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from app.redactor import PIIRedactor, get_redactor

# Configure logging
logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s | %(levelname)s | %(message)s",
)
logger = logging.getLogger(__name__)

# Create FastAPI app
app = FastAPI(
    title="AegisAI SLM PII Redactor",
    description="On-premise PII redaction service for AI agent governance",
    version="1.0.0",
)

# CORS - allow all origins since this runs in a trusted network
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Request/Response models
class Message(BaseModel):
    """A single chat message."""
    role: str = Field(..., description="Message role (system, user, assistant)")
    content: Any = Field(..., description="Message content (string or array)")

    class Config:
        extra = "allow"

class RedactRequest(BaseModel):
    """Request to redact PII from a list of messages."""
    agent_id: Optional[str] = Field(None, description="Agent identifier for stats")
    messages: List[Dict[str, Any]] = Field(..., description="List of chat messages")

class RedactResponse(BaseModel):
    """Response after PII redaction."""
    agent_id: Optional[str] = None
    messages: List[Dict[str, Any]]
    redactions_made: int = 0
    processing_time_ms: float = 0.0

class RedactResponseContentRequest(BaseModel):
    """Request to redact PII from LLM response content."""
    agent_id: Optional[str] = None
    content: str = Field(..., description="LLM response text to redact")
    model: Optional[str] = Field(None, description="Model that generated the response")

class RedactResponseContent(BaseModel):
    """Response after PII redaction of LLM content."""
    agent_id: Optional[str] = None
    content: str
    redactions_made: int = 0
    processing_time_ms: float = 0.0

class HealthResponse(BaseModel):
    """Health check response."""
    status: str = "ok"
    version: str = "1.0.0"
    uptime_seconds: float = 0.0

class StatsResponse(BaseModel):
    """Redaction statistics response."""
    total_redactions: int
    redacted_fields: Dict[str, int]

# Service start time
_start_time = time.time()


def get_or_create_redactor() -> PIIRedactor:
    """Get or initialize the PII redactor."""
    try:
        return get_redactor()
    except Exception as e:
        logger.error("Failed to initialize redactor: %s", e)
        raise HTTPException(
            status_code=503,
            detail=f"Redactor initialization failed: {str(e)}. "
                   f"Ensure spacy model 'en_core_web_sm' is installed: "
                   f"python -m spacy download en_core_web_sm"
        )


@app.on_event("startup")
async def startup():
    """Initialize the redactor on startup."""
    logger.info("Initializing AegisAI SLM Redactor...")
    try:
        redactor = get_or_create_redactor()
        logger.info("Redactor initialized successfully")
    except Exception as e:
        logger.warning("Redactor initialization failed: %s", e)
        logger.warning("Service will start but redaction endpoints may fail")


@app.post("/redact", response_model=RedactResponse)
async def redact_messages(request: RedactRequest):
    """
    Redact PII from a list of chat messages.

    Scans message content for PII patterns (email, phone, SSN, API keys, etc.)
    and replaces them with [REDACTED - {type}] placeholders.
    """
    start_time = time.time()

    try:
        redactor = get_or_create_redactor()
    except Exception as e:
        raise HTTPException(status_code=503, detail=str(e))

    redacted_messages = redactor.redact_messages(
        messages=request.messages,
        agent_id=request.agent_id,
    )

    processing_time = (time.time() - start_time) * 1000

    return RedactResponse(
        agent_id=request.agent_id,
        messages=redacted_messages,
        redactions_made=redactor.stats["total_redactions"],
        processing_time_ms=round(processing_time, 2),
    )


@app.post("/redact/response", response_model=RedactResponseContent)
async def redact_response_content(request: RedactResponseContentRequest):
    """
    Redact PII from LLM response content.

    Defense-in-depth: catches any PII the LLM might generate.
    """
    start_time = time.time()

    try:
        redactor = get_or_create_redactor()
    except Exception as e:
        raise HTTPException(status_code=503, detail=str(e))

    redacted_content = redactor.redact_response(request.content)

    processing_time = (time.time() - start_time) * 1000

    # Count redactions by comparing original vs redacted
    redactions_made = 0
    for result in redactor.analyzer.analyze(
        text=request.content,
        language="en",
        score_threshold=redactor.min_score_threshold,
    ):
        redactions_made += 1

    return RedactResponseContent(
        agent_id=request.agent_id,
        content=redacted_content,
        redactions_made=redactions_made,
        processing_time_ms=round(processing_time, 2),
    )


@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    return HealthResponse(
        status="ok",
        version="1.0.0",
        uptime_seconds=round(time.time() - _start_time, 2),
    )


@app.get("/stats", response_model=StatsResponse)
async def get_stats():
    """Get redaction statistics."""
    try:
        redactor = get_or_create_redactor()
        return StatsResponse(
            total_redactions=redactor.stats["total_redactions"],
            redacted_fields=dict(redactor.stats["redacted_fields"]),
        )
    except Exception:
        return StatsResponse(total_redactions=0, redacted_fields={})


@app.post("/stats/reset")
async def reset_stats():
    """Reset redaction statistics."""
    try:
        redactor = get_or_create_redactor()
        redactor.reset_statistics()
        return {"status": "ok", "message": "Statistics reset"}
    except Exception as e:
        raise HTTPException(status_code=503, detail=str(e))


@app.get("/validate")
async def validate_text(text: str):
    """
    Check if text contains PII without redacting it.
    Returns detection results for analysis.
    """
    try:
        redactor = get_or_create_redactor()
    except Exception as e:
        raise HTTPException(status_code=503, detail=str(e))

    results = redactor.analyzer.analyze(
        text=text,
        language="en",
        score_threshold=redactor.min_score_threshold,
    )

    findings = []
    for result in results:
        findings.append({
            "entity_type": result.entity_type,
            "start": result.start,
            "end": result.end,
            "score": result.score,
            "text_snippet": text[result.start:result.end],
        })

    return {
        "clean": len(findings) == 0,
        "findings": findings,
        "total_findings": len(findings),
    }


if __name__ == "__main__":
    import uvicorn

    port = int(os.getenv("PORT", "8000"))
    host = os.getenv("HOST", "0.0.0.0")

    uvicorn.run(
        "app.main:app",
        host=host,
        port=port,
        reload=os.getenv("DEV_MODE", "").lower() == "true",
        log_level=os.getenv("LOG_LEVEL", "info").lower(),
    )
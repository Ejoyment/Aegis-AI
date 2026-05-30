"""
AegisAI PII Redaction Engine
=============================
Uses Microsoft Presidio for PII detection and anonymization.
Performs entirely on-premise to ensure sensitive data never leaves
the client's network.

Detected PII types:
- EMAIL_ADDRESS
- PHONE_NUMBER
- CREDIT_CARD
- SSN (US Social Security Number)
- PERSON (person name via NLP)
- LOCATION (address, city, state via NLP)
- IP_ADDRESS
- URL
- CRYPTO (Bitcoin, Ethereum addresses)
- BANK_ACCOUNT
- DATE_TIME (for sensitive date references)
"""

import json
import logging
import re
from typing import List, Dict, Any, Optional

from presidio_analyzer import AnalyzerEngine, Pattern, PatternRecognizer
from presidio_anonymizer import AnonymizerEngine
from presidio_anonymizer.entities import OperatorConfig

logger = logging.getLogger(__name__)


class PIIRedactor:
    """
    PII redactor that detects and replaces personally identifiable
    information in LLM prompts using Presidio.
    """

    # Custom patterns for high-precision detection
    CUSTOM_PATTERNS = [
        # US SSN: 123-45-6789
        {
            "name": "SSN",
            "regex": r"\b\d{3}-\d{2}-\d{4}\b",
            "score": 0.85,
        },
        # API keys / secrets (e.g., sk-...)
        {
            "name": "API_KEY",
            "regex": r"(?:sk|pk|api|token|secret|key)[-_]?[a-zA-Z0-9]{16,64}",
            "score": 0.9,
        },
        # Bitcoin address
        {
            "name": "CRYPTO_ADDRESS",
            "regex": r"\b[13][a-km-zA-HJ-NP-Z1-9]{25,34}\b",
            "score": 0.8,
        },
        # Ethereum address
        {
            "name": "ETH_ADDRESS",
            "regex": r"\b0x[a-fA-F0-9]{40}\b",
            "score": 0.8,
        },
        # Internal project/codename patterns (customizable)
        {
            "name": "INTERNAL_CODE",
            "regex": r"\b[A-Z]{2,6}-\d{4,6}\b",
            "score": 0.6,
        },
        # JWT tokens
        {
            "name": "JWT_TOKEN",
            "regex": r"eyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}",
            "score": 0.95,
        },
        # AWS keys
        {
            "name": "AWS_ACCESS_KEY",
            "regex": r"AKIA[0-9A-Z]{16}",
            "score": 0.95,
        },
    ]

    def __init__(
        self,
        supported_languages: Optional[List[str]] = None,
        min_score_threshold: float = 0.5,
    ):
        """
        Initialize the PII redactor with Presidio analyzers.

        Args:
            supported_languages: List of language codes (default: ["en"])
            min_score_threshold: Minimum confidence score for PII detection (0.0-1.0)
        """
        self.supported_languages = supported_languages or ["en"]
        self.min_score_threshold = min_score_threshold

        # Initialize Presidio analyzer
        self.analyzer = AnalyzerEngine(
            supported_languages=self.supported_languages,
        )

        # Register custom recognizers
        self._register_custom_recognizers()

        # Initialize anonymizer
        self.anonymizer = AnonymizerEngine()

        # Statistics tracking
        self.stats = {
            "total_redactions": 0,
            "redacted_fields": {},
        }

        logger.info(
            "PII Redactor initialized with %d custom patterns",
            len(self.CUSTOM_PATTERNS),
        )

    def _register_custom_recognizers(self) -> None:
        """Register custom PII pattern recognizers with Presidio."""
        for pattern_def in self.CUSTOM_PATTERNS:
            pattern = Pattern(
                name=pattern_def["name"],
                regex=pattern_def["regex"],
                score=pattern_def["score"],
            )
            recognizer = PatternRecognizer(
                supported_entity=pattern_def["name"],
                patterns=[pattern],
                supported_language=self.supported_languages[0],
            )
            self.analyzer.registry.add_recognizer(recognizer)
            logger.debug("Registered recognizer: %s", pattern_def["name"])

    def redact_messages(
        self,
        messages: List[Dict[str, Any]],
        agent_id: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """
        Redact PII from a list of chat messages.

        Args:
            messages: List of message dicts with 'role' and 'content' keys.
            agent_id: Optional agent identifier for statistics tracking.

        Returns:
            List of messages with PII redacted from content fields.
        """
        redacted_messages = []
        agent_stats = {"messages_scanned": 0, "redactions_made": 0}

        for message in messages:
            redacted_msg = self._redact_single_message(message, agent_stats)
            redacted_messages.append(redacted_msg)

        if agent_id:
            self.stats["total_redactions"] += agent_stats["redactions_made"]
            logger.info(
                "Agent %s: scanned %d messages, made %d redactions",
                agent_id,
                agent_stats["messages_scanned"],
                agent_stats["redactions_made"],
            )

        return redacted_messages

    def _redact_single_message(
        self,
        message: Dict[str, Any],
        stats: Dict[str, int],
    ) -> Dict[str, Any]:
        """
        Redact PII from a single message.

        Handles both simple string content and multi-modal arrays.
        """
        redacted = dict(message)
        content = message.get("content", "")

        if isinstance(content, str):
            # Simple text content
            redacted["content"] = self._redact_text(content, stats)
            stats["messages_scanned"] += 1

        elif isinstance(content, list):
            # Multi-modal content (text + images)
            redacted_parts = []
            for part in content:
                if isinstance(part, dict) and part.get("type") == "text":
                    part_copy = dict(part)
                    part_copy["text"] = self._redact_text(
                        part.get("text", ""), stats
                    )
                    redacted_parts.append(part_copy)
                else:
                    redacted_parts.append(part)
            redacted["content"] = redacted_parts
            stats["messages_scanned"] += 1

        return redacted

    def _redact_text(self, text: str, stats: Dict[str, int]) -> str:
        """
        Detect and redact PII from a text string.

        Uses Presidio analyzer to detect entities, then anonymizes them.
        """
        if not text or not text.strip():
            return text

        # Analyze text for PII
        analyzer_results = self.analyzer.analyze(
            text=text,
            language=self.supported_languages[0],
            score_threshold=self.min_score_threshold,
        )

        if not analyzer_results:
            return text

        # Build anonymizer operators for detected entity types
        operators = {}
        for result in analyzer_results:
            entity_type = result.entity_type
            if entity_type not in operators:
                operators[entity_type] = OperatorConfig(
                    operator_name="replace",
                    params={
                        "new_value": f"[REDACTED - {entity_type}]"
                    },
                )

        # Anonymize the text
        anonymized_result = self.anonymizer.anonymize(
            text=text,
            analyzer_results=analyzer_results,
            operators=operators,
        )

        redacted_text = anonymized_result.text
        stats["redactions_made"] += len(analyzer_results)

        # Track per-field statistics
        for result in analyzer_results:
            field = result.entity_type
            if field not in self.stats["redacted_fields"]:
                self.stats["redacted_fields"][field] = 0
            self.stats["redacted_fields"][field] += 1

        return redacted_text

    def redact_response(self, content: str) -> str:
        """
        Redact PII from LLM responses as well (defense in depth).

        This catches any PII that the LLM might generate in its output.
        """
        return self._redact_text(content, {"redactions_made": 0})

    def get_statistics(self) -> Dict[str, Any]:
        """Return current redaction statistics."""
        return {
            "total_redactions": self.stats["total_redactions"],
            "redacted_fields": dict(self.stats["redacted_fields"]),
        }

    def reset_statistics(self) -> None:
        """Reset redaction statistics."""
        self.stats = {"total_redactions": 0, "redacted_fields": {}}

    def validate_no_pii(self, text: str) -> bool:
        """
        Verify that a text contains no detectable PII.
        Returns True if clean (no PII found), False if PII detected.
        """
        results = self.analyzer.analyze(
            text=text,
            language=self.supported_languages[0],
            score_threshold=self.min_score_threshold,
        )
        return len(results) == 0


# Global redactor instance (lazy-initialized)
_redactor: Optional[PIIRedactor] = None


def get_redactor() -> PIIRedactor:
    """Get or create the global PII redactor singleton."""
    global _redactor
    if _redactor is None:
        _redactor = PIIRedactor()
    return _redactor
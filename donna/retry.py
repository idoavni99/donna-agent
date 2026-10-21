from __future__ import annotations

import http.client
import errno
import random
import socket
import ssl
import time
import urllib.error
from collections.abc import Callable
from typing import TypeVar

from donna.constants import (
    HTTP_MAX_ATTEMPTS,
    HTTP_RETRY_BASE_DELAY_SECONDS,
    HTTP_RETRY_MAX_DELAY_SECONDS,
    HTTP_RETRYABLE_STATUS_CODES,
)

T = TypeVar("T")
_RETRYABLE_NETWORK_ERRNOS = frozenset(
    getattr(errno, name)
    for name in (
        "ECONNABORTED",
        "ECONNREFUSED",
        "ECONNRESET",
        "EHOSTUNREACH",
        "ENETUNREACH",
        "EPIPE",
        "ETIMEDOUT",
    )
    if hasattr(errno, name)
)


def is_retryable_status(status_code: int) -> bool:
    return status_code in HTTP_RETRYABLE_STATUS_CODES or 500 <= status_code < 600


class TransientHTTPStatus(Exception):
    def __init__(self, status_code: int) -> None:
        super().__init__(f"temporary HTTP status {status_code}")
        self.status_code = status_code


def _is_retryable(exc: Exception) -> bool:
    if isinstance(exc, TransientHTTPStatus):
        return is_retryable_status(exc.status_code)
    if isinstance(exc, urllib.error.HTTPError):
        return is_retryable_status(exc.code)
    if isinstance(exc, ssl.SSLCertVerificationError):
        return False
    if isinstance(exc, urllib.error.URLError):
        return isinstance(exc.reason, Exception) and _is_retryable(exc.reason)
    if isinstance(exc, socket.gaierror):
        return exc.errno == socket.EAI_AGAIN
    if isinstance(exc, (ConnectionError, TimeoutError)):
        return True
    if isinstance(exc, OSError):
        return exc.errno in _RETRYABLE_NETWORK_ERRNOS
    return isinstance(exc, (http.client.HTTPException, ssl.SSLError))


def retry_request(operation: Callable[[], T]) -> T:
    """Retry a replayable request on transient failures with bounded full jitter."""
    for attempt in range(HTTP_MAX_ATTEMPTS):
        try:
            return operation()
        except Exception as exc:
            retryable = _is_retryable(exc)
            if isinstance(exc, urllib.error.HTTPError):
                exc.close()
            if attempt + 1 == HTTP_MAX_ATTEMPTS or not retryable:
                raise
            delay_ceiling = min(
                HTTP_RETRY_BASE_DELAY_SECONDS * 2**attempt,
                HTTP_RETRY_MAX_DELAY_SECONDS,
            )
            time.sleep(random.uniform(0, delay_ceiling))
    raise RuntimeError("retry loop exhausted unexpectedly")

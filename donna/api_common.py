from __future__ import annotations

import hmac
import json
from typing import Annotated, Any

from fastapi import Depends, HTTPException, Request
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

bearer = HTTPBearer(auto_error=False)


class Service:
    def __init__(
        self,
        classifier: Any,
        token: str,
        minimum: float,
        chat: Any = None,
        tools: Any = None,
    ) -> None:
        self.classifier, self.chat, self.tools = classifier, chat, tools
        self.token = token
        self.minimum = minimum

    async def authorize(
        self, credentials: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer)]
    ) -> None:
        if not self.token:
            raise HTTPException(status_code=503, detail="model API is not configured")
        if (
            credentials is None
            or credentials.scheme.lower() != "bearer"
            or not hmac.compare_digest(credentials.credentials, self.token)
        ):
            raise HTTPException(
                status_code=401, detail="unauthorized", headers={"WWW-Authenticate": "Bearer"}
            )


async def parse_json_object(
    request: Request,
    allowed: set[str],
    required: set[str] | None = None,
    error_message: str = "invalid request JSON",
) -> dict:
    try:
        data = json.loads(await request.body())
    except json.JSONDecodeError, UnicodeDecodeError:
        raise HTTPException(400, error_message) from None
    if not isinstance(data, dict) or data.keys() - allowed or (required and required - data.keys()):
        raise HTTPException(400, error_message)
    return data

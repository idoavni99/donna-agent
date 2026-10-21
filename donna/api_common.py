from __future__ import annotations

import hmac
import json
from typing import Annotated, Any

from donna.auth import JWTAuth, User

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
        google: Any = None,
        auth: JWTAuth | None = None,
        database: Any = None,
    ) -> None:
        self.classifier, self.chat, self.tools, self.google, self.auth, self.database = (
            classifier, chat, tools, google, auth, database
        )
        self.token = token
        self.minimum = minimum


    async def current_user(
        self, credentials: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer)]
    ) -> User | None:
        return await self.authorize(credentials)

    async def authorize(
        self, credentials: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer)]
    ) -> User | None:
        if self.auth:
            if credentials is None or credentials.scheme.lower() != "bearer":
                raise HTTPException(status_code=401, detail="unauthorized", headers={"WWW-Authenticate": "Bearer"})
            user = await self.auth.authenticate(credentials.credentials)
            if user is None:
                raise HTTPException(status_code=401, detail="unauthorized", headers={"WWW-Authenticate": "Bearer"})
            return user
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

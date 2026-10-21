from __future__ import annotations

import uuid

from fastapi import Depends, FastAPI, HTTPException, Request

from donna.api_common import Service, parse_json_object
from donna.auth import User


def register_account_routes(app: FastAPI, service: Service) -> None:
    if service.auth is None:
        return

    @app.post("/v1/auth/login")
    async def login(request: Request) -> dict[str, str]:
        payload = await parse_json_object(request, {"email"}, required={"email"})
        email = payload["email"]
        if not isinstance(email, str) or "@" not in email or len(email) > 320:
            raise HTTPException(400, "valid email is required")
        user = User(str(uuid.uuid5(uuid.NAMESPACE_URL, "donna:" + email.lower())), email.lower())
        return {"access_token": await service.auth.issue(user), "token_type": "bearer"}

    @app.get("/v1/auth/me")
    async def me(user: User = Depends(service.current_user)) -> dict[str, str]:
        if user is None:
            raise HTTPException(401, "unauthorized")
        return {"id": user.id, "email": user.email}

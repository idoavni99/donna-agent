from __future__ import annotations

from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.responses import RedirectResponse

from donna.api_common import Service, parse_json_object
from donna.auth import User
from donna.google_workspace import approval_json


def register_google_routes(app: FastAPI, service: Service) -> None:
    google = service.google
    if google is None:
        return

    @app.get("/v1/integrations/google/authorize")
    async def google_authorize(user: User = Depends(service.current_user)) -> RedirectResponse:
        if user is None:
            raise HTTPException(401, "unauthorized")
        return RedirectResponse(google.oauth.authorization_url(user.id))

    @app.get("/v1/integrations/google/callback")
    async def google_callback(state: str, code: str) -> dict[str, str]:
        try:
            google.oauth.complete(state, code)
        except Exception as exc:
            raise HTTPException(400, "Google authorization failed") from exc
        return {"status": "connected"}

    @app.delete("/v1/integrations/google")
    async def google_disconnect(user: User = Depends(service.current_user)) -> dict[str, str]:
        if user is None:
            raise HTTPException(401, "unauthorized")
        google.oauth.store.delete(user.id)
        return {"status": "disconnected"}

    @app.get("/v1/approvals")
    async def list_approvals(user: User = Depends(service.current_user)) -> list[dict]:
        if user is None:
            raise HTTPException(401, "unauthorized")
        return [
            approval_json(item)
            for item in google.approvals.items()
            if item.user_id == user.id and item.status == "pending"
        ]

    @app.post("/v1/approvals/{approval_id}/approve")
    async def approve(approval_id: str, user: User = Depends(service.current_user)) -> dict:
        if user is None:
            raise HTTPException(401, "unauthorized")
        approval = google.approvals.get(approval_id)
        if approval is None or approval.user_id != user.id:
            raise HTTPException(404, "approval not found")
        if approval.status != "pending" or approval.expires_at <= google.clock():
            raise HTTPException(409, "approval expired or already resolved")
        # Execution is deliberately isolated behind this service boundary. The Google
        # Workspace MCP toolset can be connected here without changing the HTTP contract.
        updated = google.approvals.update(approval, "approved")
        return approval_json(updated)

    @app.post("/v1/approvals/{approval_id}/reject")
    async def reject(approval_id: str, user: User = Depends(service.current_user)) -> dict:
        if user is None:
            raise HTTPException(401, "unauthorized")
        approval = google.approvals.get(approval_id)
        if approval is None or approval.user_id != user.id:
            raise HTTPException(404, "approval not found")
        if approval.status != "pending":
            raise HTTPException(409, "approval already resolved")
        return approval_json(google.approvals.update(approval, "rejected"))

    @app.post("/v1/integrations/google/proposals")
    async def proposal(request: Request, user: User = Depends(service.current_user)) -> dict:
        if user is None:
            raise HTTPException(401, "unauthorized")
        payload = await parse_json_object(
            request, {"action", "payload"}, required={"action", "payload"}
        )
        if not isinstance(payload["action"], str) or not isinstance(payload["payload"], dict):
            raise HTTPException(400, "invalid proposal JSON")
        if payload["action"] not in google.mutating_actions:
            raise HTTPException(400, "unsupported Google action")
        return approval_json(
            google.approvals.create(user.id, payload["action"], payload["payload"])
        )

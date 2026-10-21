from fastapi import Depends, FastAPI, HTTPException, Request

from donna.api_common import Service, parse_json_object
from donna.auth import User
from donna.chat import ChatError


def register_chat_routes(app: FastAPI, service: Service) -> None:
    if service.chat is None:
        return

    @app.post("/v1/chat")
    async def chat(
        request: Request, user: User | None = Depends(service.current_user)
    ) -> dict[str, str]:
        if user is None:
            raise HTTPException(401, "unauthorized")
        payload = await parse_json_object(
            request,
            {"message", "session_id"},
            required={"message"},
            error_message="invalid chat JSON",
        )
        message = payload["message"]
        session_id = payload.get("session_id", "")
        if not isinstance(message, str) or not isinstance(session_id, str):
            raise HTTPException(400, "invalid chat JSON")
        try:
            return await service.chat.reply(message, session_id, user.id)
        except ChatError as exc:
            raise HTTPException(exc.status_code, exc.message) from exc
        except Exception as exc:
            raise HTTPException(502, "agent completion failed") from exc

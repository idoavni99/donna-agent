from __future__ import annotations

import hmac
import json
import logging
from pathlib import Path
import time
from typing import Annotated, Any

from fastapi import Depends, FastAPI, HTTPException, Request, Response
from fastapi.responses import JSONResponse, PlainTextResponse
from fastapi.openapi.docs import get_swagger_ui_html
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from pydantic import ValidationError
from pydantic_ai.exceptions import UnexpectedModelBehavior

from donna.decision import Action, CheckResult, DecisionResult, evaluate, route
from donna.chat import ChatError

MAX_REQUEST_BYTES = 16 * 1024
bearer = HTTPBearer(auto_error=False)


class RequestSizeLimit:
    def __init__(self, app, limit: int = MAX_REQUEST_BYTES) -> None:
        self.app = app
        self.limit = limit

    async def __call__(self, scope, receive, send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        started, status_code = time.monotonic(), 0

        async def logged_send(message):
            nonlocal status_code
            if message["type"] == "http.response.start":
                status_code = message["status"]
            await send(message)

        messages = []
        size = 0
        while True:
            message = await receive()
            messages.append(message)
            size += len(message.get("body", b""))
            if size > self.limit:
                error = (
                    "invalid chat JSON"
                    if scope.get("path") == "/v1/chat"
                    else "tool call requires one name and arguments object"
                    if scope.get("path") == "/v1/tools/call"
                    else "invalid request JSON"
                )
                response = PlainTextResponse(error, status_code=400)
                await response(scope, receive, logged_send)
                _log_request(scope, status_code, started)
                return
            if not message.get("more_body", False):
                break
        sent = False

        async def replay():
            nonlocal sent
            if not sent:
                sent = True
                return {
                    "type": "http.request",
                    "body": b"".join(m.get("body", b"") for m in messages),
                    "more_body": False,
                }
            return await receive()

        await self.app(scope, replay, logged_send)
        _log_request(scope, status_code, started)


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


def create_app(service: Service) -> FastAPI:
    app = FastAPI(title="Donna", version="1.0.0", docs_url=None, redoc_url=None, openapi_url=None)
    app.add_middleware(RequestSizeLimit)

    @app.exception_handler(HTTPException)
    async def plain_http_errors(request: Request, exc: HTTPException) -> PlainTextResponse:
        return PlainTextResponse(
            f"{exc.detail}\n", status_code=exc.status_code, headers=exc.headers
        )

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/healthz", status_code=204)
    async def healthz() -> Response:
        return Response(status_code=204)

    @app.get("/openapi.yaml", include_in_schema=False)
    async def openapi_yaml() -> Response:
        spec = Path(__file__).with_name("openapi.yaml").read_text(encoding="utf-8")
        return Response(spec, media_type="application/yaml")

    @app.get("/swagger", include_in_schema=False)
    async def swagger() -> Response:
        return get_swagger_ui_html(openapi_url="/openapi.yaml", title="Donna API")

    @app.post(
        "/v1/decisions", response_model=DecisionResult, dependencies=[Depends(service.authorize)]
    )
    async def decide(request: Request) -> DecisionResult:
        payload = await _parse(request, {"query"}, required={"query"})
        query = payload["query"]
        if not isinstance(query, str) or not query.strip():
            raise HTTPException(400, "query is required")
        try:
            scores = await service.classifier.classify(query)
            return route(scores, service.minimum)
        except UnexpectedModelBehavior:
            return DecisionResult(action=Action.CLARIFY, confidence=0)
        except Exception as exc:
            if isinstance(exc, ValidationError):
                return DecisionResult(action=Action.CLARIFY, confidence=0)
            raise HTTPException(502, "decision service unavailable") from exc

    @app.post("/v1/checks", response_model=CheckResult, dependencies=[Depends(service.authorize)])
    async def check(request: Request) -> CheckResult:
        payload = await _parse(request, {"question", "evidence"}, required={"question"})
        question = payload["question"]
        evidence = payload.get("evidence", "")
        if not isinstance(question, str) or not question.strip():
            raise HTTPException(400, "question is required")
        if not isinstance(evidence, str):
            raise HTTPException(400, "invalid request JSON")
        try:
            scores = await service.classifier.evaluate(question, evidence)
            return evaluate(scores, service.minimum)
        except UnexpectedModelBehavior:
            return CheckResult(answer=None, confidence=0)
        except Exception as exc:
            if isinstance(exc, ValidationError):
                return CheckResult(answer=None, confidence=0)
            raise HTTPException(502, "decision service unavailable") from exc

    if service.chat is not None:

        @app.post("/v1/chat", dependencies=[Depends(service.authorize)])
        async def chat(request: Request) -> dict[str, str]:
            payload = await _parse(
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
                return await service.chat.reply(message, session_id)
            except ChatError as exc:
                raise HTTPException(exc.status_code, exc.message) from exc
            except Exception as exc:
                raise HTTPException(502, "agent completion failed") from exc

    if service.tools is not None:

        @app.get("/v1/tools", dependencies=[Depends(service.authorize)])
        async def list_tools() -> list[dict]:
            return service.tools.definitions()

        @app.post("/v1/tools/call", dependencies=[Depends(service.authorize)])
        async def call_tool(request: Request):
            error = "tool call requires one name and arguments object"
            payload = await _parse(
                request, {"name", "arguments"}, required={"name", "arguments"}, error_message=error
            )
            if (
                not isinstance(payload["name"], str)
                or not payload["name"]
                or not isinstance(payload["arguments"], dict)
            ):
                raise HTTPException(400, "tool call requires one name and arguments object")
            try:
                return JSONResponse(
                    await service.tools.invoke(payload["name"], payload["arguments"])
                )
            except KeyError as exc:
                raise HTTPException(404, "unknown tool") from exc
            except LookupError as exc:
                raise HTTPException(503, str(exc)) from exc
            except (ValueError, ValidationError) as exc:
                raise HTTPException(400, "invalid tool arguments") from exc
            except Exception as exc:
                raise HTTPException(502, "tool execution failed") from exc

    return app


async def _parse(
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


def _log_request(scope: dict, status_code: int, started: float) -> None:
    path = scope.get("path", "")
    known = {
        "/healthz",
        "/health",
        "/v1/decisions",
        "/v1/checks",
        "/v1/chat",
        "/v1/tools",
        "/v1/tools/call",
        "/openapi.yaml",
        "/swagger",
    }
    level = (
        logging.ERROR
        if status_code >= 500
        else logging.WARNING
        if status_code >= 400
        else logging.INFO
    )
    logging.getLogger("donna.http").log(
        level,
        json.dumps(
            {
                "method": scope.get("method", ""),
                "route": path if path in known else "unmatched",
                "status": status_code,
                "duration_ms": round((time.monotonic() - started) * 1000),
            }
        ),
    )

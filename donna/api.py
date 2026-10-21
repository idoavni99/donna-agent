from __future__ import annotations

import json
import logging
from pathlib import Path
import time

from fastapi import Depends, FastAPI, HTTPException, Request, Response
from fastapi.responses import PlainTextResponse
from fastapi.openapi.docs import get_swagger_ui_html
from pydantic import ValidationError
from pydantic_ai.exceptions import UnexpectedModelBehavior

from donna.api_common import Service, parse_json_object as _parse
from donna.chat_routes import register_chat_routes
from donna.constants import API_MAX_REQUEST_BODY_BYTES
from donna.decision import Action, CheckResult, DecisionResult, evaluate, route
from donna.tool_routes import register_tool_routes
from donna.google_routes import register_google_routes
from donna.account_routes import register_account_routes


class RequestSizeLimit:
    def __init__(self, app, limit: int = API_MAX_REQUEST_BODY_BYTES) -> None:
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

    register_chat_routes(app, service)
    register_tool_routes(app, service)
    register_google_routes(app, service)
    register_account_routes(app, service)

    return app


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

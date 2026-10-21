from __future__ import annotations

import asyncio
import http.client
import json
import ssl
import urllib.error
import urllib.request
from typing import Any
from urllib.parse import urlsplit, urlunsplit

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator
from pydantic_ai.exceptions import UnexpectedModelBehavior

from donna.constants import (
    PROFILE_SEARCH_MAX_CANDIDATES,
    PROFILE_SEARCH_MAX_RESPONSE_BYTES,
    PROFILE_SEARCH_MAX_TEXT_CHARS,
    PROFILE_SEARCH_TIMEOUT_SECONDS,
    PROFILE_NAME_MAX_CHARS,
    PROFILE_CONTEXT_MAX_CHARS,
    PUBLIC_URL_MAX_CHARS,
)
from donna.decision import Action, CheckResult, DecisionResult, evaluate, route
from donna.retry import retry_request

TOOL_SCHEMAS = [
    {
        "name": "donna_route_query",
        "description": "Choose how Donna should handle a user query before retrieving memory.",
        "input_schema": {
            "type": "object",
            "properties": {"query": {"type": "string"}},
            "required": ["query"],
            "additionalProperties": False,
        },
    },
    {
        "name": "donna_check_yes_no",
        "description": "Evaluate a yes/no question against evidence. A null answer means uncertain.",
        "input_schema": {
            "type": "object",
            "properties": {"question": {"type": "string"}, "evidence": {"type": "string"}},
            "required": ["question", "evidence"],
            "additionalProperties": False,
        },
    },
]


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True)


class RouteInput(StrictModel):
    query: str


class CheckInput(StrictModel):
    question: str
    evidence: str


class PersonLookup(StrictModel):
    first_name: str = Field(min_length=1, max_length=PROFILE_NAME_MAX_CHARS)
    last_name: str = Field(min_length=1, max_length=PROFILE_NAME_MAX_CHARS)
    work_field: str = Field(default="", max_length=PROFILE_CONTEXT_MAX_CHARS)
    location: str = Field(default="", max_length=PROFILE_CONTEXT_MAX_CHARS)

    @field_validator("first_name", "last_name", "work_field", "location", mode="before")
    @classmethod
    def trim_fields(cls, value):
        return value.strip() if isinstance(value, str) else value

    @model_validator(mode="after")
    def require_context(self):
        if not self.work_field.strip() and not self.location.strip():
            raise ValueError("work_field or location is required")
        return self


class ProfileSearcher:
    def __init__(self, endpoint: str, api_key: str = "") -> None:
        parsed = urlsplit(endpoint)
        if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password:
            raise ValueError("profile search endpoint must be an absolute HTTPS URL")
        self.endpoint, self.api_key = endpoint, api_key

    async def search(self, lookup: PersonLookup) -> list[dict[str, str]]:
        body = json.dumps(lookup.model_dump()).encode()
        return await asyncio.to_thread(self._request, body)

    def _request(self, body: bytes) -> list[dict[str, str]]:
        headers = {"Content-Type": "application/json", "Accept": "application/json"}
        if self.api_key:
            headers["Authorization"] = "Bearer " + self.api_key
        request = urllib.request.Request(self.endpoint, body, headers)
        opener = urllib.request.build_opener(
            _NoRedirect(), urllib.request.HTTPSHandler(context=ssl.create_default_context())
        )

        def send_request() -> bytes:
            with opener.open(request, timeout=PROFILE_SEARCH_TIMEOUT_SECONDS) as response:
                if not response.geturl().startswith("https://"):
                    raise ValueError("profile search provider redirected outside HTTPS")
                return response.read(PROFILE_SEARCH_MAX_RESPONSE_BYTES + 1)

        try:
            raw = retry_request(send_request)
        except (OSError, urllib.error.URLError, http.client.HTTPException) as exc:
            raise RuntimeError("profile search provider unavailable") from exc
        if len(raw) > PROFILE_SEARCH_MAX_RESPONSE_BYTES:
            raise RuntimeError(
                f"profile search response exceeds {PROFILE_SEARCH_MAX_RESPONSE_BYTES // 1024} KiB"
            )
        try:
            result = json.loads(raw)
            if (
                not isinstance(result, dict)
                or set(result) != {"candidates"}
                or not isinstance(result["candidates"], list)
                or len(result["candidates"]) > PROFILE_SEARCH_MAX_CANDIDATES
            ):
                raise ValueError("invalid candidate list")
        except (json.JSONDecodeError, ValueError) as exc:
            raise RuntimeError("profile search provider returned invalid JSON") from exc
        candidates = []
        for item in result["candidates"]:
            if (
                not isinstance(item, dict)
                or set(item) != {"url", "platform", "display_name"}
                or not all(isinstance(item[key], str) for key in item)
                or any(
                    len(item[key]) > PROFILE_SEARCH_MAX_TEXT_CHARS
                    for key in ("platform", "display_name")
                )
            ):
                raise RuntimeError("profile search provider returned an invalid candidate")
            try:
                url = normalize_profile_url(item["url"])
            except ValueError:
                continue
            candidates.append(
                {
                    "url": url,
                    "platform": item["platform"].strip(),
                    "display_name": item["display_name"].strip(),
                    "identity_confirmed": False,
                }
            )
        return candidates


class Tools:
    def __init__(
        self, classifier: Any, minimum: float, profile_searcher: ProfileSearcher | None = None
    ) -> None:
        self.classifier, self.minimum, self.profile_searcher = classifier, minimum, profile_searcher

    def definitions(self) -> list[dict]:
        definitions = list(TOOL_SCHEMAS)
        if self.profile_searcher:
            definitions.append(
                {
                    "name": "donna_find_person_profiles",
                    "description": "Find up to five unverified public profile candidates by full name and work field or broad location.",
                    "input_schema": {
                        "type": "object",
                        "properties": {
                            "first_name": {
                                "type": "string",
                                "minLength": 1,
                                "maxLength": PROFILE_NAME_MAX_CHARS,
                            },
                            "last_name": {
                                "type": "string",
                                "minLength": 1,
                                "maxLength": PROFILE_NAME_MAX_CHARS,
                            },
                            "work_field": {
                                "type": "string",
                                "maxLength": PROFILE_CONTEXT_MAX_CHARS,
                            },
                            "location": {
                                "type": "string",
                                "maxLength": PROFILE_CONTEXT_MAX_CHARS,
                            },
                        },
                        "required": ["first_name", "last_name"],
                        "additionalProperties": False,
                    },
                }
            )
        return definitions

    async def invoke(self, name: str, arguments: dict) -> Any:
        if name == "donna_route_query":
            route_input = RouteInput.model_validate(arguments)
            if not route_input.query.strip():
                raise InvalidToolArguments("query is required")
            try:
                result = route(await self.classifier.classify(route_input.query), self.minimum)
            except UnexpectedModelBehavior:
                result = DecisionResult(action=Action.CLARIFY, confidence=0)
            return result.model_dump(mode="json")
        if name == "donna_check_yes_no":
            check_input = CheckInput.model_validate(arguments)
            if not check_input.question.strip() or not check_input.evidence.strip():
                raise InvalidToolArguments("question and evidence are required")
            try:
                check_result = evaluate(
                    await self.classifier.evaluate(check_input.question, check_input.evidence),
                    self.minimum,
                )
            except UnexpectedModelBehavior:
                check_result = CheckResult(answer=None, confidence=0)
            return check_result.model_dump(mode="json")
        if name == "donna_find_person_profiles" and self.profile_searcher:
            lookup = PersonLookup.model_validate(arguments)
            return {
                "candidates": await self.profile_searcher.search(lookup),
                "note": "Search results are unverified candidates. Confirm a profile URL before researching it; name, work field, and location do not prove identity.",
            }
        if name == "donna_find_person_profiles":
            raise LookupError("public profile search is not configured")
        raise KeyError("unknown tool")


class InvalidToolArguments(ValueError):
    pass


def normalize_profile_url(raw: str) -> str:
    if len(raw) > PUBLIC_URL_MAX_CHARS:
        raise ValueError("profile candidate URL is too long")
    parsed = urlsplit(raw.strip())
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password:
        raise ValueError("profile candidate URL must use HTTPS")
    host = parsed.hostname.lower()
    netloc = f"[{host}]" if ":" in host else host
    if parsed.port and parsed.port != 443:
        netloc += f":{parsed.port}"
    return urlunsplit(("https", netloc, parsed.path.rstrip("/"), "", ""))


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise urllib.error.HTTPError(req.full_url, code, "redirects are not followed", headers, fp)

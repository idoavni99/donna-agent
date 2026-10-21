from __future__ import annotations

import hashlib
import hmac
import json
import secrets
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any, Protocol


@dataclass(frozen=True)
class GoogleOAuthSettings:
    client_id: str
    client_secret: str
    redirect_uri: str
    scopes: tuple[str, ...] = (
        "openid",
        "email",
        "https://www.googleapis.com/auth/gmail.readonly",
        "https://www.googleapis.com/auth/calendar.readonly",
        "https://www.googleapis.com/auth/gmail.send",
        "https://www.googleapis.com/auth/calendar.events",
    )


@dataclass
class GoogleCredential:
    refresh_token: str
    access_token: str = ""
    expires_at: float = 0.0
    email: str = ""


class CredentialStore(Protocol):
    async def save(self, user_id: str, credential: GoogleCredential) -> None: ...
    async def get(self, user_id: str) -> GoogleCredential | None: ...
    async def delete(self, user_id: str) -> None: ...


class InMemoryCredentialStore:
    """Replaceable store boundary; production persistence can implement CredentialStore."""

    def __init__(self) -> None:
        self._items: dict[str, GoogleCredential] = {}

    async def save(self, user_id: str, credential: GoogleCredential) -> None:
        self._items[user_id] = credential

    async def get(self, user_id: str) -> GoogleCredential | None:
        return self._items.get(user_id)

    async def delete(self, user_id: str) -> None:
        self._items.pop(user_id, None)


class GoogleOAuth:
    def __init__(self, settings: GoogleOAuthSettings, store: CredentialStore) -> None:
        self.settings, self.store = settings, store
        self._states: dict[str, tuple[str, float, str]] = {}

    def authorization_url(self, user_id: str) -> str:
        state = secrets.token_urlsafe(32)
        verifier = secrets.token_urlsafe(48)
        self._states[state] = (user_id, time.time() + 600, verifier)
        query = urllib.parse.urlencode(
            {
                "client_id": self.settings.client_id,
                "redirect_uri": self.settings.redirect_uri,
                "response_type": "code",
                "scope": " ".join(self.settings.scopes),
                "access_type": "offline",
                "prompt": "consent",
                "state": state,
                "code_challenge": hashlib.sha256(verifier.encode()).digest().hex(),
                "code_challenge_method": "S256",
            }
        )
        return "https://accounts.google.com/o/oauth2/v2/auth?" + query

    async def complete(self, state: str, code: str) -> str:
        item = self._states.pop(state, None)
        if item is None or item[1] < time.time():
            raise ValueError("invalid or expired OAuth state")
        user_id, _, verifier = item
        payload = urllib.parse.urlencode(
            {
                "code": code,
                "client_id": self.settings.client_id,
                "client_secret": self.settings.client_secret,
                "redirect_uri": self.settings.redirect_uri,
                "grant_type": "authorization_code",
                "code_verifier": verifier,
            }
        ).encode()
        request = urllib.request.Request(
            "https://oauth2.googleapis.com/token",
            payload,
            {"Content-Type": "application/x-www-form-urlencoded"},
        )
        with urllib.request.urlopen(request, timeout=15) as response:
            result = json.loads(response.read())
        refresh = result.get("refresh_token")
        if not isinstance(refresh, str) or not refresh:
            raise ValueError("Google did not return a refresh token")
        await self.store.save(
            user_id,
            GoogleCredential(
                refresh_token=refresh,
                access_token=str(result.get("access_token", "")),
                expires_at=time.time() + float(result.get("expires_in", 3600)),
            ),
        )
        return user_id

    async def access_token(self, user_id: str) -> str | None:
        credential = await self.store.get(user_id)
        if credential is None:
            return None
        if credential.access_token and credential.expires_at > time.time() + 60:
            return credential.access_token
        payload = urllib.parse.urlencode(
            {
                "client_id": self.settings.client_id,
                "client_secret": self.settings.client_secret,
                "refresh_token": credential.refresh_token,
                "grant_type": "refresh_token",
            }
        ).encode()
        request = urllib.request.Request(
            "https://oauth2.googleapis.com/token",
            payload,
            {"Content-Type": "application/x-www-form-urlencoded"},
        )
        try:
            with urllib.request.urlopen(request, timeout=15) as response:
                result = json.loads(response.read())
        except Exception:
            return None
        credential.access_token = str(result.get("access_token", ""))
        credential.expires_at = time.time() + float(result.get("expires_in", 3600))
        await self.store.save(user_id, credential)
        return credential.access_token or None


@dataclass(frozen=True)
class Approval:
    approval_id: str
    user_id: str
    action: str
    payload: dict[str, Any]
    digest: str
    expires_at: float
    status: str = "pending"


class GoogleWorkspaceService:
    def __init__(self, oauth: GoogleOAuth, approvals: ApprovalStore | None = None) -> None:
        self.oauth = oauth
        self.approvals = approvals or ApprovalStore()
        self.mutating_actions = {
            "gmail.send", "gmail.reply", "gmail.modify",
            "calendar.create", "calendar.update", "calendar.delete",
        }

    @staticmethod
    def clock() -> float:
        return time.time()


class ApprovalStore:
    """In-memory approval repository with a database-friendly interface."""

    def __init__(self) -> None:
        self._items: dict[str, Approval] = {}

    def items(self) -> list[Approval]:
        return list(self._items.values())

    def create(self, user_id: str, action: str, payload: dict[str, Any], ttl: int = 600) -> Approval:
        raw = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()
        digest = hmac.new(b"donna-approval", action.encode() + b"\0" + raw, hashlib.sha256).hexdigest()
        approval = Approval(secrets.token_urlsafe(18), user_id, action, payload, digest, time.time() + ttl)
        self._items[approval.approval_id] = approval
        return approval

    def get(self, approval_id: str) -> Approval | None:
        return self._items.get(approval_id)

    def update(self, approval: Approval, status: str) -> Approval:
        updated = Approval(approval.approval_id, approval.user_id, approval.action, approval.payload, approval.digest, approval.expires_at, status)
        self._items[approval.approval_id] = updated
        return updated


def approval_json(approval: Approval) -> dict[str, Any]:
    return {
        "approval_id": approval.approval_id,
        "action": approval.action,
        "payload": approval.payload,
        "expires_at": approval.expires_at,
        "status": approval.status,
    }

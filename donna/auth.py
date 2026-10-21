from __future__ import annotations

import time
import uuid
from dataclasses import dataclass
from typing import Any, Protocol

import jwt


@dataclass(frozen=True)
class User:
    id: str
    email: str
    google_subject: str | None = None


@dataclass(frozen=True)
class AuthSession:
    user: User
    jti: str
    expires_at: float


class AuthRepository(Protocol):
    async def session_user(self, jti: str) -> User | None: ...
    async def replace_session(self, user: User, jti: str, expires_at: float) -> None: ...


class InMemoryAuthRepository:
    def __init__(self) -> None:
        self.users: dict[str, User] = {}
        self.sessions: dict[str, tuple[str, float]] = {}

    async def session_user(self, jti: str) -> User | None:
        item = self.sessions.get(jti)
        if item is None or item[1] <= time.time():
            self.sessions.pop(jti, None)
            return None
        return self.users.get(item[0])

    async def replace_session(self, user: User, jti: str, expires_at: float) -> None:
        self.users[user.id] = user
        for old_jti, (user_id, _) in list(self.sessions.items()):
            if user_id == user.id:
                del self.sessions[old_jti]
        self.sessions[jti] = (user.id, expires_at)


class JWTAuth:
    def __init__(self, secret: str, repository: AuthRepository, lifetime: int = 86400) -> None:
        if len(secret) < 32:
            raise ValueError("JWT secret must contain at least 32 characters")
        self.secret, self.repository, self.lifetime = secret, repository, lifetime

    async def issue(self, user: User) -> str:
        now = int(time.time())
        expires_at = now + self.lifetime
        jti = str(uuid.uuid4())
        await self.repository.replace_session(user, jti, expires_at)
        return jwt.encode(
            {"sub": user.id, "email": user.email, "jti": jti, "iat": now, "exp": expires_at},
            self.secret,
            algorithm="HS256",
        )

    async def authenticate(self, token: str) -> User | None:
        try:
            claims: dict[str, Any] = jwt.decode(token, self.secret, algorithms=["HS256"])
            jti, subject = claims.get("jti"), claims.get("sub")
            if not isinstance(jti, str) or not isinstance(subject, str):
                return None
        except jwt.InvalidTokenError:
            return None
        return await self.repository.session_user(jti)

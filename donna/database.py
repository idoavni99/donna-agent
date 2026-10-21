from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from donna.auth import AuthRepository, User
from donna.google_workspace import Approval, ApprovalStore, CredentialStore, GoogleCredential


@dataclass
class Postgres:
    url: str
    pool: Any = None

    async def ensure_pool(self) -> Any:
        if self.pool is None:
            try:
                import asyncpg
            except ImportError as exc:
                raise RuntimeError("install asyncpg to use PostgreSQL persistence") from exc
            self.pool = await asyncpg.create_pool(self.url, min_size=1, max_size=10)
        return self.pool

    async def close(self) -> None:
        if self.pool is not None:
            await self.pool.close()
            self.pool = None

    async def migrate(self) -> None:
        pool = await self.ensure_pool()
        await pool.execute(
            """
            CREATE TABLE IF NOT EXISTS users (
                id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL,
                google_subject TEXT UNIQUE, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
            );
            CREATE TABLE IF NOT EXISTS sessions (
                jti TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                expires_at TIMESTAMPTZ NOT NULL
            );
            CREATE TABLE IF NOT EXISTS google_credentials (
                user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
                refresh_token TEXT NOT NULL, access_token TEXT NOT NULL DEFAULT '',
                expires_at DOUBLE PRECISION NOT NULL DEFAULT 0, email TEXT NOT NULL DEFAULT ''
            );
            CREATE TABLE IF NOT EXISTS approvals (
                approval_id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                action TEXT NOT NULL, payload JSONB NOT NULL, digest TEXT NOT NULL,
                expires_at DOUBLE PRECISION NOT NULL, status TEXT NOT NULL
            );
            """
        )

    async def session_user(self, jti: str) -> User | None:
        pool = await self.ensure_pool()
        row = await pool.fetchrow(
            "SELECT u.id, u.email, u.google_subject FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.jti=$1 AND s.expires_at > now()", jti
        )
        return User(str(row["id"]), row["email"], row["google_subject"]) if row else None

    async def replace_session(self, user: User, jti: str, expires_at: float) -> None:
        pool = await self.ensure_pool()
        async with pool.acquire() as conn:
            async with conn.transaction():
                await conn.execute("INSERT INTO users(id,email,google_subject) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET email=EXCLUDED.email, google_subject=EXCLUDED.google_subject", user.id, user.email, user.google_subject)
                await conn.execute("DELETE FROM sessions WHERE user_id=$1", user.id)
                await conn.execute("INSERT INTO sessions(jti,user_id,expires_at) VALUES($1,$2,to_timestamp($3))", jti, user.id, expires_at)


class PostgresAuthRepository(AuthRepository):
    def __init__(self, db: Postgres) -> None:
        self.db = db

    async def session_user(self, jti: str) -> User | None:
        return await self.db.session_user(jti)

    async def replace_session(self, user: User, jti: str, expires_at: float) -> None:
        await self.db.replace_session(user, jti, expires_at)


class PostgresCredentialStore(CredentialStore):
    def __init__(self, db: Postgres) -> None:
        self.db = db

    async def save(self, user_id: str, credential: GoogleCredential) -> None:
        pool = await self.db.ensure_pool()
        await pool.execute(
            """INSERT INTO google_credentials(user_id,refresh_token,access_token,expires_at,email)
               VALUES($1,$2,$3,$4,$5)
               ON CONFLICT(user_id) DO UPDATE SET refresh_token=$2, access_token=$3, expires_at=$4, email=$5""",
            user_id, credential.refresh_token, credential.access_token,
            credential.expires_at, credential.email,
        )

    async def get(self, user_id: str) -> GoogleCredential | None:
        pool = await self.db.ensure_pool()
        row = await pool.fetchrow(
            "SELECT refresh_token, access_token, expires_at, email FROM google_credentials WHERE user_id=$1",
            user_id,
        )
        if row is None:
            return None
        return GoogleCredential(row["refresh_token"], row["access_token"], row["expires_at"], row["email"])

    async def delete(self, user_id: str) -> None:
        pool = await self.db.ensure_pool()
        await pool.execute("DELETE FROM google_credentials WHERE user_id=$1", user_id)


class PostgresApprovalStore(ApprovalStore):
    """Reserved adapter boundary; routes can switch to async repository methods later."""
    pass

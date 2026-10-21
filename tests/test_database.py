import asyncio
import json

from donna.auth import User
from donna.database import Postgres, PostgresApprovalStore, PostgresCredentialStore
from donna.google_workspace import GoogleCredential


class FakePool:
    def __init__(self):
        self.commands = []
        self.rows = {}

    async def execute(self, query, *args):
        self.commands.append((query, args))
        if query.startswith("UPDATE approvals"):
            approval_id, status, _ = args
            row = self.rows.get(approval_id)
            if row and row["status"] == "pending":
                row["status"] = status
                return "UPDATE 1"
            return "UPDATE 0"
        if query.startswith("INSERT INTO approvals"):
            approval_id, user_id, action, payload, digest, expires_at, status = args
            self.rows[approval_id] = {
                "approval_id": approval_id, "user_id": user_id, "action": action,
                "payload": json.loads(payload), "digest": digest,
                "expires_at": expires_at, "status": status,
            }
        return "OK"

    async def fetch(self, query, *args):
        return list(self.rows.values())

    async def fetchrow(self, query, *args):
        if "approvals" in query:
            return self.rows.get(args[0])
        return None


async def _store():
    pool = FakePool()
    db = Postgres("postgres://test", pool)
    store = PostgresApprovalStore(db)
    created = await store.acreate("user-1", "gmail.send", {"to": "a@example.com"})
    loaded = await store.aget(created.approval_id)
    assert loaded == created
    assert (await store.aupdate(created, "approved")).status == "approved"
    assert await store.aupdate(created, "rejected") is None
    assert (await store.aitems())[0].status == "approved"


def test_postgres_approval_store_persists_and_atomically_transitions():
    asyncio.run(_store())


def test_postgres_credential_store_uses_database():
    async def run():
        pool = FakePool()
        db = Postgres("postgres://test", pool)
        store = PostgresCredentialStore(db)
        credential = GoogleCredential("refresh", "access", 123.0, "a@example.com")
        await store.save("user-1", credential)
        assert await store.get("user-1") is None
        await store.delete("user-1")

    asyncio.run(run())

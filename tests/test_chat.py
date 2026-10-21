import asyncio

from pydantic_ai.models.test import TestModel

from donna.chat import ChatRuntime, InvalidSession, SessionNotFound
from donna.web import fetch_public_url, find_public_urls


class ClassifierStub:
    pass


def runtime():
    return ChatRuntime(TestModel(call_tools=[], custom_output_text="Hello."), ClassifierStub(), 0.55, "System prompt", search_enabled=False)


def test_chat_creates_and_reuses_bounded_session():
    agent = runtime()
    first = asyncio.run(agent.reply("hello"))
    second = asyncio.run(agent.reply("follow up", first["session_id"]))
    assert first["answer"] == "Hello."
    assert second["session_id"] == first["session_id"]
    assert sum(map(len, agent.sessions[first["session_id"]].exchanges)) <= 24


def test_chat_rejects_invalid_or_expired_session_ids():
    agent = runtime()
    for session_id, error in [("bad/id", InvalidSession), ("unknown", SessionNotFound)]:
        try:
            asyncio.run(agent.reply("hello", session_id))
        except error:
            continue
        raise AssertionError(f"expected {error.__name__}")


def test_user_urls_only_approve_public_hosts_and_fetch_requires_selection():
    found, urls = find_public_urls("Read https://example.com/story and http://127.0.0.1/private")
    assert found and urls == ["https://example.com/story"]
    blocked = fetch_public_url("https://example.com/", [])
    assert blocked.startswith("Fetch blocked")

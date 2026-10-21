import io
import socket
import urllib.error

import pytest

from donna import retry, web
from donna.tools import ProfileSearcher


def test_retry_request_uses_bounded_exponential_full_jitter(monkeypatch):
    calls = 0
    delays = []
    monkeypatch.setattr(retry.time, "sleep", delays.append)
    monkeypatch.setattr(retry.random, "uniform", lambda low, high: high / 2)

    def request():
        nonlocal calls
        calls += 1
        if calls < 3:
            raise TimeoutError("temporary timeout")
        return "ok"

    assert retry.retry_request(request) == "ok"
    assert calls == 3
    assert delays == [0.125, 0.25]


def test_retry_request_does_not_retry_permanent_http_errors(monkeypatch):
    calls = 0
    monkeypatch.setattr(retry.time, "sleep", lambda _: pytest.fail("unexpected retry"))

    def request():
        nonlocal calls
        calls += 1
        raise urllib.error.HTTPError("https://example.com", 401, "unauthorized", {}, io.BytesIO())

    with pytest.raises(urllib.error.HTTPError):
        retry.retry_request(request)
    assert calls == 1


def test_page_fetch_retries_transient_server_status(monkeypatch):
    monkeypatch.setattr(
        web.socket,
        "getaddrinfo",
        lambda *args, **kwargs: [
            (socket.AF_INET, socket.SOCK_STREAM, socket.IPPROTO_TCP, "", ("93.184.216.34", 80))
        ],
    )
    monkeypatch.setattr(retry.time, "sleep", lambda _: None)
    statuses = iter([503, 200])

    class Headers:
        def get_content_type(self):
            return "text/plain"

    class Response:
        headers = Headers()

        def __init__(self, status):
            self.status = status

        def read(self, _size):
            return b"page text"

    class Connection:
        def __init__(self, *_args, **_kwargs):
            pass

        def request(self, *_args, **_kwargs):
            pass

        def getresponse(self):
            return Response(next(statuses))

        def close(self):
            pass

    monkeypatch.setattr(web, "_PinnedHTTPConnection", Connection)
    assert web.fetch_public_url("http://example.com/story", ["example.com"]) == "page text"


def test_profile_search_retries_transient_provider_status(monkeypatch):
    monkeypatch.setattr(retry.time, "sleep", lambda _: None)
    calls = 0

    class Response:
        def __enter__(self):
            return self

        def __exit__(self, *_args):
            pass

        def geturl(self):
            return "https://search.example/search"

        def read(self, _size):
            return b'{"candidates": []}'

    class Opener:
        def open(self, *_args, **_kwargs):
            nonlocal calls
            calls += 1
            if calls == 1:
                raise urllib.error.HTTPError(
                    "https://search.example/search", 503, "unavailable", {}, io.BytesIO()
                )
            return Response()

    monkeypatch.setattr("donna.tools.urllib.request.build_opener", lambda *_args: Opener())
    searcher = ProfileSearcher("https://search.example/search")
    assert searcher._request(b"{}") == []
    assert calls == 2

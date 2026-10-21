import http.client
import ipaddress
import re
import socket
import ssl
from html.parser import HTMLParser
from urllib.parse import urlsplit

MAX_FETCHES = 3
URL_PATTERN = re.compile(r"(?i)(https?://[^\s<>\"']+|www\.[^\s<>\"']+)")


class _Text(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.parts: list[str] = []

    def handle_data(self, data: str) -> None:
        if data.strip():
            self.parts.append(data.strip())


def find_public_urls(message: str) -> tuple[bool, list[str]]:
    found, urls, seen = False, [], set()
    for match in URL_PATTERN.findall(message):
        match = match.rstrip(".,!?;:)]}")
        found = True
        if match.lower().startswith("www."):
            match = "https://" + match
        parsed = urlsplit(match)
        host = (parsed.hostname or "").lower()
        if (
            parsed.scheme not in {"http", "https"}
            or not host
            or parsed.username
            or parsed.password
            or not _public_host(host)
        ):
            continue
        canonical = parsed._replace(netloc=parsed.netloc.lower()).geturl()
        if canonical not in seen:
            seen.add(canonical)
            urls.append(canonical)
        if len(urls) == MAX_FETCHES:
            break
    return found, urls


def _public_host(host: str) -> bool:
    if host == "localhost" or host.endswith((".localhost", ".local", ".internal")):
        return False
    try:
        return ipaddress.ip_address(host).is_global
    except ValueError:
        return "." in host


def fetch_public_url(url: str, allowed_hosts: list[str]) -> str:
    parsed = urlsplit(url)
    host = (parsed.hostname or "").lower()
    port = parsed.port or (443 if parsed.scheme == "https" else 80)
    if (
        parsed.scheme not in {"http", "https"}
        or port != (443 if parsed.scheme == "https" else 80)
        or host not in allowed_hosts
        or not _public_host(host)
    ):
        return "Fetch blocked: URL host was not selected by the user or is not public."
    try:
        records = socket.getaddrinfo(host, port, type=socket.SOCK_STREAM)
        addresses = {ipaddress.ip_address(item[4][0]) for item in records}
        if not addresses or any(not address.is_global for address in addresses):
            return "Fetch blocked: host resolves to a non-public address."
        record = records[0]
        connection = (
            _PinnedHTTPSConnection(host, port, record)
            if parsed.scheme == "https"
            else _PinnedHTTPConnection(host, port, record)
        )
        target = parsed.path or "/"
        if parsed.query:
            target += "?" + parsed.query
        host_header = f"[{host}]" if ":" in host else host
        try:
            connection.request(
                "GET",
                target,
                headers={
                    "Host": host_header,
                    "User-Agent": "Donna/1.0",
                    "Accept": "text/html,text/plain",
                },
            )
            response = connection.getresponse()
            if response.status < 200 or response.status >= 300:
                return "Fetch failed or was redirected. Ask the user to share the relevant text."
            if response.headers.get_content_type() not in {"text/html", "text/plain"}:
                return "Fetch blocked: only HTML and plain text pages are supported."
            body = response.read(512 * 1024 + 1)
        finally:
            connection.close()
        parser = _Text()
        parser.feed(body[: 512 * 1024].decode("utf-8", errors="replace"))
        return " ".join(parser.parts)[:32_000]
    except OSError, ValueError, http.client.HTTPException:
        return "Fetch failed. Ask the user to share the relevant text."


class _PinnedHTTPConnection(http.client.HTTPConnection):
    def __init__(self, host: str, port: int, record: tuple) -> None:
        super().__init__(host, port, timeout=8)
        self.record = record

    def connect(self) -> None:
        family, kind, protocol, _, address = self.record
        self.sock = socket.socket(family, kind, protocol)
        self.sock.settimeout(self.timeout)
        self.sock.connect(address)


class _PinnedHTTPSConnection(_PinnedHTTPConnection):
    def connect(self) -> None:
        super().connect()
        self.sock = ssl.create_default_context().wrap_socket(self.sock, server_hostname=self.host)

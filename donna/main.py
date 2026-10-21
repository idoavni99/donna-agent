import logging
from pathlib import Path

import uvicorn

from donna.api import Service, create_app
from donna.chat import ChatRuntime
from donna.classifier import ModelClassifier
from donna.config import Settings
from donna.tools import ProfileSearcher, Tools
from donna.google_workspace import GoogleOAuth, GoogleOAuthSettings, GoogleWorkspaceService, InMemoryCredentialStore
from donna.auth import InMemoryAuthRepository, JWTAuth
from donna.database import (
    Postgres,
    PostgresApprovalStore,
    PostgresAuthRepository,
    PostgresCredentialStore,
)


def _system_prompt(path: str) -> str:
    content = (
        Path(path).read_text(encoding="utf-8")
        if path.strip()
        else Path(__file__).with_name("system_prompt.txt").read_text(encoding="utf-8")
    )
    content = content.strip()
    if not content or len(content.encode()) > 32 * 1024:
        raise ValueError("system prompt must be non-empty and at most 32 KiB")
    return content


def build_app(database: Postgres | None = None):
    settings = Settings()  # type: ignore[call-arg]  # Loaded from aliased environment settings.
    level = {"WARN": "WARNING"}.get(
        settings.donna_log_level.upper(), settings.donna_log_level.upper()
    )
    if level not in {"DEBUG", "INFO", "WARNING", "ERROR"}:
        raise ValueError("DONNA_LOG_LEVEL must be debug, info, warn, or error")
    logging.basicConfig(level=level, format="%(message)s")
    classifier = ModelClassifier(
        settings.openrouter_api_key, settings.openrouter_model, settings.openrouter_app_url
    )
    searcher = (
        ProfileSearcher(settings.donna_profile_search_url, settings.donna_profile_search_api_key)
        if settings.donna_profile_search_url
        else None
    )
    tools = Tools(classifier, settings.donna_min_confidence, searcher)
    database = database or (Postgres(settings.donna_database_url) if settings.donna_database_url else None)
    google = None
    if settings.google_client_id and settings.google_client_secret and settings.google_redirect_uri:
        google = GoogleWorkspaceService(
            GoogleOAuth(
                GoogleOAuthSettings(
                    settings.google_client_id,
                    settings.google_client_secret,
                    settings.google_redirect_uri,
                ),
                PostgresCredentialStore(database) if database else InMemoryCredentialStore(),
            ),
            PostgresApprovalStore(database) if database else None,
        )
    chat = ChatRuntime(
        classifier.model,
        classifier,
        settings.donna_min_confidence,
        _system_prompt(settings.donna_system_prompt_file),
        tools,
        google=google,
    )
    auth_repository = PostgresAuthRepository(database) if database else InMemoryAuthRepository()
    auth = JWTAuth(settings.donna_jwt_secret, auth_repository) if settings.donna_jwt_secret else None
    return create_app(
        Service(
            classifier,
            settings.donna_agent_api_key,
            settings.donna_min_confidence,
            chat,
            tools,
            google,
            auth,
            database,
        )
    )


app = build_app()


if __name__ == "__main__":
    settings = Settings()  # type: ignore[call-arg]  # Loaded from aliased environment settings.
    host, separator, port = settings.donna_addr.rpartition(":")
    if not separator or not port.isdigit():
        raise ValueError("DONNA_ADDR must be host:port")
    uvicorn.run(app, host=host or "0.0.0.0", port=int(port))

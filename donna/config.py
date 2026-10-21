from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    openrouter_api_key: str = Field(alias="OPENROUTER_API_KEY")
    openrouter_model: str = Field(alias="OPENROUTER_MODEL")
    donna_agent_api_key: str = Field(alias="DONNA_AGENT_API_KEY")
    openrouter_app_url: str = Field(default="", alias="OPENROUTER_APP_URL")
    donna_min_confidence: float = Field(default=0.55, alias="DONNA_MIN_CONFIDENCE", ge=0, le=1)
    donna_addr: str = Field(default="127.0.0.1:8080", alias="DONNA_ADDR")
    donna_log_level: str = Field(default="info", alias="DONNA_LOG_LEVEL")
    donna_system_prompt_file: str = Field(default="", alias="DONNA_SYSTEM_PROMPT_FILE")
    donna_profile_search_url: str = Field(default="", alias="DONNA_PROFILE_SEARCH_URL")
    donna_profile_search_api_key: str = Field(default="", alias="DONNA_PROFILE_SEARCH_API_KEY")
    google_client_id: str = Field(default="", alias="GOOGLE_CLIENT_ID")
    google_client_secret: str = Field(default="", alias="GOOGLE_CLIENT_SECRET")
    google_redirect_uri: str = Field(default="", alias="GOOGLE_REDIRECT_URI")
    donna_database_url: str = Field(default="", alias="DONNA_DATABASE_URL")
    donna_jwt_secret: str = Field(default="", alias="DONNA_JWT_SECRET")

    @field_validator("openrouter_api_key", "openrouter_model", "donna_agent_api_key")
    @classmethod
    def require_nonempty(cls, value: str) -> str:
        value = value.strip()
        if not value or value.lower() in {
            "replace-with-your-key",
            "replace-with-a-local-token",
            "local-agent-token",
        }:
            raise ValueError("required setting is empty or still a placeholder")
        return value

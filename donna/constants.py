"""Resource limits for the HTTP API, chat sessions, and external adapters."""

# HTTP requests are buffered before route dispatch, so this is also the body memory ceiling.
API_MAX_REQUEST_BODY_BYTES = 16 * 1024

# Chat history stays small enough for predictable process memory while preserving recent turns.
CHAT_MAX_INPUT_BYTES = 16 * 1024
CHAT_MAX_PROMPT_BYTES = 32 * 1024
CHAT_MAX_SESSIONS = 128
CHAT_SESSION_TTL_SECONDS = 2 * 60 * 60
CHAT_MAX_HISTORY_MESSAGES = 24
CHAT_MAX_HISTORY_BYTES = 128 * 1024
CHAT_MAX_REMEMBERED_URLS = 10
CHAT_MAX_REMEMBERED_HOSTS = 10
PUBLIC_URL_MAX_CHARS = 2048

# Limit bytes and individual text fields received from the optional profile-search provider.
PROFILE_SEARCH_MAX_RESPONSE_BYTES = 128 * 1024
PROFILE_SEARCH_MAX_CANDIDATES = 5
PROFILE_NAME_MAX_CHARS = 100
PROFILE_CONTEXT_MAX_CHARS = 160
PROFILE_SEARCH_MAX_TEXT_CHARS = 512
PROFILE_SEARCH_TIMEOUT_SECONDS = 10

# The agent gets at most this many user-selected pages to fetch per chat turn.
WEB_MAX_FETCHES_PER_TURN = 3
WEB_FETCH_TIMEOUT_SECONDS = 8
WEB_FETCH_MAX_BODY_BYTES = 512 * 1024
WEB_FETCH_MAX_EXTRACTED_CHARS = 32_000

# Retry only a small number of transient outbound request failures; delay uses full jitter.
HTTP_MAX_ATTEMPTS = 3
HTTP_RETRY_BASE_DELAY_SECONDS = 0.25
HTTP_RETRY_MAX_DELAY_SECONDS = 2.0
HTTP_RETRYABLE_STATUS_CODES = frozenset({408, 409, 425, 429})

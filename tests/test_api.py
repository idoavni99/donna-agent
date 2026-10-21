from fastapi.testclient import TestClient

from donna.api import Service, create_app
from donna.decision import Scores, YesNoScores
from donna.tools import Tools


class ClassifierStub:
    async def classify(self, query):
        return Scores(answer_directly=0.1, structured_lookup=0.1, semantic_search=0.6, hybrid_search=0.1, clarify=0.1)

    async def evaluate(self, question, evidence):
        return YesNoScores(yes=0.5, no=0.5)


def client():
    return TestClient(create_app(Service(ClassifierStub(), "token", 0.55)))


def test_decision_and_check_contracts():
    api = client()
    headers = {"Authorization": "Bearer token"}
    route = api.post("/v1/decisions", headers=headers, json={"query": "find notes"})
    check = api.post("/v1/checks", headers=headers, json={"question": "saved?"})
    assert route.status_code == 200 and route.json() == {"action": "semantic_search", "confidence": 0.6}
    assert check.status_code == 200 and check.json() == {"answer": None, "confidence": 0.5}


def test_auth_exact_fields_and_request_size():
    api = client()
    assert api.post("/v1/decisions", json={"query": "hello"}).status_code == 401
    headers = {"Authorization": "Bearer token"}
    assert api.post("/v1/decisions", headers=headers, json={"query": "hello", "extra": 1}).status_code == 400
    assert api.post("/v1/checks", headers=headers, content=b"{" + b" " * (16 * 1024)).status_code == 400


def test_tool_catalog_and_dispatch_require_auth():
    tools = Tools(ClassifierStub(), 0.55)
    api = TestClient(create_app(Service(ClassifierStub(), "token", 0.55, tools=tools)))
    headers = {"Authorization": "Bearer token"}
    listed = api.get("/v1/tools", headers=headers)
    called = api.post("/v1/tools/call", headers=headers, json={"name": "donna_check_yes_no", "arguments": {"question": "saved?", "evidence": "notes"}})
    assert listed.status_code == 200 and listed.json()[0]["name"] == "donna_route_query"
    assert called.status_code == 200 and called.json() == {"answer": None, "confidence": 0.5}
    assert api.get("/v1/tools").status_code == 401


def test_health_openapi_and_swagger_are_available():
    api = client()
    assert api.get("/healthz").status_code == 204
    assert api.get("/openapi.yaml").status_code == 200
    assert api.get("/swagger").status_code == 200

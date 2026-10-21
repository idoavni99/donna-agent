from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
from pydantic import ValidationError

from donna.api_common import Service, parse_json_object


def register_tool_routes(app: FastAPI, service: Service) -> None:
    if service.tools is None:
        return

    @app.get("/v1/tools", dependencies=[Depends(service.authorize)])
    async def list_tools() -> list[dict]:
        return service.tools.definitions()

    @app.post("/v1/tools/call", dependencies=[Depends(service.authorize)])
    async def call_tool(request: Request):
        error = "tool call requires one name and arguments object"
        payload = await parse_json_object(
            request, {"name", "arguments"}, required={"name", "arguments"}, error_message=error
        )
        if (
            not isinstance(payload["name"], str)
            or not payload["name"]
            or not isinstance(payload["arguments"], dict)
        ):
            raise HTTPException(400, error)
        try:
            return JSONResponse(await service.tools.invoke(payload["name"], payload["arguments"]))
        except KeyError as exc:
            raise HTTPException(404, "unknown tool") from exc
        except LookupError as exc:
            raise HTTPException(503, str(exc)) from exc
        except (ValueError, ValidationError) as exc:
            raise HTTPException(400, "invalid tool arguments") from exc
        except Exception as exc:
            raise HTTPException(502, "tool execution failed") from exc

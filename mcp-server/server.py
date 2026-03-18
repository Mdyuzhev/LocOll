"""Homelab MCP Server — управление homelab-сервером через MCP протокол."""

from fastmcp import FastMCP
from starlette.requests import Request
from starlette.responses import Response, JSONResponse

from tools.services import get_services_status, get_service_logs, get_system_metrics
from tools.docker_tools import restart_service, stop_service, start_service, get_docker_ps, get_docker_stats
from tools.utils import run_health_check, get_server_info
from tools.shell import run_shell_command, exec_in_container, grep_docker_logs, compose_up
from tools.bot_manager import manage_bot
from tools.notes import add_note, list_notes, complete_note
from tools.bridge import claude_bridge_handler, TOOL_REGISTRY
from tools.workflows import deploy_project, restart_and_verify, run_db_migration
from tools.git_tools import git_status, git_log
from tools.production import get_production_status, notify_deploy, scan_projects, identify_project

mcp = FastMCP(
    name="homelab-mcp",
    instructions=(
        "MCP-сервер для управления homelab-сервером (Ubuntu 24.04, Docker). "
        "Предоставляет инструменты для мониторинга сервисов, управления Docker-контейнерами, "
        "просмотра логов, системных метрик и комплексной проверки здоровья инфраструктуры."
    ),
)

# Register all tools
mcp.tool()(get_services_status)
mcp.tool()(get_service_logs)
mcp.tool()(get_system_metrics)
mcp.tool()(restart_service)
mcp.tool()(stop_service)
mcp.tool()(start_service)
mcp.tool()(get_docker_ps)
mcp.tool()(get_docker_stats)
mcp.tool()(run_health_check)
mcp.tool()(get_server_info)
mcp.tool()(run_shell_command)
mcp.tool()(exec_in_container)
mcp.tool()(grep_docker_logs)
mcp.tool()(compose_up)
mcp.tool()(manage_bot)
mcp.tool()(add_note)
mcp.tool()(list_notes)
mcp.tool()(complete_note)
mcp.tool()(deploy_project)
mcp.tool()(restart_and_verify)
mcp.tool()(run_db_migration)
mcp.tool()(git_status)
mcp.tool()(git_log)
mcp.tool()(get_production_status)
mcp.tool()(notify_deploy)
mcp.tool()(scan_projects)
mcp.tool()(identify_project)


# --- claude-bridge: simplified HTTP endpoint ---

@mcp.custom_route("/claude-bridge", methods=["POST"])
async def claude_bridge(request: Request) -> Response:
    """POST /claude-bridge — вызов инструмента одним запросом."""
    response = await claude_bridge_handler(request)
    response.headers["Access-Control-Allow-Origin"] = "*"
    return response


@mcp.custom_route("/claude-bridge", methods=["OPTIONS"])
async def claude_bridge_options(request: Request) -> Response:
    """CORS preflight для браузерных запросов."""
    r = Response()
    r.headers["Access-Control-Allow-Origin"] = "*"
    r.headers["Access-Control-Allow-Methods"] = "POST, OPTIONS"
    r.headers["Access-Control-Allow-Headers"] = "Content-Type"
    return r


@mcp.custom_route("/claude-bridge/tools", methods=["GET"])
async def claude_bridge_tools(request: Request) -> Response:
    """GET /claude-bridge/tools — список доступных инструментов."""
    r = JSONResponse({"ok": True, "tools": sorted(TOOL_REGISTRY.keys())})
    r.headers["Access-Control-Allow-Origin"] = "*"
    return r


@mcp.custom_route("/static/mcp-client.js", methods=["GET"])
async def serve_mcp_client(request: Request) -> Response:
    """Serve JS bridge client."""
    from starlette.responses import FileResponse
    return FileResponse("/app/static/mcp-client.js",
                        media_type="application/javascript")



@mcp.custom_route("/health", methods=["GET"])
async def health(request: Request) -> JSONResponse:
    return JSONResponse({"status": "ok", "service": "homelab-mcp"})


if __name__ == "__main__":
    mcp.run(transport="streamable-http", host="0.0.0.0", port=8765, stateless_http=True)

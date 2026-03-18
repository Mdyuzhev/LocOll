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
from tools.production import scan_projects, identify_project

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
mcp.tool()(scan_projects)
mcp.tool()(identify_project)


# --- claude-bridge: simplified HTTP endpoint ---

@mcp.custom_route("/bridge/{tool_name}", methods=["POST"])
async def bridge_route(request: Request) -> Response:
    return await claude_bridge_handler(request)

@mcp.custom_route("/bridge/tools", methods=["GET"])
async def bridge_tools_list(request: Request) -> Response:
    return JSONResponse({"tools": list(TOOL_REGISTRY.keys())})


if __name__ == "__main__":
    mcp.run(transport="streamable-http", host="0.0.0.0", port=8765, stateless_http=True)

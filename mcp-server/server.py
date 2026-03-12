"""Homelab MCP Server — управление homelab-сервером через MCP протокол."""

from fastmcp import FastMCP

from tools.services import get_services_status, get_service_logs, get_system_metrics
from tools.docker_tools import restart_service, stop_service, start_service, get_docker_ps
from tools.utils import run_health_check, get_server_info
from tools.shell import run_shell_command, exec_in_container, grep_docker_logs, compose_up
from tools.bot_manager import manage_bot

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
mcp.tool()(run_health_check)
mcp.tool()(get_server_info)
mcp.tool()(run_shell_command)
mcp.tool()(exec_in_container)
mcp.tool()(grep_docker_logs)
mcp.tool()(compose_up)
mcp.tool()(manage_bot)

if __name__ == "__main__":
    mcp.run(transport="streamable-http", host="0.0.0.0", port=8765)

"""A client of the host API."""

import os
from collections.abc import Mapping

import httpx

from mavio_plugin.serve import ENV_HOST_ADDRESS, ENV_HOST_SOCKET, ENV_TOKEN, HOST_URL, loopback


def host_client(
    headers: Mapping[str, str] | None = None, timeout: httpx.Timeout | None = None
) -> httpx.AsyncClient:
    """An HTTP client of the host API, whose relative URLs are the host
    API's and whose requests carry the plugin's token and headers; it fails
    outside a Mavio host.

    Requests go over the host socket, or on Windows to the host's loopback
    address. Connect's procedures are POSTed to "/<service>/<method>".
    """
    socket = os.environ.get(ENV_HOST_SOCKET, "")
    address = os.environ.get(ENV_HOST_ADDRESS, "")
    if loopback() and address:
        transport, base_url = httpx.AsyncHTTPTransport(), f"http://{address}"
    elif socket:
        transport, base_url = httpx.AsyncHTTPTransport(uds=socket), HOST_URL
    else:
        raise RuntimeError("the host API is not available outside a Mavio host")
    return httpx.AsyncClient(
        base_url=base_url,
        transport=transport,
        headers={"Authorization": "Bearer " + os.environ.get(ENV_TOKEN, ""), **(headers or {})},
        # Streams, such as Ask's, wait for their messages as long as they
        # take.
        timeout=timeout or httpx.Timeout(10, read=None),
    )


_REPORT_TASK_PROGRESS = "/mavio.plugin.v1.HostService/ReportTaskProgress"
_START_TASK = "/mavio.plugin.v1.HostService/StartTask"


async def report_task_progress(
    host: httpx.AsyncClient, task_id: str, progress: float, status: str = ""
) -> bool:
    """Tells the host the fraction done, 0 to 1, of the plugin's task
    running, which it shows as its job's progress, and what the run is
    doing, such as the file it downloads; an empty status clears the last.
    The host ignores a report of a task not running. It returns whether the
    host answered, for a plugin to log, never failing the run."""
    body: dict[str, object] = {"taskId": task_id, "progress": min(max(progress, 0.0), 1.0)}
    if status:
        body["status"] = status[:200]
    return await _post(host, _REPORT_TASK_PROGRESS, body)


async def start_task(host: httpx.AsyncClient, task_id: str) -> bool:
    """Asks the host for a run of one of the plugin's tasks, which it queues
    as an administrator's run once the plugin is ready, joining a run
    waiting: work the plugin starts itself, such as after its
    configuration, shows in the tasks. It returns whether the host took the
    request, for a plugin to log; it never waits for the run."""
    return await _post(host, _START_TASK, {"taskId": task_id})


async def _post(host: httpx.AsyncClient, procedure: str, body: dict[str, object]) -> bool:
    try:
        r = await host.post(procedure, json=body, headers={"Content-Type": "application/json"})
    except httpx.HTTPError:
        return False
    return r.status_code == 200
